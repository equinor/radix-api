package environments

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/statoil/radix-api/api/deployments"
	environmentModels "github.com/statoil/radix-api/api/environments/models"
	"github.com/statoil/radix-api/api/utils"
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
	client      kubernetes.Interface
	radixclient radixclient.Interface
}

// Init Constructor
func Init(client kubernetes.Interface, radixclient radixclient.Interface) EnvironmentHandler {
	return EnvironmentHandler{client, radixclient}
}

// HandleGetEnvironmentSummary Handler for GetEnvironmentSummary
func (eh EnvironmentHandler) HandleGetEnvironmentSummary(appName string) ([]*environmentModels.EnvironmentSummary, error) {
	radixApplication, err := eh.radixclient.RadixV1().RadixApplications(k8sObjectUtils.GetAppNamespace(appName)).Get(appName, metav1.GetOptions{})
	if err != nil {
		// This is no error, as the application may only have been just registered
		return []*environmentModels.EnvironmentSummary{}, nil
	}

	deployHandler := deployments.Init(eh.client, eh.radixclient)

	environments := make([]*environmentModels.EnvironmentSummary, len(radixApplication.Spec.Environments))
	for i, environment := range radixApplication.Spec.Environments {
		environmentSummary := &environmentModels.EnvironmentSummary{
			Name:          environment.Name,
			BranchMapping: environment.Build.From,
		}

		configurationStatus := eh.getConfigurationStatus(k8sObjectUtils.GetEnvironmentNamespace(appName, environment.Name), radixApplication)
		if err != nil {
			return nil, err
		}

		deploymentSummaries, err := deployHandler.HandleGetDeployments(appName, environment.Name, latestDeployment)
		if err != nil {
			return nil, err
		}

		environmentSummary.Status = configurationStatus.String()

		if len(deploymentSummaries) == 1 {
			environmentSummary.ActiveDeployment = deploymentSummaries[0]
		}

		environments[i] = environmentSummary
	}

	orphanedEnvironments, err := eh.getOrphanedEnvironments(appName, radixApplication, deployHandler)
	environments = append(environments, orphanedEnvironments...)

	return environments, nil
}

// HandleChangeEnvironmentComponentSecret handler for HandleChangeEnvironmentComponentSecret
func (eh EnvironmentHandler) HandleChangeEnvironmentComponentSecret(appName, envName, componentName, secretName string, componentSecret environmentModels.ComponentSecret) (*environmentModels.ComponentSecret, error) {
	newSecretValue := componentSecret.SecretValue
	if strings.TrimSpace(newSecretValue) == "" {
		return nil, utils.ValidationError("Secret", "New secret value is empty")
	}

	ns := k8sObjectUtils.GetEnvironmentNamespace(appName, envName)
	secretObject, err := eh.client.CoreV1().Secrets(ns).Get(componentName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		return nil, utils.TypeMissingError("Secret object does not exist", err)
	}
	if err != nil {
		return nil, utils.UnexpectedError("Failed getting secret object", err)
	}

	oldSecretValue, exists := secretObject.Data[secretName]
	if !exists {
		return nil, utils.ValidationError("Secret", "Secret name does not exist")
	}

	if string(oldSecretValue) == newSecretValue {
		return nil, utils.ValidationError("Secret", "No change in secret value")
	}

	secretObject.Data[secretName] = []byte(newSecretValue)

	updatedSecret, err := eh.client.CoreV1().Secrets(ns).Update(secretObject)
	if err != nil {
		return nil, err
	}

	componentSecret.SecretValue = string(updatedSecret.Data[secretName])

	return &componentSecret, nil
}

func (eh EnvironmentHandler) getConfigurationStatus(namespace string, radixApplication *v1.RadixApplication) environmentModels.ConfigurationStatus {
	_, err := eh.client.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	if err != nil {
		return environmentModels.Pending
	}

	return environmentModels.Consistent
}

func (eh EnvironmentHandler) getOrphanedEnvironments(appName string, radixApplication *v1.RadixApplication, deployHandler deployments.DeployHandler) ([]*environmentModels.EnvironmentSummary, error) {
	namespaces, err := eh.client.CoreV1().Namespaces().List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("radix-app=%s", appName),
	})
	if err != nil {
		return nil, err
	}

	namespacesInConfig := make(map[string]bool)
	for _, environment := range radixApplication.Spec.Environments {
		environmentNamespace := k8sObjectUtils.GetEnvironmentNamespace(appName, environment.Name)
		namespacesInConfig[environmentNamespace] = true
	}

	orphanedEnvironments := make([]*environmentModels.EnvironmentSummary, 0)
	for _, namespace := range namespaces.Items {
		if !isAppNamespace(namespace) &&
			isOrphaned(namespace.Name, namespacesInConfig) {

			// Orphaned namespace
			environment := namespace.Labels["radix-env"]
			deploymentSummaries, err := deployHandler.HandleGetDeployments(appName, environment, latestDeployment)
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
