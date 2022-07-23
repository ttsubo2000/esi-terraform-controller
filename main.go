package main

import (
	"os"
	"time"

	"k8s.io/klog/v2"

	crossplanetypes "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/ttsubo2000/esi-terraform-worker/controllers"
	"github.com/ttsubo2000/esi-terraform-worker/manager"
	cacheObj "github.com/ttsubo2000/esi-terraform-worker/tools/cache"
	"github.com/ttsubo2000/esi-terraform-worker/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runTime "k8s.io/apimachinery/pkg/runtime"
)

func main() {
	clientState := cacheObj.NewStore(cacheObj.MetaNamespaceKeyFunc)

	go func() {
		time.Sleep(1 * time.Second)
		obj := &types.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind: "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gcp-account-creds",
				Namespace: "vela-system",
			},
			Data: map[string]string{
				"credentials": "{}",
			},
		}
		clientState.Add(obj)
	}()

	go func() {
		time.Sleep(5 * time.Second)
		obj := &types.Configuration{
			TypeMeta: metav1.TypeMeta{
				Kind: "Configuration",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "Configuration",
				Namespace: "default",
			},
			Spec: types.ConfigurationSpec{
				HCL: "resource \"google_storage_bucket\" \"bucket\" {\n  name = var.bucket\n}\n\noutput \"BUCKET_URL\" {\n  value = google_storage_bucket.bucket.url\n}\n\nvariable \"bucket\" {\n  default = \"vela-website\"\n}\n",
				Variable: &runTime.RawExtension{
					Raw: []byte(`{"bucket":"vela-website", "acl":"private"}`),
				},
				Backend: &types.Backend{
					Path: "/tmp/terraform.tfstate",
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
				Name:      "default",
				Namespace: "default",
			},
			Spec: types.ProviderSpec{
				Provider: "gcp",
				Region:   "us-central1",
				Credentials: types.ProviderCredentials{
					Source: "Secret",
					SecretRef: crossplanetypes.SecretKeySelector{
						SecretReference: crossplanetypes.SecretReference{
							Name:      "gcp-account-creds",
							Namespace: "vela-system",
						},
						Key: "credentials",
					},
				},
			},
		}
		clientState.Add(obj)
	}()

	mgr := manager.NewManager()
	mgr.Add(controllers.NewController("secret", &controllers.SecretReconciler{Client: clientState}, &types.Secret{}, clientState))
	mgr.Add(controllers.NewController("provider", &controllers.ProviderReconciler{Client: clientState}, &types.Provider{}, clientState))
	mgr.Add(controllers.NewController("configuration", &controllers.ConfigurationReconciler{Client: clientState}, &types.Configuration{}, clientState))
	mgr.Add(controllers.NewController("job", &controllers.JobReconciler{Client: clientState}, &types.Job{}, clientState))
	if err := mgr.Start(manager.SetupSignalHandler()); err != nil {
		klog.Error(err, "problem controller")
		os.Exit(1)
	}
}
