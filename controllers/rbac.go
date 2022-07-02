package controllers

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	cacheObj "github.com/ttsubo2000/esi-terraform-worker/tools/cache"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
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
		if kerrors.IsNotFound(err) {
			if err := Client.Add(&clusterRole); err != nil {
				return errors.Wrap(err, "failed to create ClusterRole for Terraform executor")
			}
		}
	}
	return nil
}

func createTerraformExecutorClusterRoleBinding(ctx context.Context, Client cacheObj.Store, namespace, clusterRoleName, serviceAccountName string) error {
	var crbName = fmt.Sprintf("%s-tf-executor-clusterrole-binding", namespace)
	var clusterRoleBinding = rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      crbName,
			Namespace: namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: namespace,
			},
		},
	}
	key := "ClusterRole" + "/" + namespace + "/" + crbName
	_, _, err := Client.GetByKey(key)
	if err != nil {
		if kerrors.IsNotFound(err) {
			if err := Client.Add(&clusterRoleBinding); err != nil {
				return errors.Wrap(err, "failed to create ClusterRoleBinding for Terraform executor")
			}
		}
	}
	return nil
}

func createTerraformExecutorServiceAccount(ctx context.Context, Client cacheObj.Store, namespace, serviceAccountName string) error {
	var serviceAccount = v1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}
	key := "ServiceAccount" + "/" + namespace + "/" + serviceAccountName
	_, _, err := Client.GetByKey(key)
	if err != nil {
		if kerrors.IsNotFound(err) {
			if err := Client.Add(&serviceAccount); err != nil {
				return errors.Wrap(err, "failed to create ServiceAccount for Terraform executor")
			}
		}
	}
	return nil
}
