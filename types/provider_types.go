package types

import (
	"github.com/oam-dev/terraform-controller/api/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	crossplanetypes "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
)

// ProviderCredentials required to authenticate.
type ProviderCredentials struct {
	// Source of the provider credentials.
	Source crossplanetypes.CredentialsSource `json:"source"`

	// A SecretRef is a reference to a secret key that contains the credentials
	// that must be used to connect to the provider.
	SecretRef *crossplanetypes.SecretKeySelector `json:"secretRef,omitempty"`
}

// ProviderStatus defines the observed state of Provider.
type ProviderStatus struct {
	State   types.ProviderState `json:"state,omitempty"`
	Message string              `json:"message,omitempty"`
}

// Provider is the Schema for the providers API.
type Provider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProviderSpec   `json:"spec,omitempty"`
	Status ProviderStatus `json:"status,omitempty"`
}
