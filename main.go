package main

import (
	"os"
	"time"

	"k8s.io/klog/v2"

	"github.com/ttsubo/client-go/tools/cache"
	"github.com/ttsubo2000/esi-terraform-worker/controllers"
	"github.com/ttsubo2000/esi-terraform-worker/manager"
	"github.com/ttsubo2000/esi-terraform-worker/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {
	informerConfigChan := make(chan cache.Controller, 1)
	informerProviderChan := make(chan cache.Controller, 1)

	go func() {
		informer := <-informerConfigChan
		obj := &types.Configuration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "myConfiguration",
				Namespace: v1.NamespaceDefault,
			},
		}
		informer.InjectWorkerQueue(obj)
	}()

	go func() {
		informer := <-informerProviderChan
		obj := &types.Provider{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "myProvider",
				Namespace: v1.NamespaceDefault,
			},
		}
		informer.InjectWorkerQueue(obj)
	}()

	mgr := manager.NewManager()
	mgr.Add(controllers.NewController("configuration", &controllers.ConfigurationReconciler{}, informerConfigChan))
	mgr.Add(controllers.NewController("provider", &controllers.ProviderReconciler{}, informerProviderChan))
	if err := mgr.Start(manager.SetupSignalHandler()); err != nil {
		klog.Error(err, "problem controller")
		os.Exit(1)
	}
	time.Sleep(1 * time.Second)
}
