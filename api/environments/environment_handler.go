package environments

import (
	"fmt"
	"strings"

	"github.com/equinor/radix-api/api/deployments"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	"github.com/equinor/radix-api/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	k8sObjectUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

const latestDeployment = true

// EnvironmentHandler Instance variables
type EnvironmentHandler struct {
	client          kubernetes.Interface
	radixclient     radixclient.Interface
	inClusterClient kubernetes.Interface
	deployHandler   deployments.DeployHandler
}

// Init Constructor
func Init(accounts models.Accounts) EnvironmentHandler {
	deployHandler := deployments.Init(accounts)
	return EnvironmentHandler{
		client:          accounts.UserAccount.Client,
		radixclient:     accounts.UserAccount.RadixClient,
		inClusterClient: accounts.ServiceAccount.Client,
		deployHandler:   deployHandler,
	}
}

// GetEnvironmentSummary handles api calls and returns a slice of EnvironmentSummary data for each environment
func (eh EnvironmentHandler) GetEnvironmentSummary(appName string) ([]*environmentModels.EnvironmentSummary, error) {
	radixApplication, err := eh.radixclient.RadixV1().RadixApplications(k8sObjectUtils.GetAppNamespace(appName)).Get(appName, metav1.GetOptions{})
	if err != nil {
		// This is no error, as the application may only have been just registered
		return []*environmentModels.EnvironmentSummary{}, nil
	}

	environments := make([]*environmentModels.EnvironmentSummary, len(radixApplication.Spec.Environments))
	for i, environment := range radixApplication.Spec.Environments {
		environments[i], err = eh.getEnvironmentSummary(radixApplication, environment)
		if err != nil {
			return nil, err
		}
	}

	orphanedEnvironments, err := eh.getOrphanedEnvironments(appName, radixApplication)
	environments = append(environments, orphanedEnvironments...)

	return environments, nil
}

// GetEnvironment Handler for GetEnvironment
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

	deployments, err := eh.deployHandler.GetDeploymentsForApplicationEnvironment(appName, envName, false)

	if err != nil {
		return nil, err
	}

	// data-transfer-object for serialization
	environmentDto := &environmentModels.Environment{
		Name:          envName,
		BranchMapping: buildFrom,
		Status:        configurationStatus.String(),
		Deployments:   deployments,
	}

	if len(deployments) > 0 {
		deployment, err := eh.deployHandler.GetDeploymentWithName(appName, deployments[0].Name)
		if err != nil {
			return nil, err
		}

		environmentDto.ActiveDeployment = deployment

		secrets, err := eh.GetEnvironmentSecretsForDeployment(appName, envName, deployment)
		if err != nil {
			return nil, err
		}

		environmentDto.Secrets = secrets
	}

	return environmentDto, nil
}

// DeleteEnvironment Handler for DeleteEnvironment. Deletes an environment if it is considered orphaned
func (eh EnvironmentHandler) DeleteEnvironment(appName, envName string) error {

	radixApplication, err := eh.radixclient.RadixV1().RadixApplications(k8sObjectUtils.GetAppNamespace(appName)).Get(appName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	uniqueName := k8sObjectUtils.GetEnvironmentNamespace(appName, envName)

	existInConfig, _ := envInConfig(uniqueName, radixApplication)

	if existInConfig {
		// Must be removed from radix config first
		return environmentModels.CannotDeleteNonOrphanedEnvironment(appName, uniqueName)
	}

	// idempotent removal of RadixEnvironment
	err = eh.radixclient.RadixV1().RadixEnvironments().Delete(uniqueName, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (eh EnvironmentHandler) getConfigurationStatusOfNamespace(namespace string) environmentModels.ConfigurationStatus {
	_, err := eh.client.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	if err != nil {
		return environmentModels.Pending
	}

	return environmentModels.Consistent
}

func (eh EnvironmentHandler) getConfigurationStatus(uniqueName string, radixApplication *v1.RadixApplication) (environmentModels.ConfigurationStatus, error) {

	namespacesInConfig := getNamespacesInConfig(radixApplication)

	exists, err := eh.namespaceExists(uniqueName)

	if namespacesInConfig[uniqueName] && !exists {
		// Environment is in config, but no namespace exist
		return environmentModels.Pending, nil

	} else if !exists {
		return 0, environmentModels.NonExistingEnvironment(err, radixApplication.Name, uniqueName)

	} else if isOrphaned(uniqueName, namespacesInConfig) {
		return environmentModels.Orphan, nil

	}

	return environmentModels.Consistent, nil
}

func (eh EnvironmentHandler) getEnvironmentSummary(app *v1.RadixApplication, env v1.Environment) (*environmentModels.EnvironmentSummary, error) {

	environmentSummary := &environmentModels.EnvironmentSummary{
		Name:          env.Name,
		BranchMapping: env.Build.From,
	}

	deploymentSummaries, err := eh.deployHandler.GetDeploymentsForApplicationEnvironment(app.Name, env.Name, latestDeployment)
	if err != nil {
		return nil, err
	}

	configurationStatus := eh.getConfigurationStatusOfNamespace(k8sObjectUtils.GetEnvironmentNamespace(app.Name, env.Name))
	environmentSummary.Status = configurationStatus.String()

	if len(deploymentSummaries) == 1 {
		environmentSummary.ActiveDeployment = deploymentSummaries[0]
	}

	return environmentSummary, nil
}

func (eh EnvironmentHandler) getOrphanEnvironmentSummary(appName string, envName string) (*environmentModels.EnvironmentSummary, error) {

	deploymentSummaries, err := eh.deployHandler.GetDeploymentsForApplicationEnvironment(appName, envName, latestDeployment)
	if err != nil {
		return nil, err
	}

	environmentSummary := &environmentModels.EnvironmentSummary{
		Name:   envName,
		Status: environmentModels.Orphan.String(),
	}

	if len(deploymentSummaries) == 1 {
		environmentSummary.ActiveDeployment = deploymentSummaries[0]
	}

	return environmentSummary, nil
}

// getOrphanedEnvironments returns a slice of Summary data of orphaned Namespaces or RadixEnvironments
func (eh EnvironmentHandler) getOrphanedEnvironments(appName string, radixApplication *v1.RadixApplication) ([]*environmentModels.EnvironmentSummary, error) {

	orphanedEnvironments := make([]*environmentModels.EnvironmentSummary, 0)

	for _, name := range eh.getOrphanedEnvNames(radixApplication) {
		summary, err := eh.getOrphanEnvironmentSummary(appName, name)
		if err != nil {
			return nil, err
		}

		orphanedEnvironments = append(orphanedEnvironments, summary)
	}

	return orphanedEnvironments, nil
}

// getOrphanedEnvNames returns a slice of non-unique-names of orphaned Namespaces or RadixEnvironments
func (eh EnvironmentHandler) getOrphanedEnvNames(app *v1.RadixApplication) []string {

	envNames := make([]string, 0)
	appLabel := fmt.Sprintf("%s=%s", kube.RadixAppLabel, app.Name)
	namespacesInConfig := getNamespacesInConfig(app)

	radixEnvironments, _ := eh.radixclient.RadixV1().RadixEnvironments().List(metav1.ListOptions{
		LabelSelector: appLabel,
	})

	for _, re := range radixEnvironments.Items {
		if isOrphaned(re.Name, namespacesInConfig) {
			envNames = append(envNames, re.Spec.EnvName)
		}
	}

	// TODO: make sure this second part is even necessary!
	namespaces, _ := eh.client.CoreV1().Namespaces().List(metav1.ListOptions{
		LabelSelector: appLabel,
	})

	for _, ns := range namespaces.Items {
		if !isAppNamespace(ns) &&
			isOrphaned(ns.Name, namespacesInConfig) {

			envName := ns.Labels[kube.RadixEnvLabel]

			covered := false
			for _, name := range envNames {
				if envName == name {
					covered = true
				}
			}
			if !covered {
				envNames = append(envNames, ns.Labels[kube.RadixEnvLabel])
			}
		}
	}

	return envNames
}

func (eh EnvironmentHandler) namespaceExists(uniqueName string) (bool, error) {
	_, err := eh.client.CoreV1().Namespaces().Get(uniqueName, metav1.GetOptions{})
	return err == nil, err
}

func getNamespacesInConfig(radixApplication *v1.RadixApplication) map[string]bool {
	namespacesInConfig := make(map[string]bool)
	for _, environment := range radixApplication.Spec.Environments {
		uniqueName := k8sObjectUtils.GetEnvironmentNamespace(radixApplication.Name, environment.Name)
		namespacesInConfig[uniqueName] = true
	}

	return namespacesInConfig
}

func isAppNamespace(namespace corev1.Namespace) bool {
	environment := namespace.Labels[kube.RadixEnvLabel]
	if !strings.EqualFold(environment, "app") {
		return false
	}

	return true
}

func isOrphaned(uniqueNamespaceName string, namespacesInConfig map[string]bool) bool {
	return !namespacesInConfig[uniqueNamespaceName]
}

func envInConfig(uniqueName string, radixApplication *v1.RadixApplication) (bool, map[string]bool) {
	environments := getNamespacesInConfig(radixApplication)
	return environments[uniqueName], environments
}
