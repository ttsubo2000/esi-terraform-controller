package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	"github.com/ttsubo/client-go/tools/cache"
	"github.com/ttsubo2000/esi-terraform-worker/types"
)

type ProviderReconciler struct {
}

func (r *ProviderReconciler) Reconcile(ctx context.Context, req Request, indexer cache.Indexer) (Result, error) {
	klog.Info("Starting Reconcile ...")
	obj, exists, err := indexer.GetByKey(req.NamespacedName)
	if err != nil {
		klog.Errorf("Fetching object with key %s from store failed with %v", req.NamespacedName, err)
		return Result{RequeueAfter: 3 * time.Second}, errors.Wrap(err, "failed to fetch object")
	}

	if !exists {
		// Below we will warm up our cache with a sample, so that we will see a delete for one sample
		fmt.Printf("Sample %s does not exist anymore\n", req.NamespacedName)
	} else {
		// Note that you also have to check the uid if you have a local controlled resource, which
		// is dependent on the actual instance, to detect that a sample was recreated with the same name
		fmt.Printf("Sync/Add/Update for Sample [%v]\n", obj.(*types.Provider))
	}
	return Result{}, nil
}
