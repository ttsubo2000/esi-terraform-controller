package provider

import (
	"context"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/sts"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	cacheObj "github.com/ttsubo2000/terraform-controller/tools/cache"
	"github.com/ttsubo2000/terraform-controller/types"
)

const (
	// DefaultName is the name of Provider object
	DefaultName = "default"
	// DefaultNamespace is the namespace of Provider object
	DefaultNamespace = "default"
)

// CloudProvider is a type for mark a Cloud Provider
type CloudProvider string

const (
	alibaba   CloudProvider = "alibaba"
	aws       CloudProvider = "aws"
	gcp       CloudProvider = "gcp"
	tencent   CloudProvider = "tencent"
	azure     CloudProvider = "azure"
	vsphere   CloudProvider = "vsphere"
	ec        CloudProvider = "ec"
	ucloud    CloudProvider = "ucloud"
	custom    CloudProvider = "custom"
	baidu     CloudProvider = "baidu"
	hashicups CloudProvider = "hashicups"
)

const (
	envAlicloudAcessKey  = "ALICLOUD_ACCESS_KEY"
	envAlicloudSecretKey = "ALICLOUD_SECRET_KEY"
	envAlicloudRegion    = "ALICLOUD_REGION"
	envAliCloudStsToken  = "ALICLOUD_SECURITY_TOKEN"

	errConvertCredentials     = "failed to convert the credentials of Secret from Provider"
	errCredentialValid        = "Credentials are not valid"
	ErrCredentialNotRetrieved = "Credentials are not retrieved from referenced Provider"
)

// AlibabaCloudCredentials are credentials for Alibaba Cloud
type AlibabaCloudCredentials struct {
	AccessKeyID     string `yaml:"accessKeyID"`
	AccessKeySecret string `yaml:"accessKeySecret"`
	SecurityToken   string `yaml:"securityToken"`
}

// GetProviderCredentials gets provider credentials by cloud provider name
func GetProviderCredentials(ctx context.Context, Client cacheObj.Store, provider *types.Provider, region string) (map[string]string, error) {
	switch provider.Spec.Credentials.Source {
	case "Secret":
		var secret *types.Secret
		secretRef := provider.Spec.Credentials.SecretRef
		name := secretRef.Name
		namespace := secretRef.Namespace
		key := "Secret" + "/" + namespace + "/" + name
		obj, exists, err := Client.GetByKey(key)
		if err != nil || !exists {
			errMsg := "failed to get the Secret from Provider"
			klog.ErrorS(err, errMsg, "key", key)
			return nil, errors.Wrap(err, errMsg)
		}
		secret = obj.(*types.Secret)
		secretData, ok := secret.Data[secretRef.Key]
		if !ok {
			return nil, errors.Errorf("in the provider %s, the key %s not found in the referenced secret %s", provider.Name, secretRef.Key, name)
		}
		SecretDataByte := []byte(secretData)
		switch provider.Spec.Provider {
		case string(alibaba):
			var ak AlibabaCloudCredentials
			if err := yaml.Unmarshal([]byte(secret.Data[secretRef.Key]), &ak); err != nil {
				klog.ErrorS(err, errConvertCredentials, "Name", name, "Namespace", namespace)
				return nil, errors.Wrap(err, errConvertCredentials)
			}
			if err := checkAlibabaCloudCredentials(region, ak.AccessKeyID, ak.AccessKeySecret, ak.SecurityToken); err != nil {
				klog.ErrorS(err, errCredentialValid)
				return nil, errors.Wrap(err, errCredentialValid)
			}
			return map[string]string{
				envAlicloudAcessKey:  ak.AccessKeyID,
				envAlicloudSecretKey: ak.AccessKeySecret,
				envAlicloudRegion:    region,
				envAliCloudStsToken:  ak.SecurityToken,
			}, nil
		case string(ucloud):
			return getUCloudCredentials(SecretDataByte, name, namespace)
		case string(aws):
			return getAWSCredentials(SecretDataByte, name, namespace, region)
		case string(gcp):
			return getGCPCredentials(SecretDataByte, name, namespace, region)
		case string(tencent):
			return getTencentCloudCredentials(SecretDataByte, name, namespace, region)
		case string(azure):
			return getAzureCredentials(SecretDataByte, name, namespace)
		case string(vsphere):
			return getVSphereCredentials(SecretDataByte, name, namespace)
		case string(ec):
			return getECCloudCredentials(SecretDataByte, name, namespace)
		case string(custom):
			return getCustomCredentials(SecretDataByte, name, namespace)
		case string(baidu):
			return getBaiduCloudCredentials(SecretDataByte, name, namespace, region)
		case string(hashicups):
			return getHashicupsCredentials(SecretDataByte, name, namespace)
		default:
			errMsg := "unsupported provider"
			klog.InfoS(errMsg, "Provider", provider.Spec.Provider)
			return nil, errors.New(errMsg)
		}
	default:
		errMsg := "the credentials type is not supported."
		err := errors.New(errMsg)
		klog.ErrorS(err, "", "CredentialType", provider.Spec.Credentials.Source)
		return nil, err
	}
}

// GetProviderFromConfiguration gets provider object from Configuration
// Returns:
// 1) (nil, err): hit an issue to find the provider
// 2) (nil, nil): provider not found
// 3) (provider, nil): provider found
func GetProviderFromConfiguration(ctx context.Context, Client cacheObj.Store, namespace, name string) (*types.Provider, error) {
	var provider = &types.Provider{}
	key := "Provider" + "/" + namespace + "/" + name
	obj, exists, err := Client.GetByKey(key)
	if err != nil || !exists {
		if !exists {
			return nil, nil
		}
		errMsg := "failed to get Provider object"
		klog.ErrorS(err, errMsg, "Name", name)
		return nil, errors.Wrap(err, errMsg)
	}
	provider = obj.(*types.Provider)
	return provider, nil
}

// checkAlibabaCloudProvider checks if the credentials from the provider are valid
func checkAlibabaCloudCredentials(region string, accessKeyID, accessKeySecret, stsToken string) error {
	var (
		client *sts.Client
		err    error
	)
	if stsToken != "" {
		client, err = sts.NewClientWithStsToken(region, accessKeyID, accessKeySecret, stsToken)
	} else {
		client, err = sts.NewClientWithAccessKey(region, accessKeyID, accessKeySecret)
	}
	if err != nil {
		return err
	}
	request := sts.CreateGetCallerIdentityRequest()
	request.Scheme = "https"

	_, err = client.GetCallerIdentity(request)
	if err != nil {
		errMsg := "Alibaba Cloud credentials are invalid"
		klog.ErrorS(err, errMsg)
		return errors.Wrap(err, errMsg)
	}
	return nil
}
