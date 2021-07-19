package environments

import (
	"context"
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdutils "github.com/equinor/radix-operator/pkg/apis/utils"
	k8sObjectUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (eh EnvironmentHandler) getRadixDeployment(appName string, envName string) (*deploymentModels.DeploymentSummary, *v1.RadixDeployment, error) {
	envNs := crdutils.GetEnvironmentNamespace(appName, envName)
	deploymentSummary, err := eh.deployHandler.GetLatestDeploymentForApplicationEnvironment(appName, envName)
	if err != nil {
		return nil, nil, err
	}

	radixDeployment, err := eh.radixclient.RadixV1().RadixDeployments(envNs).Get(context.TODO(), deploymentSummary.Name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	return deploymentSummary, radixDeployment, nil
}

func (eh EnvironmentHandler) getRadixApplicationInAppNamespace(appName string) (*v1.RadixApplication, error) {
	return eh.radixclient.RadixV1().RadixApplications(k8sObjectUtils.GetAppNamespace(appName)).Get(context.TODO(), appName, metav1.GetOptions{})
}

func (eh EnvironmentHandler) getRadixEnvironments(name string) (*v1.RadixEnvironment, error) {
	return eh.radixclient.RadixV1().RadixEnvironments().Get(context.TODO(), name, metav1.GetOptions{})
}
