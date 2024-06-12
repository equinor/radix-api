package environments

import (
	"context"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	v2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (eh EnvironmentHandler) getRadixDeployment(ctx context.Context, appName, envName string) (*deploymentModels.DeploymentSummary, *v1.RadixDeployment, error) {
	envNs := operatorutils.GetEnvironmentNamespace(appName, envName)
	deploymentSummary, err := eh.deployHandler.GetLatestDeploymentForApplicationEnvironment(ctx, appName, envName)
	if err != nil {
		return nil, nil, err
	}

	radixDeployment, err := eh.radixclient.RadixV1().RadixDeployments(envNs).Get(ctx, deploymentSummary.Name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	return deploymentSummary, radixDeployment, nil
}

func (eh EnvironmentHandler) getRadixApplicationInAppNamespace(ctx context.Context, appName string) (*v1.RadixApplication, error) {
	return eh.radixclient.RadixV1().RadixApplications(operatorutils.GetAppNamespace(appName)).Get(ctx, appName, metav1.GetOptions{})
}

func (eh EnvironmentHandler) getRadixEnvironment(ctx context.Context, name string) (*v1.RadixEnvironment, error) {
	return eh.getServiceAccount().RadixClient.RadixV1().RadixEnvironments().Get(ctx, name, metav1.GetOptions{})
}

func (eh EnvironmentHandler) getHPAsInEnvironment(ctx context.Context, appName, envName string) ([]v2.HorizontalPodAutoscaler, error) {
	envNs := operatorutils.GetEnvironmentNamespace(appName, envName)
	hpas, err := eh.accounts.UserAccount.Client.AutoscalingV2().HorizontalPodAutoscalers(envNs).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return hpas.Items, nil
}

func (eh EnvironmentHandler) getScaledObjectsInEnvironment(ctx context.Context, appName, envName string) ([]v1alpha1.ScaledObject, error) {
	envNs := operatorutils.GetEnvironmentNamespace(appName, envName)
	scaledObjects, err := eh.accounts.UserAccount.KedaClient.KedaV1alpha1().ScaledObjects(envNs).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return scaledObjects.Items, nil
}
