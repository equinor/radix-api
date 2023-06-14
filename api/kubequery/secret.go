package kubequery

import (
	"context"

	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func GetSecretsForEnvironment(ctx context.Context, client kubernetes.Interface, appName, envName string) ([]corev1.Secret, error) {
	ns := operatorutils.GetEnvironmentNamespace(appName, envName)
	secrets, err := client.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return secrets.Items, nil
}
