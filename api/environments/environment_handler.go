package environments

import (
	"k8s.io/client-go/kubernetes"

	"github.com/statoil/radix-api/api/deployments"
	environmentModels "github.com/statoil/radix-api/api/environments/models"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/statoil/radix-operator/pkg/apis/utils"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnvironmentHandler Instance variables
type EnvironmentHandler struct {
	client      kubernetes.Interface
	radixclient radixclient.Interface
}

// Init Constructor
func Init(client kubernetes.Interface, radixclient radixclient.Interface) EnvironmentHandler {
	return EnvironmentHandler{client, radixclient}
}

// HandleGetEnvironmentSummary Handler for GetEnvironmentSummary
func (eh EnvironmentHandler) HandleGetEnvironmentSummary(appName string) ([]*environmentModels.EnvironmentSummary, error) {
	radixApplication, err := eh.radixclient.RadixV1().RadixApplications(crdUtils.GetAppNamespace(appName)).Get(appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	latestDeployment := true
	deployHandler := deployments.Init(eh.client, eh.radixclient)

	environments := make([]*environmentModels.EnvironmentSummary, len(radixApplication.Spec.Environments))
	for i, environment := range radixApplication.Spec.Environments {
		environmentSummary := &environmentModels.EnvironmentSummary{
			Name:          environment.Name,
			BranchMapping: environment.Build.From,
		}

		configurationStatus := eh.getConfigurationStatus(crdUtils.GetEnvironmentNamespace(appName, environment.Name), radixApplication)
		if err != nil {
			return nil, err
		}

		deploymentSummaries, err := deployHandler.HandleGetDeployments(appName, environment.Name, latestDeployment)
		if err != nil {
			return nil, err
		}

		environmentSummary.Status = configurationStatus

		if len(deploymentSummaries) == 1 {
			environmentSummary.ActiveDeployment = deploymentSummaries[0]
		}

		environments[i] = environmentSummary
	}

	return environments, nil
}

func (eh EnvironmentHandler) getConfigurationStatus(namespace string, radixApplication *v1.RadixApplication) environmentModels.ConfigurationStatus {
	_, err := eh.client.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	if err != nil {
		return environmentModels.Pending
	}

	return environmentModels.Consistent
}
