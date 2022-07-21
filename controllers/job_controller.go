package controllers

import (
	"context"
	"os"

	"github.com/pkg/errors"
	"github.com/ttsubo/client-go/tools/cache"
	cacheObj "github.com/ttsubo2000/esi-terraform-worker/tools/cache"
	"github.com/ttsubo2000/esi-terraform-worker/types"
	"k8s.io/klog/v2"
)

type JobReconciler struct {
	Client cacheObj.Store
}

func (r *JobReconciler) Reconcile(ctx context.Context, req Request, indexer cache.Indexer) (Result, error) {
	klog.InfoS("reconciling Terraform job controller...", "NamespacedName", req.NamespacedName)

	key := "ConfigMap/default/tf-Configuration"
	obj, exists, err := r.Client.GetByKey(key)
	if err != nil || !exists {
		return Result{}, errors.Wrap(err, "failed to fetch TF configuration ConfigMap")
	}
	gotCM := obj.(*types.ConfigMap)
	for k, v := range gotCM.Data {
		klog.Infof("### TF configuration: [%s]=[%v]\n", k, v)
		f, _ := os.Create(k)
		f.Write([]byte(v))
		f.Close()
	}

	return Result{}, nil
}
