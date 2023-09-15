package rolebindings

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	log "github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
)

// EnsureRoleBindingExists creates a role binding with a subject that binds AD groups to a specific role
func EnsureRoleBindingExists(ctx context.Context, kubeClient kubernetes.Interface, namespace, roleName, roleBindingName string, adGroups *[]string) error {
	var subjects []rbacv1.Subject
	for _, adGroup := range *adGroups {
		subjects = append(subjects, rbacv1.Subject{
			Kind:     k8s.KindGroup,
			Name:     adGroup,
			APIGroup: k8s.RbacApiGroup,
		})
	}
	newRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: roleBindingName},
		RoleRef: rbacv1.RoleRef{
			APIGroup: k8s.RbacApiGroup,
			Kind:     k8s.KindClusterRole,
			Name:     roleName,
		}, Subjects: subjects,
	}
	existingRoleBinding, err := kubeClient.RbacV1().RoleBindings(namespace).Get(ctx, roleName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			_, err = kubeClient.RbacV1().RoleBindings(namespace).Create(ctx, newRoleBinding, metav1.CreateOptions{})
		}
		return err
	}

	existingRoleBinding = existingRoleBinding.DeepCopy()
	existingRoleBindingJSON, err := json.Marshal(existingRoleBinding)
	if err != nil {
		return fmt.Errorf("failed to marshal old role binding object: %v", err)
	}

	newRoleBindingJSON, err := json.Marshal(newRoleBinding)
	if err != nil {
		return fmt.Errorf("failed to marshal new role binding object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(existingRoleBindingJSON, newRoleBindingJSON, rbacv1.Role{})
	if err != nil {
		return fmt.Errorf("failed to create two way merge patch role binding objects: %v", err)
	}

	if kube.IsEmptyPatch(patchBytes) {
		return nil
	}
	_, err = kubeClient.RbacV1().RoleBindings(namespace).Patch(ctx, roleBindingName, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch role binding object: %v", err)
	}
	log.Debugf("Patched role binding: %s in namespace %s", roleBindingName, namespace)
	return nil
}

// DeleteRoleBinding Deletes a role binding
func DeleteRoleBinding(ctx context.Context, kubeClient kubernetes.Interface, namespace, roleBindingName string) error {
	return kubeClient.RbacV1().RoleBindings(namespace).Delete(ctx, roleBindingName, metav1.DeleteOptions{})
}
