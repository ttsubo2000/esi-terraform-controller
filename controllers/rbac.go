package controllers

import (
	"context"

	"github.com/pkg/errors"
	cacheObj "github.com/ttsubo2000/terraform-controller/tools/cache"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createTerraformExecutorClusterRole(ctx context.Context, Client cacheObj.Store, clusterRoleName string) error {
	var clusterRole = rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterRoleName,
			Namespace: "Default",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "list", "create", "update", "delete"},
			},
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"get", "create", "update", "delete"},
			},
		},
	}
	key := "ClusterRole" + "/" + "Default" + "/" + clusterRoleName
	_, exists, err := Client.GetByKey(key)
	if err != nil || !exists {
		if !exists {
			if err := Client.Add(&clusterRole); err != nil {
				return errors.Wrap(err, "failed to create ClusterRole for Terraform executor")
			}
		}
	}
	return nil
}
