package roles

import (
	"context"

	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// EnsureRoleCreateConfigMapExists creates a role with a rule that allows creating a configmap with the given name
func EnsureRoleCreateConfigMapExists(ctx context.Context, kubeClient kubernetes.Interface, namespace, roleName, configMapName string) error {
	_, err := kubeClient.RbacV1().Roles(namespace).Get(ctx, roleName, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !k8serrors.IsNotFound(err) {
		return err
	}
	_, err = kubeClient.RbacV1().Roles(namespace).Create(ctx, &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: roleName},
		Rules: []rbacv1.PolicyRule{{
			Verbs:         []string{"create"},
			APIGroups:     []string{""},
			Resources:     []string{"configmaps"},
			ResourceNames: []string{configMapName},
		}},
	}, metav1.CreateOptions{})
	return err
}

// DeleteRole Deletes a role
func DeleteRole(ctx context.Context, kubeClient kubernetes.Interface, namespace, roleName string) error {
	return kubeClient.RbacV1().Roles(namespace).Delete(ctx, roleName, metav1.DeleteOptions{})
}
