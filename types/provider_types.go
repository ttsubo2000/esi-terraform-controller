package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runTime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	crossplanetypes "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
)

// ProviderSpec defines the desired state of Provider.
type ProviderSpec struct {
	// Provider is the cloud service provider, like `alibaba`
	Provider string `json:"provider"`

	// Region is cloud provider's region
	Region string `json:"region,omitempty"`

	// Credentials required to authenticate to this provider.
	Credentials ProviderCredentials `json:"credentials"`
}

// ProviderCredentials required to authenticate.
type ProviderCredentials struct {
	// Source of the provider credentials.
	Source crossplanetypes.CredentialsSource `json:"source"`

	// A SecretRef is a reference to a secret key that contains the credentials
	// that must be used to connect to the provider.
	SecretRef crossplanetypes.SecretKeySelector `json:"secretRef,omitempty"`
}

// ProviderStatus defines the observed state of Provider.
type ProviderStatus struct {
	State   ProviderState `json:"state,omitempty"`
	Message string        `json:"message,omitempty"`
}

// Provider is the Schema for the providers API.
type Provider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProviderSpec   `json:"spec,omitempty"`
	Status ProviderStatus `json:"status,omitempty"`
}

func (p *Provider) DeepCopyObject() runTime.Object {
	panic("not supported")
}

func (p *Provider) GetObjectKind() schema.ObjectKind {
	return &p.TypeMeta
}

// Costomized for this original program
func (p *Provider) GetGenerateName() string {
	return p.TypeMeta.Kind
}
