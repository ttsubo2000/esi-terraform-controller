package main

import (
	"os"
	"time"

	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"

	crossplanetypes "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/ttsubo2000/esi-terraform-worker/controllers"
	"github.com/ttsubo2000/esi-terraform-worker/manager"
	cacheObj "github.com/ttsubo2000/esi-terraform-worker/tools/cache"
	"github.com/ttsubo2000/esi-terraform-worker/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runTime "k8s.io/apimachinery/pkg/runtime"
)

const hclContent = `|-
  resource "hashicups_order" "edu" {
    items {
      coffee {
        id = 3
      }
      quantity = 2
    }
    items {
      coffee {
        id = 2
      }
      quantity = 2
    }
  }
 
  output "edu_order" {
    value = hashicups_order.edu
  }
`

func main() {
	clientState := cacheObj.NewStore(cacheObj.MetaNamespaceKeyFunc)

	go func() {
		time.Sleep(1 * time.Second)
		obj := &types.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind: "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hashicups-account-creds",
				Namespace: "hashicups",
			},
			Data: map[string]string{
				"credentials": "HashicupsUser: education\nHashicupsPassword: test123",
			},
		}
		clientState.Add(obj)
	}()

	go func() {
		time.Sleep(5 * time.Second)
		var body interface{}
		if err := yaml.Unmarshal([]byte(hclContent), &body); err != nil {
			panic(err)
		}

		hclContext := body.(string)
		klog.Infof("### HCL=[%s]\n", hclContext)

		obj := &types.Configuration{
			TypeMeta: metav1.TypeMeta{
				Kind: "Configuration",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "Configuration",
				Namespace: "default",
			},
			Spec: types.ConfigurationSpec{
				HCL: hclContext,
				Variable: &runTime.RawExtension{
					Raw: []byte(`{"variable1":"hoge", "variable2":"fuga"}`),
				},
				Backend: &types.Backend{
					Path: "/tmp/terraform.tfstate",
				},
				BaseConfigurationSpec: types.BaseConfigurationSpec{
					ProviderReference: &crossplanetypes.Reference{
						Name:      "hashicups",
						Namespace: "default",
					},
				},
			},
		}
		clientState.Add(obj)
	}()

	go func() {
		time.Sleep(2 * time.Second)
		obj := &types.Provider{
			TypeMeta: metav1.TypeMeta{
				Kind: "Provider",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hashicups",
				Namespace: "default",
			},
			Spec: types.ProviderSpec{
				Provider: "hashicups",
				Credentials: types.ProviderCredentials{
					Source: "Secret",
					SecretRef: crossplanetypes.SecretKeySelector{
						SecretReference: crossplanetypes.SecretReference{
							Name:      "hashicups-account-creds",
							Namespace: "hashicups",
						},
						Key: "credentials",
					},
				},
			},
		}
		clientState.Add(obj)
	}()

	mgr := manager.NewManager()
	mgr.Add(controllers.NewController("provider", &controllers.ProviderReconciler{Client: clientState}, &types.Provider{}, clientState))
	mgr.Add(controllers.NewController("configuration", &controllers.ConfigurationReconciler{Client: clientState}, &types.Configuration{}, clientState))
	if err := mgr.Start(manager.SetupSignalHandler()); err != nil {
		klog.Error(err, "problem controller")
		os.Exit(1)
	}
}
