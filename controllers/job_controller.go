package controllers

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/terraform-exec/tfexec"
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
		filename := k
		content := v
		klog.Infof("### TF configuration: [%s]=[%v]\n", filename, content)
		f, _ := os.Create(filename)
		f.Write([]byte(content))
		f.Close()
		err := os.Rename(filename, fmt.Sprintf("work/%s", filename))
		if err != nil {
			klog.Fatal(err)
		}
	}

	installer := &releases.ExactVersion{
		Product: product.Terraform,
		Version: version.Must(version.NewVersion("1.0.6")),
	}

	execPath, err := installer.Install(context.Background())
	if err != nil {
		klog.Fatalf("error installing Terraform: %s", err)
	}

	workingDir := "./work"
	tf, err := tfexec.NewTerraform(workingDir, execPath)
	if err != nil {
		klog.Fatalf("error running NewTerraform: %s", err)
	}

	err = tf.Init(context.Background(), tfexec.Upgrade(true))
	if err != nil {
		klog.Fatalf("error running Init: %s", err)
	}

	state, err := tf.Show(context.Background())
	if err != nil {
		klog.Fatalf("error running Show: %s", err)
	}

	klog.Infof("state.FormatVersion:[%s]", state.FormatVersion)

	return Result{}, nil
}
