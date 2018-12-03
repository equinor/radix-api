package environments

import (
	"fmt"
	"strings"

	"github.com/statoil/radix-api/api/deployments"
	environmentModels "github.com/statoil/radix-api/api/environments/models"
	k8sObjectUtils "github.com/statoil/radix-operator/pkg/apis/utils"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
)

const latestDeployment = true

// EnvironmentHandler Instance variables
type EnvironmentHandler struct {
	client        kubernetes.Interface
	radixclient   radixclient.Interface
	deployHandler deployments.DeployHandler
}

// Init Constructor
func Init(client kubernetes.Interface, radixclient radixclient.Interface) EnvironmentHandler {
	deployHandler := deployments.Init(client, radixclient)
	return EnvironmentHandler{client, radixclient, deployHandler}
}

// GetEnvironmentSummary GetEnvironmentSummary
func (eh EnvironmentHandler) GetEnvironmentSummary(appName string) ([]*environmentModels.EnvironmentSummary, error) {
	radixApplication, err := eh.radixclient.RadixV1().RadixApplications(k8sObjectUtils.GetAppNamespace(appName)).Get(appName, metav1.GetOptions{})
	if err != nil {
		// This is no error, as the application may only have been just registered
		return []*environmentModels.EnvironmentSummary{}, nil
	}

	environments := make([]*environmentModels.EnvironmentSummary, len(radixApplication.Spec.Environments))
	for i, environment := range radixApplication.Spec.Environments {
		environmentSummary := &environmentModels.EnvironmentSummary{
			Name:          environment.Name,
			BranchMapping: environment.Build.From,
		}

		deploymentSummaries, err := eh.deployHandler.GetDeployments(appName, environment.Name, latestDeployment)
		if err != nil {
			return nil, err
		}

		configurationStatus := eh.getConfigurationStatusOfNamespace(k8sObjectUtils.GetEnvironmentNamespace(appName, environment.Name))
		environmentSummary.Status = configurationStatus.String()

		if len(deploymentSummaries) == 1 {
			environmentSummary.ActiveDeployment = deploymentSummaries[0]
		}

		environments[i] = environmentSummary
	}

	orphanedEnvironments, err := eh.getOrphanedEnvironments(appName, radixApplication)
	environments = append(environments, orphanedEnvironments...)

	return environments, nil
}

// GetEnvironment Handler for GetEnvironmentSummary
func (eh EnvironmentHandler) GetEnvironment(appName, envName string) (*environmentModels.Environment, error) {
	radixApplication, err := eh.radixclient.RadixV1().RadixApplications(k8sObjectUtils.GetAppNamespace(appName)).Get(appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	configurationStatus, err := eh.getConfigurationStatus(envName, radixApplication)
	if err != nil {
		return nil, err
	}

	buildFrom := ""

	if configurationStatus != environmentModels.Orphan {
		// Find the environment
		var theEnvironment *v1.Environment
		for _, environment := range radixApplication.Spec.Environments {
			if strings.EqualFold(environment.Name, envName) {
				theEnvironment = &environment
				break
			}
		}

		buildFrom = theEnvironment.Build.From
	}

	deployments, err := eh.deployHandler.GetDeployments(appName, envName, false)

	if err != nil {
		return nil, err
	}

	secrets, err := eh.GetEnvironmentSecrets(appName, envName)
	if err != nil {
		return nil, err
	}

	environment := &environmentModels.Environment{
		Name:          envName,
		BranchMapping: buildFrom,
		Status:        configurationStatus.String(),
		Deployments:   deployments,
		Secrets:       secrets,
	}

	if len(deployments) > 0 {
		deployment, err := eh.deployHandler.GetDeploymentWithName(appName, deployments[0].Name)
		if err != nil {
			return nil, err
		}

		environment.ActiveDeployment = deployment
	}

	return environment, nil
}

func (eh EnvironmentHandler) getConfigurationStatusOfNamespace(namespace string) environmentModels.ConfigurationStatus {
	_, err := eh.client.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	if err != nil {
		return environmentModels.Pending
	}

	return environmentModels.Consistent
}

func (eh EnvironmentHandler) getConfigurationStatus(envName string, radixApplication *v1.RadixApplication) (environmentModels.ConfigurationStatus, error) {
	environmentNamespace := k8sObjectUtils.GetEnvironmentNamespace(radixApplication.Name, envName)
	namespacesInConfig := getNamespacesInConfig(radixApplication)

	_, err := eh.client.CoreV1().Namespaces().Get(environmentNamespace, metav1.GetOptions{})
	if namespacesInConfig[environmentNamespace] && err != nil {
		// Environment is in config, but no namespace exist
		return environmentModels.Pending, nil

	} else if err != nil {
		return 0, environmentModels.NonExistingEnvironment(err, radixApplication.Name, envName)

	} else if isOrphaned(environmentNamespace, namespacesInConfig) {
		return environmentModels.Orphan, nil

	}

	return environmentModels.Consistent, nil
}

func (eh EnvironmentHandler) getOrphanedEnvironments(appName string, radixApplication *v1.RadixApplication) ([]*environmentModels.EnvironmentSummary, error) {
	namespaces, err := eh.client.CoreV1().Namespaces().List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("radix-app=%s", appName),
	})
	if err != nil {
		return nil, err
	}

	namespacesInConfig := getNamespacesInConfig(radixApplication)

	orphanedEnvironments := make([]*environmentModels.EnvironmentSummary, 0)
	for _, namespace := range namespaces.Items {
		if !isAppNamespace(namespace) &&
			isOrphaned(namespace.Name, namespacesInConfig) {

			// Orphaned namespace
			environment := namespace.Labels["radix-env"]
			deploymentSummaries, err := eh.deployHandler.GetDeployments(appName, environment, latestDeployment)
			if err != nil {
				return nil, err
			}

			environmentSummary := &environmentModels.EnvironmentSummary{
				Name:   environment,
				Status: environmentModels.Orphan.String(),
			}

			if len(deploymentSummaries) == 1 {
				environmentSummary.ActiveDeployment = deploymentSummaries[0]
			}

			orphanedEnvironments = append(orphanedEnvironments, environmentSummary)
		}
	}

	return orphanedEnvironments, nil
}

func getNamespacesInConfig(radixApplication *v1.RadixApplication) map[string]bool {
	namespacesInConfig := make(map[string]bool)
	for _, environment := range radixApplication.Spec.Environments {
		environmentNamespace := k8sObjectUtils.GetEnvironmentNamespace(radixApplication.Name, environment.Name)
		namespacesInConfig[environmentNamespace] = true
	}

	return namespacesInConfig
}

func isAppNamespace(namespace corev1.Namespace) bool {
	environment := namespace.Labels["radix-env"]
	if !strings.EqualFold(environment, "app") {
		return false
	}

	return true
}

func isOrphaned(namespace string, namespacesInConfig map[string]bool) bool {
	if namespacesInConfig[namespace] {
		return false
	}

	return true
}
