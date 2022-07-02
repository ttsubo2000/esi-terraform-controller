package main

import (
	"os"
	"time"

	"k8s.io/klog/v2"

	crossplanetypes "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/ttsubo/client-go/tools/cache"
	"github.com/ttsubo2000/esi-terraform-worker/controllers"
	"github.com/ttsubo2000/esi-terraform-worker/manager"
	cacheObj "github.com/ttsubo2000/esi-terraform-worker/tools/cache"
	"github.com/ttsubo2000/esi-terraform-worker/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {
	clientState := cacheObj.NewStore(cacheObj.MetaNamespaceKeyFunc)
	informerConfigChan := make(chan cache.Controller, 1)
	informerProviderChan := make(chan cache.Controller, 1)
	informerSecretChan := make(chan cache.Controller, 1)

	go func() {
		informer := <-informerSecretChan
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
		informer.InjectWorkerQueue(obj)
	}()

	go func() {
		informer := <-informerConfigChan
		time.Sleep(1 * time.Second)
		obj := &types.Configuration{
			TypeMeta: metav1.TypeMeta{
				Kind: "Configuration",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "myConfiguration",
				Namespace: v1.NamespaceDefault,
			},
			Spec: types.ConfigurationSpec{
				HCL: "resource \"google_storage_bucket\" \"bucket\" {\n  name = var.bucket\n}\n\noutput \"BUCKET_URL\" {\n  value = google_storage_bucket.bucket.url\n}\n\nvariable \"bucket\" {\n  default = \"vela-website\"\n}\n",
				Backend: &types.Backend{
					SecretSuffix:    "oss",
					InClusterConfig: true,
				},
			},
		}
		clientState.Add(obj)
		informer.InjectWorkerQueue(obj)
	}()

	go func() {
		informer := <-informerProviderChan
		obj := &types.Provider{
			TypeMeta: metav1.TypeMeta{
				Kind: "Provider",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: v1.NamespaceDefault,
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
		time.Sleep(1 * time.Second)
		clientState.Add(obj)
		informer.InjectWorkerQueue(obj)
	}()

	mgr := manager.NewManager()
	mgr.Add(controllers.NewController("configuration", &controllers.ConfigurationReconciler{Client: clientState}, informerConfigChan))
	mgr.Add(controllers.NewController("provider", &controllers.ProviderReconciler{Client: clientState}, informerProviderChan))
	mgr.Add(controllers.NewController("secret", &controllers.SecretReconciler{Client: clientState}, informerSecretChan))
	if err := mgr.Start(manager.SetupSignalHandler()); err != nil {
		klog.Error(err, "problem controller")
		os.Exit(1)
	}
	time.Sleep(1 * time.Second)
}
