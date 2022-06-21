package controllers

import (
	"context"

	"github.com/ttsubo/client-go/tools/cache"
	cacheObj "github.com/ttsubo2000/esi-terraform-worker/tools/cache"
)

type SecretReconciler struct {
	Client cacheObj.Store
}

func (r *SecretReconciler) Reconcile(ctx context.Context, req Request, indexer cache.Indexer) (Result, error) {
	return Result{}, nil
}
