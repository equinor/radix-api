package kubequery

import (
	"context"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetDeploymentsForEnvironment returns all Deployments for the specified application and environment.
func GetDeploymentsForEnvironment(ctx context.Context, client kubernetes.Interface, appName, envName string) ([]appsv1.Deployment, error) {
	ns := operatorUtils.GetEnvironmentNamespace(appName, envName)
	deployments, err := client.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{LabelSelector: labels.ForApplicationName(appName).String()})
	if err != nil {
		return nil, err
	}
	return deployments.Items, nil
}

// GetRadixDeploymentByName returns a RadixDeployment for an application and namespace
func GetRadixDeploymentByName(ctx context.Context, radixClient radixclient.Interface, appName, envName, deploymentName string) (*radixv1.RadixDeployment, error) {
	ns := operatorUtils.GetEnvironmentNamespace(appName, envName)
	return radixClient.RadixV1().RadixDeployments(ns).Get(ctx, deploymentName, metav1.GetOptions{})
}
