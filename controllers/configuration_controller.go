package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	crossplane "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/pkg/errors"
	"github.com/ttsubo/client-go/tools/cache"
	tfcfg "github.com/ttsubo2000/esi-terraform-worker/controllers/configuration"
	"github.com/ttsubo2000/esi-terraform-worker/controllers/provider"
	"github.com/ttsubo2000/esi-terraform-worker/controllers/terraform"
	"github.com/ttsubo2000/esi-terraform-worker/controllers/util"
	cacheObj "github.com/ttsubo2000/esi-terraform-worker/tools/cache"
	"github.com/ttsubo2000/esi-terraform-worker/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	terraformWorkspace = "default"
	// WorkingVolumeMountPath is the mount path for working volume
	WorkingVolumeMountPath = "/data"
	// InputTFConfigurationVolumeName is the volume name for input Terraform Configuration
	InputTFConfigurationVolumeName = "tf-input-configuration"
	// BackendVolumeName is the volume name for Terraform backend
	BackendVolumeName = "tf-backend"
	// InputTFConfigurationVolumeMountPath is the volume mount path for input Terraform Configuration
	InputTFConfigurationVolumeMountPath = "/opt/tf-configuration"
	// BackendVolumeMountPath is the volume mount path for Terraform backend
	BackendVolumeMountPath = "/opt/tf-backend"
	// terraformContainerName is the name of the container that executes the terraform in the pod
	terraformContainerName     = "terraform-executor"
	terraformInitContainerName = "terraform-init"
)

const (
	// TerraformStateNameInSecret is the key name to store Terraform state
	TerraformStateNameInSecret = "tfstate"
	// TFInputConfigMapName is the CM name for Terraform Input Configuration
	TFInputConfigMapName = "tf-%s"
	// TFVariableSecret is the Secret name for variables, including credentials from Provider
	TFVariableSecret = "variable-%s"
	// TFBackendSecret is the Secret name for Kubernetes backend
	TFBackendSecret = "tfstate-%s-%s"
)

// TerraformExecutionType is the type for Terraform execution
type TerraformExecutionType string

const (
	// TerraformApply is the name to mark `terraform apply`
	TerraformApply TerraformExecutionType = "apply"
	// TerraformDestroy is the name to mark `terraform destroy`
	TerraformDestroy TerraformExecutionType = "destroy"
)

const (
	configurationFinalizer = "configuration.finalizers.terraform-controller"
	// ClusterRoleName is the name of the ClusterRole for Terraform Job
	ClusterRoleName = "tf-executor-clusterrole"
	// ServiceAccountName is the name of the ServiceAccount for Terraform Job
	ServiceAccountName = "tf-executor-service-account"
)

// ConfigurationReconciler reconciles a Configuration object.
type ConfigurationReconciler struct {
	ProviderName string
	Client       cacheObj.Store
}

func (r *ConfigurationReconciler) Reconcile(ctx context.Context, req Request, indexer cache.Indexer) (Result, error) {
	klog.InfoS("reconciling Terraform Configuration...", "NamespacedName", req.NamespacedName)

	obj, _, err := indexer.GetByKey(req.NamespacedName)
	if err != nil {
		err = nil
		return Result{}, err
	}
	configuration := obj.(*types.Configuration)

	meta := initTFConfigurationMeta(req, configuration)

	// add finalizer
	var isDeleting = !configuration.ObjectMeta.DeletionTimestamp.IsZero()
	if !isDeleting {
		if !controllerutil.ContainsFinalizer(configuration, configurationFinalizer) {
			controllerutil.AddFinalizer(configuration, configurationFinalizer)
			if err := r.Client.Update(configuration); err != nil {
				return Result{RequeueAfter: 3 * time.Second}, errors.Wrap(err, "failed to add finalizer")
			}
		}
	}

	// pre-check Configuration
	if err := r.preCheck(ctx, configuration, meta); err != nil && !isDeleting {
		return Result{}, err
	}

	var tfExecutionJob = &types.Job{}
	key := "Job" + "/" + meta.Namespace + "/" + meta.ApplyJobName
	obj, _, err = r.Client.GetByKey(key)
	if err == nil {
		tfExecutionJob = obj.(*types.Job)
		if !meta.EnvChanged && tfExecutionJob.Status.Succeeded == int32(1) {
			if err := meta.updateApplyStatus(ctx, r.Client, types.Available, types.MessageCloudResourceDeployed); err != nil {
				return Result{}, err
			}
		}
	}
	var Namespace, Name string
	NamespacedName := strings.Split(req.NamespacedName, "/")
	if len(NamespacedName) != 2 {
		Namespace = ""
		Name = ""
	} else {
		Namespace = NamespacedName[0]
		Name = NamespacedName[1]
	}

	if isDeleting {
		// terraform destroy
		klog.InfoS("performing Configuration Destroy", "NamespacedName", req.NamespacedName, "JobName", meta.DestroyJobName)

		_, err := terraform.GetTerraformStatus(ctx, meta.Namespace, meta.DestroyJobName, terraformContainerName, terraformInitContainerName)
		if err != nil {
			klog.ErrorS(err, "Terraform destroy failed")
			if updateErr := meta.updateDestroyStatus(ctx, r.Client, types.ConfigurationDestroyFailed, err.Error()); updateErr != nil {
				return Result{}, updateErr
			}
		}
		if err := r.terraformDestroy(ctx, req.NamespacedName, configuration, meta); err != nil {
			if err.Error() == types.MessageDestroyJobNotCompleted {
				return Result{RequeueAfter: 3 * time.Second}, nil
			}
			return Result{RequeueAfter: 3 * time.Second}, errors.Wrap(err, "continue reconciling to destroy cloud resource")
		}
		configuration, err := tfcfg.Get(ctx, r.Client, Namespace, Name)
		if err != nil {
			return Result{}, err
		}
		if controllerutil.ContainsFinalizer(&configuration, configurationFinalizer) {
			controllerutil.RemoveFinalizer(&configuration, configurationFinalizer)
			if err := r.Client.Update(&configuration); err != nil {
				return Result{RequeueAfter: 3 * time.Second}, errors.Wrap(err, "failed to remove finalizer")
			}
		}
		return Result{}, nil
	}

	// Terraform apply (create or update)
	klog.InfoS("performing Terraform Apply (cloud resource create/update)", "Namespace", Namespace, "Name", Name)
	if err := r.terraformApply(ctx, Namespace, configuration, meta); err != nil {
		if err.Error() == types.MessageApplyJobNotCompleted {
			return Result{RequeueAfter: 3 * time.Second}, nil
		}
		return Result{RequeueAfter: 3 * time.Second}, errors.Wrap(err, "failed to create/update cloud resource")
	}
	state, err := terraform.GetTerraformStatus(ctx, meta.Namespace, meta.ApplyJobName, terraformContainerName, terraformInitContainerName)
	if err != nil {
		klog.ErrorS(err, "Terraform apply failed")
		if updateErr := meta.updateApplyStatus(ctx, r.Client, state, err.Error()); updateErr != nil {
			return Result{}, updateErr
		}
	}

	return Result{}, nil
}

// TFConfigurationMeta is all the metadata of a Configuration
type TFConfigurationMeta struct {
	Name                  string
	Namespace             string
	ConfigurationType     types.ConfigurationType
	CompleteConfiguration string
	RemoteGit             string
	RemoteGitPath         string
	ConfigurationChanged  bool
	EnvChanged            bool
	ConfigurationCMName   string
	BackendSecretName     string
	ApplyJobName          string
	DestroyJobName        string
	Envs                  []v1.EnvVar
	ProviderReference     *crossplane.Reference
	VariableSecretName    string
	VariableSecretData    map[string]string
	DeleteResource        bool
	Credentials           map[string]string

	// TerraformImage is the Terraform image which can run `terraform init/plan/apply`
	TerraformImage            string
	TerraformBackendNamespace string
	BusyboxImage              string
	GitImage                  string

	// Resources series Variables are for Setting Compute Resources required by this container
	ResourcesLimitsCPU              string
	ResourcesLimitsCPUQuantity      resource.Quantity
	ResourcesLimitsMemory           string
	ResourcesLimitsMemoryQuantity   resource.Quantity
	ResourcesRequestsCPU            string
	ResourcesRequestsCPUQuantity    resource.Quantity
	ResourcesRequestsMemory         string
	ResourcesRequestsMemoryQuantity resource.Quantity
}

func initTFConfigurationMeta(req Request, configuration *types.Configuration) *TFConfigurationMeta {
	var Namespace, Name string

	NamespacedName := strings.Split(req.NamespacedName, "/")
	if len(NamespacedName) != 2 {
		Namespace = ""
		Name = ""
	} else {
		Namespace = NamespacedName[0]
		Name = NamespacedName[1]
	}
	var meta = &TFConfigurationMeta{
		Namespace:           Namespace,
		Name:                Name,
		ConfigurationCMName: fmt.Sprintf(TFInputConfigMapName, Name),
		VariableSecretName:  fmt.Sprintf(TFVariableSecret, Name),
		ApplyJobName:        Name + "-" + string(TerraformApply),
		DestroyJobName:      Name + "-" + string(TerraformDestroy),
	}

	// githubBlocked mark whether GitHub is blocked in the cluster
	githubBlockedStr := os.Getenv("GITHUB_BLOCKED")
	if githubBlockedStr == "" {
		githubBlockedStr = "false"
	}

	meta.RemoteGit = tfcfg.ReplaceTerraformSource(configuration.Spec.Remote, githubBlockedStr)
	meta.DeleteResource = configuration.Spec.DeleteResource
	if configuration.Spec.Path == "" {
		meta.RemoteGitPath = "."
	} else {
		meta.RemoteGitPath = configuration.Spec.Path
	}

	meta.ProviderReference = tfcfg.GetProviderNamespacedName(configuration)

	// Check the existence of Terraform state secret which is used to store TF state file. For detailed information,
	// please refer to https://www.terraform.io/docs/language/settings/backends/kubernetes.html#configuration-variables
	// Secrets will be named in the format: tfstate-{workspace}-{configuration.Name}
	meta.BackendSecretName = fmt.Sprintf(TFBackendSecret, terraformWorkspace, configuration.Name)

	return meta
}

func (r *ConfigurationReconciler) terraformApply(ctx context.Context, namespace string, configuration *types.Configuration, meta *TFConfigurationMeta) error {
	klog.InfoS("terraform apply job", "Namespace", namespace, "Name", meta.ApplyJobName)

	var (
		Client         = r.Client
		tfExecutionJob types.Job
	)

	key := "Job" + "/" + namespace + "/" + meta.ApplyJobName
	obj, _, err := Client.GetByKey(key)
	if err != nil {
		return meta.assembleAndTriggerJob(ctx, Client, TerraformApply)
	}
	tfExecutionJob = obj.(types.Job)

	if err := meta.updateTerraformJobIfNeeded(ctx, Client, tfExecutionJob); err != nil {
		klog.ErrorS(err, types.ErrUpdateTerraformApplyJob, "Name", meta.ApplyJobName)
		return errors.Wrap(err, types.ErrUpdateTerraformApplyJob)
	}

	if !meta.EnvChanged && tfExecutionJob.Status.Succeeded == int32(1) {
		if err := meta.updateApplyStatus(ctx, Client, types.Available, types.MessageCloudResourceDeployed); err != nil {
			return err
		}
	} else {
		// start provisioning and check the status of the provision
		// If the state is types.InvalidRegion, no need to continue checking
		if configuration.Status.Apply.State != types.ConfigurationProvisioningAndChecking &&
			configuration.Status.Apply.State != types.InvalidRegion {
			if err := meta.updateApplyStatus(ctx, r.Client, types.ConfigurationProvisioningAndChecking, types.MessageCloudResourceProvisioningAndChecking); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *ConfigurationReconciler) terraformDestroy(ctx context.Context, NamespacedName string, configuration *types.Configuration, meta *TFConfigurationMeta) error {
	var (
		destroyJob types.Job
		Client     = r.Client
		namespace  string
	)

	namespacedName := strings.Split(NamespacedName, "/")
	if len(namespacedName) != 2 {
		namespace = ""
	} else {
		namespace = namespacedName[0]
	}

	deletable, err := tfcfg.IsDeletable(ctx, Client, configuration)
	if err != nil {
		return err
	}

	deleteConfigurationDirectly := deletable || !meta.DeleteResource

	if !deleteConfigurationDirectly {
		key := "Configuration" + "/" + meta.Namespace + "/" + meta.DestroyJobName
		_, _, err := Client.GetByKey(key)
		if err != nil {
			configKey := "Configuration" + "/" + configuration.Namespace + "/" + configuration.Name
			_, _, err := Client.GetByKey(configKey)
			if err == nil {
				if err = meta.assembleAndTriggerJob(ctx, Client, TerraformDestroy); err != nil {
					return err
				}
			}
		}
		if err := meta.updateTerraformJobIfNeeded(ctx, Client, destroyJob); err != nil {
			klog.ErrorS(err, types.ErrUpdateTerraformApplyJob, "Name", meta.ApplyJobName)
			return errors.Wrap(err, types.ErrUpdateTerraformApplyJob)
		}
	}

	// destroying
	if err := meta.updateDestroyStatus(ctx, Client, types.ConfigurationDestroying, types.MessageCloudResourceDestroying); err != nil {
		return err
	}

	// When the deletion Job process succeeded, clean up work is starting.
	keyDestroyJob := "Job" + "/" + meta.Namespace + "/" + meta.DestroyJobName
	_, _, err = Client.GetByKey(keyDestroyJob)
	if err != nil {
		return err
	}
	if destroyJob.Status.Succeeded == int32(1) || deleteConfigurationDirectly {
		// 1. delete Terraform input Configuration ConfigMap
		if err := meta.deleteConfigMap(ctx, Client); err != nil {
			return err
		}

		// 2. delete connectionSecret
		if configuration.Spec.WriteConnectionSecretToReference != nil {
			secretName := configuration.Spec.WriteConnectionSecretToReference.Name
			secretNameSpace := configuration.Spec.WriteConnectionSecretToReference.Namespace
			if err := deleteConnectionSecret(ctx, Client, secretName, secretNameSpace); err != nil {
				return err
			}
		}

		// 3. delete apply job
		var applyJob types.Job
		keyApplyJob := "Job" + "/" + namespace + "/" + meta.ApplyJobName
		obj, _, err := Client.GetByKey(keyApplyJob)
		if err == nil {
			applyJob = obj.(types.Job)
			if err := Client.Delete(&applyJob); err != nil {
				return err
			}
		}

		// 4. delete destroy job
		var j types.Job
		keyDestroyJob = "Job" + "/" + namespace + "/" + meta.ApplyJobName
		obj, _, err = Client.GetByKey(keyDestroyJob)
		if err == nil {
			j = obj.(types.Job)
			if err := Client.Delete(&j); err != nil {
				return err
			}
		}

		// 5. delete secret which stores variables
		klog.InfoS("Deleting the secret which stores variables", "Name", meta.VariableSecretName)
		var variableSecret types.Secret
		keyVariableSecret := "Secret" + "/" + meta.Namespace + "/" + meta.VariableSecretName
		obj, _, err = Client.GetByKey(keyVariableSecret)
		if err == nil {
			variableSecret = obj.(types.Secret)
			if err := Client.Delete(&variableSecret); err != nil {
				return err
			}
		}

		// 6. delete Kubernetes backend secret
		klog.InfoS("Deleting the secret which stores Kubernetes backend", "Name", meta.BackendSecretName)
		var kubernetesBackendSecret types.Secret
		keyKubernetesBackendSecret := "Secret" + "/" + meta.TerraformBackendNamespace + "/" + meta.BackendSecretName
		obj, _, err = Client.GetByKey(keyKubernetesBackendSecret)
		if err == nil {
			kubernetesBackendSecret = obj.(types.Secret)
			if err := Client.Delete(&kubernetesBackendSecret); err != nil {
				return err
			}
		}
		return nil
	}
	return errors.New(types.MessageDestroyJobNotCompleted)
}

func (r *ConfigurationReconciler) preCheckResourcesSetting(meta *TFConfigurationMeta) error {

	meta.ResourcesLimitsCPU = os.Getenv("RESOURCES_LIMITS_CPU")
	if meta.ResourcesLimitsCPU != "" {
		limitsCPU, err := resource.ParseQuantity(meta.ResourcesLimitsCPU)
		if err != nil {
			errMsg := "failed to parse env variable RESOURCES_LIMITS_CPU into resource.Quantity"
			klog.ErrorS(err, errMsg)
			return errors.Wrap(err, errMsg)
		}
		meta.ResourcesLimitsCPUQuantity = limitsCPU
	}
	meta.ResourcesLimitsMemory = os.Getenv("RESOURCES_LIMITS_MEMORY")
	if meta.ResourcesLimitsMemory != "" {
		limitsMemory, err := resource.ParseQuantity(meta.ResourcesLimitsMemory)
		if err != nil {
			errMsg := "failed to parse env variable RESOURCES_LIMITS_MEMORY into resource.Quantity"
			klog.ErrorS(err, errMsg)
			return errors.Wrap(err, errMsg)
		}
		meta.ResourcesLimitsMemoryQuantity = limitsMemory
	}
	meta.ResourcesRequestsCPU = os.Getenv("RESOURCES_REQUESTS_CPU")
	if meta.ResourcesRequestsCPU != "" {
		requestsCPU, err := resource.ParseQuantity(meta.ResourcesRequestsCPU)
		if err != nil {
			errMsg := "failed to parse env variable RESOURCES_REQUESTS_CPU into resource.Quantity"
			klog.ErrorS(err, errMsg)
			return errors.Wrap(err, errMsg)
		}
		meta.ResourcesRequestsCPUQuantity = requestsCPU
	}
	meta.ResourcesRequestsMemory = os.Getenv("RESOURCES_REQUESTS_MEMORY")
	if meta.ResourcesRequestsMemory != "" {
		requestsMemory, err := resource.ParseQuantity(meta.ResourcesRequestsMemory)
		if err != nil {
			errMsg := "failed to parse env variable RESOURCES_REQUESTS_MEMORY into resource.Quantity"
			klog.ErrorS(err, errMsg)
			return errors.Wrap(err, errMsg)
		}
		meta.ResourcesRequestsMemoryQuantity = requestsMemory
	}
	return nil
}

func (r *ConfigurationReconciler) preCheck(ctx context.Context, configuration *types.Configuration, meta *TFConfigurationMeta) error {
	var storeClient = r.Client

	meta.TerraformImage = os.Getenv("TERRAFORM_IMAGE")
	if meta.TerraformImage == "" {
		meta.TerraformImage = "oamdev/docker-terraform:1.1.2"
	}

	meta.TerraformBackendNamespace = os.Getenv("TERRAFORM_BACKEND_NAMESPACE")
	if meta.TerraformBackendNamespace == "" {
		meta.TerraformBackendNamespace = "vela-system"
	}

	meta.BusyboxImage = os.Getenv("BUSYBOX_IMAGE")
	if meta.BusyboxImage == "" {
		meta.BusyboxImage = "busybox:latest"
	}
	meta.GitImage = os.Getenv("GIT_IMAGE")
	if meta.GitImage == "" {
		meta.GitImage = "alpine/git:latest"
	}

	if err := r.preCheckResourcesSetting(meta); err != nil {
		return err
	}

	// Validation: 1) validate Configuration itself
	configurationType, err := tfcfg.ValidConfigurationObject(configuration)
	if err != nil {
		if updateErr := meta.updateApplyStatus(ctx, storeClient, types.ConfigurationStaticCheckFailed, err.Error()); updateErr != nil {
			return updateErr
		}
		return err
	}
	meta.ConfigurationType = configurationType

	// TODO(zzxwill) Need to find an alternative to check whether there is an state backend in the Configuration

	// Render configuration with backend
	completeConfiguration, err := tfcfg.RenderConfiguration(configuration, meta.TerraformBackendNamespace, configurationType)
	if err != nil {
		return err
	}
	meta.CompleteConfiguration = completeConfiguration

	if err := meta.storeTFConfiguration(ctx, storeClient); err != nil {
		return err
	}

	// Check whether configuration(hcl/json) is changed
	if err := meta.CheckWhetherConfigurationChanges(ctx, storeClient, configurationType); err != nil {
		return err
	}

	if meta.ConfigurationChanged {
		klog.InfoS("Configuration hanged, reloading...")
		if err := meta.updateApplyStatus(ctx, storeClient, types.ConfigurationReloading, types.ConfigurationReloadingAsHCLChanged); err != nil {
			return err
		}
		// store configuration to ConfigMap
		return meta.storeTFConfiguration(ctx, storeClient)
	}

	// Check provider
	p, err := provider.GetProviderFromConfiguration(ctx, storeClient, meta.ProviderReference.Namespace, meta.ProviderReference.Name)
	if p == nil {
		msg := types.ErrProviderNotFound
		if err != nil {
			msg = err.Error()
		}
		if updateStatusErr := meta.updateApplyStatus(ctx, storeClient, types.Authorizing, msg); updateStatusErr != nil {
			return errors.Wrap(updateStatusErr, msg)
		}
		return errors.New(msg)
	}

	if err := meta.getCredentials(ctx, storeClient, p); err != nil {
		return err
	}

	// Check whether env changes
	if err := meta.prepareTFVariables(configuration); err != nil {
		return err
	}

	var variableInSecret types.Secret
	key := "Sercet" + "/" + meta.Namespace + "/" + meta.VariableSecretName
	obj, exist, err := storeClient.GetByKey(key)
	switch {
	case !exist:
		var secret = types.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      meta.VariableSecretName,
				Namespace: meta.Namespace,
			},
			TypeMeta: metav1.TypeMeta{Kind: "Secret"},
			Data:     meta.VariableSecretData,
		}

		if err := storeClient.Add(&secret); err != nil {
			return err
		}
	case err == nil:
		variableInSecret = obj.(types.Secret)
		for k, v := range meta.VariableSecretData {
			if val, ok := variableInSecret.Data[k]; !ok || !strings.EqualFold(v, val) {
				meta.EnvChanged = true
				klog.Info("Job's env changed")
				if err := meta.updateApplyStatus(ctx, storeClient, types.ConfigurationReloading, types.ConfigurationReloadingAsVariableChanged); err != nil {
					return err
				}
				break
			}
		}
	default:
		return err
	}

	// Apply ClusterRole
	return createTerraformExecutorClusterRole(ctx, storeClient, fmt.Sprintf("%s-%s", meta.Namespace, ClusterRoleName))
}

func (meta *TFConfigurationMeta) updateApplyStatus(ctx context.Context, Client cacheObj.Store, state types.ConfigurationState, message string) error {
	var configuration *types.Configuration
	key := "Configuration" + "/" + meta.Namespace + "/" + meta.Name
	obj, exists, err := Client.GetByKey(key)
	if err != nil || !exists {
		errMsg := "failed to get the configuration"
		klog.ErrorS(err, errMsg, "key", key)
		return nil
	}
	configuration = obj.(*types.Configuration)

	configuration.Status.Apply = types.ConfigurationApplyStatus{
		State:   state,
		Message: message,
	}
	configuration.Status.ObservedGeneration = configuration.Generation
	if state == types.Available {
		outputs, err := meta.getTFOutputs(ctx, Client, configuration)
		if err != nil {
			configuration.Status.Apply = types.ConfigurationApplyStatus{
				State:   types.GeneratingOutputs,
				Message: types.ErrGenerateOutputs + ": " + err.Error(),
			}
		} else {
			configuration.Status.Apply.Outputs = outputs
		}
	}
	return Client.Update(configuration)
}

func (meta *TFConfigurationMeta) updateDestroyStatus(ctx context.Context, Client cacheObj.Store, state types.ConfigurationState, message string) error {
	var configuration *types.Configuration
	key := "Configuration" + "/" + meta.Namespace + "/" + meta.Name
	obj, _, err := Client.GetByKey(key)
	if err == nil {
		configuration = obj.(*types.Configuration)
		configuration.Status.Destroy = types.ConfigurationDestroyStatus{
			State:   state,
			Message: message,
		}
		return Client.Update(configuration)
	}
	return nil
}

func (meta *TFConfigurationMeta) assembleAndTriggerJob(ctx context.Context, Client cacheObj.Store, executionType TerraformExecutionType) error {
	// apply rbac
	if err := createTerraformExecutorServiceAccount(ctx, Client, meta.Namespace, ServiceAccountName); err != nil {
		return err
	}
	if err := createTerraformExecutorClusterRoleBinding(ctx, Client, meta.Namespace, fmt.Sprintf("%s-%s", meta.Namespace, ClusterRoleName), ServiceAccountName); err != nil {
		return err
	}

	job := meta.assembleTerraformJob(executionType)
	return Client.Add(job)
}

// updateTerraformJob will set deletion finalizer to the Terraform job if its envs are changed, which will result in
// deleting the job. Finally, a new Terraform job will be generated
func (meta *TFConfigurationMeta) updateTerraformJobIfNeeded(ctx context.Context, Client cacheObj.Store, job types.Job) error {
	// if either one changes, delete the job
	if meta.EnvChanged || meta.ConfigurationChanged {
		klog.InfoS("about to delete job", "Name", job.Name, "Namespace", job.Namespace)
		keyJob := "Job" + "/" + job.Namespace + "/" + job.Name
		_, _, err := Client.GetByKey(keyJob)
		if err == nil {
			if deleteErr := Client.Delete(&job); deleteErr != nil {
				return deleteErr
			}
		}
		var s v1.Secret
		keySecret := "Secret" + "/" + meta.Namespace + "/" + meta.VariableSecretName
		obj, _, err := Client.GetByKey(keySecret)
		if err == nil {
			s = obj.(v1.Secret)
			if deleteErr := Client.Delete(&s); deleteErr != nil {
				return deleteErr
			}
		}
	}
	return nil
}

func (meta *TFConfigurationMeta) assembleTerraformJob(executionType TerraformExecutionType) *types.Job {
	var (
		initContainer           v1.Container
		tfPreApplyInitContainer v1.Container
		initContainers          []v1.Container
		parallelism             int32 = 1
		completions             int32 = 1
		backoffLimit            int32 = math.MaxInt32
	)

	executorVolumes := meta.assembleExecutorVolumes()
	initContainerVolumeMounts := []v1.VolumeMount{
		{
			Name:      meta.Name,
			MountPath: WorkingVolumeMountPath,
		},
		{
			Name:      InputTFConfigurationVolumeName,
			MountPath: InputTFConfigurationVolumeMountPath,
		},
		{
			Name:      BackendVolumeName,
			MountPath: BackendVolumeMountPath,
		},
	}

	// prepare local Terraform .tf files
	initContainer = v1.Container{
		Name:            "prepare-input-terraform-configurations",
		Image:           meta.BusyboxImage,
		ImagePullPolicy: v1.PullIfNotPresent,
		Command: []string{
			"sh",
			"-c",
			fmt.Sprintf("cp %s/* %s", InputTFConfigurationVolumeMountPath, WorkingVolumeMountPath),
		},
		VolumeMounts: initContainerVolumeMounts,
	}

	initContainers = append(initContainers, initContainer)

	hclPath := filepath.Join(BackendVolumeMountPath, meta.RemoteGitPath)

	if meta.RemoteGit != "" {
		initContainers = append(initContainers,
			v1.Container{
				Name:            "git-configuration",
				Image:           meta.GitImage,
				ImagePullPolicy: v1.PullIfNotPresent,
				Command: []string{
					"sh",
					"-c",
					fmt.Sprintf("git clone %s %s && cp -r %s/* %s", meta.RemoteGit, BackendVolumeMountPath,
						hclPath, WorkingVolumeMountPath),
				},
				VolumeMounts: initContainerVolumeMounts,
			})
	}

	// run `terraform init`
	tfPreApplyInitContainer = v1.Container{
		Name:            terraformInitContainerName,
		Image:           meta.TerraformImage,
		ImagePullPolicy: v1.PullIfNotPresent,
		Command: []string{
			"sh",
			"-c",
			"terraform init",
		},
		VolumeMounts: initContainerVolumeMounts,
	}
	initContainers = append(initContainers, tfPreApplyInitContainer)

	container := v1.Container{
		Name:            terraformContainerName,
		Image:           meta.TerraformImage,
		ImagePullPolicy: v1.PullIfNotPresent,
		Command: []string{
			"bash",
			"-c",
			fmt.Sprintf("terraform %s -lock=false -auto-approve", executionType),
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      meta.Name,
				MountPath: WorkingVolumeMountPath,
			},
			{
				Name:      InputTFConfigurationVolumeName,
				MountPath: InputTFConfigurationVolumeMountPath,
			},
		},
		Env: meta.Envs,
	}

	if meta.ResourcesLimitsCPU != "" || meta.ResourcesLimitsMemory != "" ||
		meta.ResourcesRequestsCPU != "" || meta.ResourcesRequestsMemory != "" {
		resourceRequirements := v1.ResourceRequirements{}
		if meta.ResourcesLimitsCPU != "" || meta.ResourcesLimitsMemory != "" {
			resourceRequirements.Limits = v1.ResourceList(map[v1.ResourceName]resource.Quantity{})
			if meta.ResourcesLimitsCPU != "" {
				resourceRequirements.Limits["cpu"] = meta.ResourcesLimitsCPUQuantity
			}
			if meta.ResourcesLimitsMemory != "" {
				resourceRequirements.Limits["memory"] = meta.ResourcesLimitsMemoryQuantity
			}
		}
		if meta.ResourcesRequestsCPU != "" || meta.ResourcesLimitsMemory != "" {
			resourceRequirements.Requests = v1.ResourceList(map[v1.ResourceName]resource.Quantity{})
			if meta.ResourcesRequestsCPU != "" {
				resourceRequirements.Requests["cpu"] = meta.ResourcesRequestsCPUQuantity
			}
			if meta.ResourcesRequestsMemory != "" {
				resourceRequirements.Requests["memory"] = meta.ResourcesRequestsMemoryQuantity
			}
		}
		container.Resources = resourceRequirements
	}

	return &types.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "batch/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name + "-" + string(executionType),
			Namespace: meta.Namespace,
		},
		Spec: types.JobSpec{
			Parallelism:  &parallelism,
			Completions:  &completions,
			BackoffLimit: &backoffLimit,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						// This annotation will prevent istio-proxy sidecar injection in the pods
						// as having the sidecar would have kept the Job in `Running` state and would
						// not transition to `Completed`
						"sidecar.istio.io/inject": "false",
					},
				},
				Spec: v1.PodSpec{
					// InitContainer will copy Terraform configuration files to working directory and create Terraform
					// state file directory in advance
					InitContainers: initContainers,
					// Container terraform-executor will first copy predefined terraform.d to working directory, and
					// then run terraform init/apply.
					Containers:         []v1.Container{container},
					ServiceAccountName: ServiceAccountName,
					Volumes:            executorVolumes,
					RestartPolicy:      v1.RestartPolicyOnFailure,
				},
			},
		},
	}
}

func (meta *TFConfigurationMeta) assembleExecutorVolumes() []v1.Volume {
	workingVolume := v1.Volume{Name: meta.Name}
	workingVolume.EmptyDir = &v1.EmptyDirVolumeSource{}
	inputTFConfigurationVolume := meta.createConfigurationVolume()
	tfBackendVolume := meta.createTFBackendVolume()
	return []v1.Volume{workingVolume, inputTFConfigurationVolume, tfBackendVolume}
}

func (meta *TFConfigurationMeta) createConfigurationVolume() v1.Volume {
	inputCMVolumeSource := v1.ConfigMapVolumeSource{}
	inputCMVolumeSource.Name = meta.ConfigurationCMName
	inputTFConfigurationVolume := v1.Volume{Name: InputTFConfigurationVolumeName}
	inputTFConfigurationVolume.ConfigMap = &inputCMVolumeSource
	return inputTFConfigurationVolume

}

func (meta *TFConfigurationMeta) createTFBackendVolume() v1.Volume {
	gitVolume := v1.Volume{Name: BackendVolumeName}
	gitVolume.EmptyDir = &v1.EmptyDirVolumeSource{}
	return gitVolume
}

// TfStateProperty is the tf state property for an output
type TfStateProperty struct {
	Value interface{} `json:"value,omitempty"`
	Type  interface{} `json:"type,omitempty"`
}

// ToProperty converts TfStateProperty type to Property
func (tp *TfStateProperty) ToProperty() (types.Property, error) {
	var (
		property types.Property
		err      error
	)
	sv, err := tfcfg.Interface2String(tp.Value)
	if err != nil {
		return property, errors.Wrapf(err, "failed to convert value %s of terraform state outputs to string", tp.Value)
	}
	property = types.Property{
		Value: sv,
	}
	return property, err
}

// TFState is Terraform State
type TFState struct {
	Outputs map[string]TfStateProperty `json:"outputs"`
}

func (meta *TFConfigurationMeta) getTFOutputs(ctx context.Context, Client cacheObj.Store, configuration *types.Configuration) (map[string]types.Property, error) {
	var s = types.Secret{}

	key := "Secret" + "/" + meta.TerraformBackendNamespace + "/" + meta.BackendSecretName
	obj, exists, err := Client.GetByKey(key)
	if err != nil || !exists {
		errMsg := "terraform state file backend secret is not generated"
		klog.ErrorS(err, errMsg, "key", key)
		return nil, errors.Wrap(err, errMsg)
	}
	s = obj.(types.Secret)
	tfStateData, ok := s.Data[TerraformStateNameInSecret]
	if !ok {
		return nil, fmt.Errorf("failed to get %s from Terraform State secret %s", TerraformStateNameInSecret, s.Name)
	}

	tfStateJSON, err := util.DecompressTerraformStateSecret(string(tfStateData))
	if err != nil {
		return nil, errors.Wrap(err, "failed to decompress state secret data")
	}

	var tfState TFState
	if err := json.Unmarshal(tfStateJSON, &tfState); err != nil {
		return nil, err
	}
	outputs := make(map[string]types.Property)
	for k, v := range tfState.Outputs {
		property, err := v.ToProperty()
		if err != nil {
			return outputs, err
		}
		outputs[k] = property
	}
	writeConnectionSecretToReference := configuration.Spec.WriteConnectionSecretToReference
	if writeConnectionSecretToReference == nil || writeConnectionSecretToReference.Name == "" {
		return outputs, nil
	}

	name := writeConnectionSecretToReference.Name
	ns := writeConnectionSecretToReference.Namespace
	if ns == "" {
		ns = "default"
	}
	data := make(map[string]string)
	for k, v := range outputs {
		data[k] = v.Value
	}
	var gotSecret *types.Secret
	configurationName := configuration.ObjectMeta.Name
	key = "Secret" + "/" + ns + "/" + name
	obj, exists, err = Client.GetByKey(key)
	if err != nil || !exists {
		var secret = types.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
				Labels: map[string]string{
					"terraform.core.oam.dev/created-by":      "terraform-controller",
					"terraform.core.oam.dev/owned-by":        configurationName,
					"terraform.core.oam.dev/owned-namespace": configuration.Namespace,
				},
			},
			TypeMeta: metav1.TypeMeta{Kind: "Secret"},
			Data:     data,
		}
		err = Client.Add(&secret)
		if err != nil {
			return nil, fmt.Errorf("secret(%s) already exists", name)
		}
	} else {
		gotSecret = obj.(*types.Secret)
		// check the owner of this secret
		labels := gotSecret.ObjectMeta.Labels
		ownerName := labels["terraform.core.oam.dev/owned-by"]
		ownerNamespace := labels["terraform.core.oam.dev/owned-namespace"]
		if (ownerName != "" && ownerName != configurationName) ||
			(ownerNamespace != "" && ownerNamespace != configuration.Namespace) {
			errMsg := fmt.Sprintf(
				"configuration(namespace: %s ; name: %s) cannot update secret(namespace: %s ; name: %s) whose owner is configuration(namespace: %s ; name: %s)",
				configuration.Namespace, configurationName,
				gotSecret.Namespace, name,
				ownerNamespace, ownerName,
			)
			return nil, errors.New(errMsg)
		}
		gotSecret.Data = data
		if err := Client.Update(gotSecret); err != nil {
			return nil, err
		}
	}
	return outputs, nil
}

func (meta *TFConfigurationMeta) prepareTFVariables(configuration *types.Configuration) error {
	var (
		envs []v1.EnvVar
		data = map[string]string{}
	)

	if configuration == nil {
		return errors.New("configuration is nil")
	}
	if meta.ProviderReference == nil {
		return errors.New("The referenced provider could not be retrieved")
	}

	tfVariable, err := getTerraformJSONVariable(configuration.Spec.Variable)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to get Terraform JSON variables from Configuration Variables %v", configuration.Spec.Variable))
	}
	for k, v := range tfVariable {
		envValue, err := tfcfg.Interface2String(v)
		if err != nil {
			return err
		}
		data[k] = envValue
		valueFrom := &v1.EnvVarSource{SecretKeyRef: &v1.SecretKeySelector{Key: k}}
		valueFrom.SecretKeyRef.Name = meta.VariableSecretName
		envs = append(envs, v1.EnvVar{Name: k, ValueFrom: valueFrom})
	}

	if meta.Credentials == nil {
		return errors.New(provider.ErrCredentialNotRetrieved)
	}
	for k, v := range meta.Credentials {
		data[k] = v
		valueFrom := &v1.EnvVarSource{SecretKeyRef: &v1.SecretKeySelector{Key: k}}
		valueFrom.SecretKeyRef.Name = meta.VariableSecretName
		envs = append(envs, v1.EnvVar{Name: k, ValueFrom: valueFrom})
	}
	// make sure the env of the Job is set
	if envs == nil {
		return errors.New(provider.ErrCredentialNotRetrieved)
	}
	meta.Envs = envs
	meta.VariableSecretData = data

	return nil
}

func getTerraformJSONVariable(tfVariables *runtime.RawExtension) (map[string]interface{}, error) {
	variables, err := tfcfg.RawExtension2Map(tfVariables)
	if err != nil {
		return nil, err
	}
	var environments = make(map[string]interface{})

	for k, v := range variables {
		environments[fmt.Sprintf("TF_VAR_%s", k)] = v
	}
	return environments, nil
}

func (meta *TFConfigurationMeta) deleteConfigMap(ctx context.Context, Client cacheObj.Store) error {
	var cm types.ConfigMap
	key := "ConfigMap" + "/" + meta.Namespace + "/" + meta.ConfigurationCMName
	obj, _, err := Client.GetByKey(key)
	if err == nil {
		cm = obj.(types.ConfigMap)
		if err := Client.Delete(&cm); err != nil {
			return err
		}
	}
	return nil
}

func deleteConnectionSecret(ctx context.Context, Client cacheObj.Store, name, ns string) error {
	if len(name) == 0 {
		return nil
	}

	var connectionSecret v1.Secret
	if len(ns) == 0 {
		ns = "default"
	}
	key := "Secret" + "/" + ns + "/" + name
	obj, _, err := Client.GetByKey(key)
	if err == nil {
		connectionSecret = obj.(v1.Secret)
		return Client.Delete(&connectionSecret)
	}
	return nil
}

func (meta *TFConfigurationMeta) createOrUpdateConfigMap(ctx context.Context, Client cacheObj.Store, data map[string]string) error {
	var gotCM *types.ConfigMap
	key := "ConfigMap" + "/" + meta.Namespace + "/" + meta.ConfigurationCMName
	obj, exists, err := Client.GetByKey(key)
	if err != nil || !exists {
		cm := types.ConfigMap{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      meta.ConfigurationCMName,
				Namespace: meta.Namespace,
			},
			Data: data,
		}
		err := Client.Add(&cm)
		return errors.Wrap(err, "failed to create TF configuration ConfigMap")
	}
	gotCM = obj.(*types.ConfigMap)
	if !reflect.DeepEqual(gotCM.Data, data) {
		gotCM.Data = data
		return errors.Wrap(Client.Update(gotCM), "failed to update TF configuration ConfigMap")
	}
	return nil
}

func (meta *TFConfigurationMeta) prepareTFInputConfigurationData() map[string]string {
	var dataName string
	switch meta.ConfigurationType {
	case types.ConfigurationHCL:
		dataName = types.TerraformHCLConfigurationName
	case types.ConfigurationRemote:
		dataName = "terraform-backend.tf"
	}
	data := map[string]string{dataName: meta.CompleteConfiguration, "kubeconfig": ""}
	return data
}

// storeTFConfiguration will store Terraform configuration to ConfigMap
func (meta *TFConfigurationMeta) storeTFConfiguration(ctx context.Context, Client cacheObj.Store) error {
	data := meta.prepareTFInputConfigurationData()
	return meta.createOrUpdateConfigMap(ctx, Client, data)
}

// CheckWhetherConfigurationChanges will check whether configuration is changed
func (meta *TFConfigurationMeta) CheckWhetherConfigurationChanges(ctx context.Context, Client cacheObj.Store, configurationType types.ConfigurationType) error {
	var cm *types.ConfigMap
	key := "ConfigMap" + "/" + meta.Namespace + "/" + meta.ConfigurationCMName
	obj, exists, err := Client.GetByKey(key)
	if err != nil || !exists {
		return err
	}
	cm = obj.(*types.ConfigMap)

	var configurationChanged bool
	switch configurationType {
	case types.ConfigurationHCL:
		configurationChanged = cm.Data[types.TerraformHCLConfigurationName] != meta.CompleteConfiguration
		meta.ConfigurationChanged = configurationChanged
		if configurationChanged {
			klog.InfoS("Configuration HCL changed", "ConfigMap", cm.Data[types.TerraformHCLConfigurationName],
				"RenderedCompletedConfiguration", meta.CompleteConfiguration)
		}

		return nil
	case types.ConfigurationRemote:
		meta.ConfigurationChanged = false
		return nil
	default:
		return errors.New("unsupported configuration type, only HCL or Remote is supported")
	}
}

// getCredentials will get credentials from secret of the Provider
func (meta *TFConfigurationMeta) getCredentials(ctx context.Context, Client cacheObj.Store, providerObj *types.Provider) error {
	region, err := tfcfg.SetRegion(ctx, Client, meta.Namespace, meta.Name, providerObj)
	if err != nil {
		return err
	}
	credentials, err := provider.GetProviderCredentials(ctx, Client, providerObj, region)
	if err != nil {
		return err
	}
	if credentials == nil {
		return errors.New(provider.ErrCredentialNotRetrieved)
	}
	meta.Credentials = credentials
	return nil
}
