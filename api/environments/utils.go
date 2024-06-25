package environments

import (
	"context"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (eh EnvironmentHandler) getRadixDeployment(ctx context.Context, appName, envName string) (*deploymentModels.DeploymentSummary, *v1.RadixDeployment, error) {
	envNs := operatorutils.GetEnvironmentNamespace(appName, envName)
	deploymentSummary, err := eh.deployHandler.GetLatestDeploymentForApplicationEnvironment(ctx, appName, envName)
	if err != nil {
		return nil, nil, err
	}

	radixDeployment, err := eh.accounts.UserAccount.RadixClient.RadixV1().RadixDeployments(envNs).Get(ctx, deploymentSummary.Name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	return deploymentSummary, radixDeployment, nil
}
