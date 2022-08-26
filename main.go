package main

import (
	"os"

	"k8s.io/klog/v2"

	"github.com/ttsubo2000/esi-terraform-worker/controllers"
	"github.com/ttsubo2000/esi-terraform-worker/manager"
	"github.com/ttsubo2000/esi-terraform-worker/rest"
	cacheObj "github.com/ttsubo2000/esi-terraform-worker/tools/cache"
	"github.com/ttsubo2000/esi-terraform-worker/types"
)

func main() {
	clientState := cacheObj.NewStore(cacheObj.MetaNamespaceKeyFunc)

	go func() {
		rest.HandleRequests(clientState)
	}()

	mgr := manager.NewManager()
	mgr.Add(controllers.NewController("provider", &controllers.ProviderReconciler{Client: clientState}, &types.Provider{}, clientState))
	mgr.Add(controllers.NewController("configuration", &controllers.ConfigurationReconciler{Client: clientState}, &types.Configuration{}, clientState))
	if err := mgr.Start(manager.SetupSignalHandler()); err != nil {
		klog.Error(err, "problem controller")
		os.Exit(1)
	}
}
