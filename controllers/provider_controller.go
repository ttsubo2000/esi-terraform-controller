package controllers

import (
	"context"
	"fmt"

	"k8s.io/klog/v2"

	"github.com/pkg/errors"
	"github.com/ttsubo/client-go/tools/cache"
	providercred "github.com/ttsubo2000/esi-terraform-worker/controllers/provider"
	cacheObj "github.com/ttsubo2000/esi-terraform-worker/tools/cache"
	"github.com/ttsubo2000/esi-terraform-worker/types"
)

const (
	errGetCredentials = "failed to get credentials from the cloud provider"
	errSettingStatus  = "failed to set status"
)

type ProviderReconciler struct {
	Client cacheObj.Store
}

func (r *ProviderReconciler) Reconcile(ctx context.Context, req Request, indexer cache.Indexer) (Result, error) {
	klog.InfoS("reconciling Terraform Provider...", "NamespacedName", req.NamespacedName)

	obj, exists, err := indexer.GetByKey(req.NamespacedName)
	if err != nil || !exists {
		if !exists {
			err = nil
		}
		return Result{}, err
	}
	provider := obj.(*types.Provider)

	if _, err := providercred.GetProviderCredentials(ctx, r.Client, provider, provider.Spec.Region); err != nil {
		provider.Status.State = types.ProviderIsNotReady
		provider.Status.Message = fmt.Sprintf("%s: %s", errGetCredentials, err.Error())
		klog.ErrorS(err, errGetCredentials, "Provider", req.NamespacedName)
		if updateErr := r.Client.Update(provider, false); updateErr != nil {
			klog.ErrorS(updateErr, errSettingStatus, "Provider", req.NamespacedName)
			return Result{}, errors.Wrap(updateErr, errSettingStatus)
		}
		return Result{}, errors.Wrap(err, errGetCredentials)
	}

	provider.Status = types.ProviderStatus{
		State: types.ProviderIsReady,
	}
	if updateErr := r.Client.Update(provider, false); updateErr != nil {
		klog.ErrorS(updateErr, errSettingStatus, "Provider", req.NamespacedName)
		return Result{}, errors.Wrap(updateErr, errSettingStatus)
	}

	return Result{}, nil
}
