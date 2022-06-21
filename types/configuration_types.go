package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runTime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	state "github.com/oam-dev/terraform-controller/api/types"
	types "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
)

// ConfigurationSpec defines the desired state of Configuration
type ConfigurationSpec struct {
	// HCL is the Terraform HCL type configuration
	HCL string `json:"hcl,omitempty"`

	// Remote is a git repo which contains hcl files. Currently, only public git repos are supported.
	Remote string `json:"remote,omitempty"`

	// +kubebuilder:pruning:PreserveUnknownFields
	Variable runTime.RawExtension `json:"variable,omitempty"`

	// Backend stores the state in a Kubernetes secret with locking done using a Lease resource.
	// TODO(zzxwill) If a backend exists in HCL/JSON, this can be optional. Currently, if Backend is not set by users, it
	// still will set by the controller, ignoring the settings in HCL/JSON backend
	Backend Backend `json:"backend,omitempty"`

	// Path is the sub-directory of remote git repository.
	Path string `json:"path,omitempty"`

	BaseConfigurationSpec `json:",inline"`
}

// BaseConfigurationSpec defines the common fields of a ConfigurationSpec
type BaseConfigurationSpec struct {
	// WriteConnectionSecretToReference specifies the namespace and name of a
	// Secret to which any connection details for this managed resource should
	// be written. Connection details frequently include the endpoint, username,
	// and password required to connect to the managed resource.
	WriteConnectionSecretToReference *types.SecretReference `json:"writeConnectionSecretToRef,omitempty"`

	// ProviderReference specifies the reference to Provider
	ProviderReference *types.Reference `json:"providerRef,omitempty"`

	// DeleteResource will determine whether provisioned cloud resources will be deleted when CR is deleted
	DeleteResource bool `json:"deleteResource,omitempty"`

	// Region is cloud provider's region. It will override the region in the region field of ProviderReference
	Region string `json:"customRegion,omitempty"`
}

// ConfigurationStatus defines the observed state of Configuration
type ConfigurationStatus struct {
	// observedGeneration is the most recent generation observed for this Configuration. It corresponds to the
	// Configuration's generation, which is updated on mutation by the API Server.
	// If ObservedGeneration equals Generation, and State is Available, the value of Outputs is latest
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	Apply   ConfigurationApplyStatus   `json:"apply,omitempty"`
	Destroy ConfigurationDestroyStatus `json:"destroy,omitempty"`
}

// ConfigurationApplyStatus is the status for Configuration apply
type ConfigurationApplyStatus struct {
	State   state.ConfigurationState `json:"state,omitempty"`
	Message string                   `json:"message,omitempty"`
	Outputs map[string]Property      `json:"outputs,omitempty"`
}

// ConfigurationDestroyStatus is the status for Configuration destroy
type ConfigurationDestroyStatus struct {
	State   state.ConfigurationState `json:"state,omitempty"`
	Message string                   `json:"message,omitempty"`
}

// Property is the property for an output
type Property struct {
	Value string `json:"value,omitempty"`
}

// Backend stores the state in a Kubernetes secret with locking done using a Lease resource.
type Backend struct {
	// SecretSuffix used when creating secrets. Secrets will be named in the format: tfstate-{workspace}-{secretSuffix}
	SecretSuffix string `json:"secretSuffix,omitempty"`
	// InClusterConfig Used to authenticate to the cluster from inside a pod. Only `true` is allowed
	InClusterConfig bool `json:"inClusterConfig,omitempty"`
}

// Configuration is the Schema for the configurations API
type Configuration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigurationSpec   `json:"spec,omitempty"`
	Status ConfigurationStatus `json:"status,omitempty"`
}

func (c *Configuration) DeepCopyObject() runTime.Object {
	panic("not supported")
}

func (c *Configuration) GetObjectKind() schema.ObjectKind {
	return &c.TypeMeta
}

// Costomized for this original program
func (c *Configuration) GetGenerateName() string {
	return c.TypeMeta.Kind
}
