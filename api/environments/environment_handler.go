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
	"k8s.io/apimachinery/pkg/api/errors"
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
	accounts        models.Accounts
}

// Init Constructor
func Init(accounts models.Accounts) EnvironmentHandler {
	deployHandler := deployments.Init(accounts)
	return EnvironmentHandler{
		client:          accounts.UserAccount.Client,
		radixclient:     accounts.UserAccount.RadixClient,
		inClusterClient: accounts.ServiceAccount.Client,
		deployHandler:   deployHandler,
		accounts:        accounts,
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
	configMap := getEnvironmentsInConfig(radixApplication)

	if configMap[uniqueName] {
		// Must be removed from radix config first
		return environmentModels.CannotDeleteNonOrphanedEnvironment(appName, envName)
	}

	// idempotent removal of RadixEnvironment
	err = eh.getServiceAccount().RadixClient.RadixV1().RadixEnvironments().Delete(uniqueName, &metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

func (eh EnvironmentHandler) getConfigurationStatus(envName string, radixApplication *v1.RadixApplication) (environmentModels.ConfigurationStatus, error) {

	configMap := getEnvironmentsInConfig(radixApplication)

	uniqueName := k8sObjectUtils.GetEnvironmentNamespace(radixApplication.Name, envName)
	exists, err := eh.environmentExists(uniqueName)

	if !exists {
		// does not exist in radix regardless of config
		return 0, environmentModels.NonExistingEnvironment(err, radixApplication.Name, envName)
	}

	if !configMap[uniqueName] {
		// does not occur in config but is still an active resource
		return environmentModels.Orphan, nil
	}

	_, err = eh.client.CoreV1().Namespaces().Get(uniqueName, metav1.GetOptions{})
	if err != nil {
		// exists but does not have underlying resources
		return environmentModels.Pending, nil
	}

	// exists and has underlying resources
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

	configurationStatus, _ := eh.getConfigurationStatus(env.Name, app)
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

// getOrphanedEnvironments returns a slice of Summary data of orphaned environments
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

// getOrphanedEnvNames returns a slice of non-unique-names of orphaned environments
func (eh EnvironmentHandler) getOrphanedEnvNames(app *v1.RadixApplication) []string {

	envNames := make([]string, 0)
	appLabel := fmt.Sprintf("%s=%s", kube.RadixAppLabel, app.Name)
	environmentsInConfig := getEnvironmentsInConfig(app)
	radixEnvironments, _ := eh.getServiceAccount().RadixClient.RadixV1().RadixEnvironments().List(metav1.ListOptions{
		LabelSelector: appLabel,
	})

	for _, re := range radixEnvironments.Items {
		if !environmentsInConfig[re.Name] {
			envNames = append(envNames, re.Spec.EnvName)
		}
	}

	return envNames
}

func (eh EnvironmentHandler) environmentExists(uniqueName string) (bool, error) {
	_, err := eh.radixclient.RadixV1().RadixEnvironments().Get(uniqueName, metav1.GetOptions{})
	return err == nil, err
}

func (eh EnvironmentHandler) getServiceAccount() models.Account {
	return eh.accounts.ServiceAccount
}

func getEnvironmentsInConfig(radixApplication *v1.RadixApplication) map[string]bool {
	environmentsInConfig := make(map[string]bool)
	for _, environment := range radixApplication.Spec.Environments {
		uniqueName := k8sObjectUtils.GetEnvironmentNamespace(radixApplication.Name, environment.Name)
		environmentsInConfig[uniqueName] = true
	}

	return environmentsInConfig
}

func isAppNamespace(namespace corev1.Namespace) bool {
	environment := namespace.Labels[kube.RadixEnvLabel]
	if !strings.EqualFold(environment, "app") {
		return false
	}

	return true
}
