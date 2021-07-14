package environments

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/equinor/radix-api/api/deployments"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	"github.com/equinor/radix-api/api/events"
	eventModels "github.com/equinor/radix-api/api/events/models"
	"github.com/equinor/radix-api/api/pods"
	"github.com/equinor/radix-api/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	k8sObjectUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const latestDeployment = true

// EnvironmentHandlerOptions defines a configuration function
type EnvironmentHandlerOptions func(*EnvironmentHandler)

// WithAccounts configures all EnvironmentHandler fields
func WithAccounts(accounts models.Accounts) EnvironmentHandlerOptions {
	return func(eh *EnvironmentHandler) {
		eh.client = accounts.UserAccount.Client
		eh.radixclient = accounts.UserAccount.RadixClient
		eh.inClusterClient = accounts.ServiceAccount.Client
		eh.deployHandler = deployments.Init(accounts)
		eh.eventHandler = events.Init(accounts.UserAccount.Client)
		eh.accounts = accounts
	}
}

// WithEventHandler configures the eventHandler used by EnvironmentHandler
func WithEventHandler(eventHandler events.EventHandler) EnvironmentHandlerOptions {
	return func(eh *EnvironmentHandler) {
		eh.eventHandler = eventHandler
	}
}

// EnvironmentHandler Instance variables
type EnvironmentHandler struct {
	client          kubernetes.Interface
	radixclient     radixclient.Interface
	inClusterClient kubernetes.Interface
	deployHandler   deployments.DeployHandler
	eventHandler    events.EventHandler
	accounts        models.Accounts
}

// Init Constructor.
// Use the WithAccounts configuration function to configure a 'ready to use' EnvironmentHandler.
// EnvironmentHandlerOptions are processed in the seqeunce they are passed to this function.
func Init(opts ...EnvironmentHandlerOptions) EnvironmentHandler {
	eh := EnvironmentHandler{}

	for _, opt := range opts {
		opt(&eh)
	}

	return eh
}

// GetEnvironmentSummary handles api calls and returns a slice of EnvironmentSummary data for each environment
func (eh EnvironmentHandler) GetEnvironmentSummary(appName string) ([]*environmentModels.EnvironmentSummary, error) {
	radixApplication, err := eh.radixclient.RadixV1().RadixApplications(k8sObjectUtils.GetAppNamespace(appName)).Get(context.TODO(), appName, metav1.GetOptions{})
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
	radixApplication, err := eh.radixclient.RadixV1().RadixApplications(k8sObjectUtils.GetAppNamespace(appName)).Get(context.TODO(), appName, metav1.GetOptions{})
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

// CreateEnvironment Handler for CreateEnvironment. Creates an environment if it does not exist
func (eh EnvironmentHandler) CreateEnvironment(appName, envName string) (*v1.RadixEnvironment, error) {

	// ensure application exists
	rr, err := eh.radixclient.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// idempotent creation of RadixEnvironment
	re, err := eh.radixclient.RadixV1().RadixEnvironments().Create(context.TODO(), builders.
		NewEnvironmentBuilder().
		WithAppLabel().
		WithAppName(appName).
		WithEnvironmentName(envName).
		WithRegistrationOwner(rr).
		BuildRE(),
		metav1.CreateOptions{})
	// if an error is anything other than already-exist, return it
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, err
	}

	return re, nil
}

// DeleteEnvironment Handler for DeleteEnvironment. Deletes an environment if it is considered orphaned
func (eh EnvironmentHandler) DeleteEnvironment(appName, envName string) error {

	uniqueName := k8sObjectUtils.GetEnvironmentNamespace(appName, envName)
	re, err := eh.radixclient.RadixV1().RadixEnvironments().Get(context.TODO(), uniqueName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if !re.Status.Orphaned {
		// Must be removed from radix config first
		return environmentModels.CannotDeleteNonOrphanedEnvironment(appName, envName)
	}

	// idempotent removal of RadixEnvironment
	err = eh.getServiceAccount().RadixClient.RadixV1().RadixEnvironments().Delete(context.TODO(), uniqueName, metav1.DeleteOptions{})
	// if an error is anything other than not-found, return it
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

// GetEnvironmentEvents Handler for GetEnvironmentEvents
func (eh EnvironmentHandler) GetEnvironmentEvents(appName, envName string) ([]*eventModels.Event, error) {
	radixApplication, err := eh.radixclient.RadixV1().RadixApplications(k8sObjectUtils.GetAppNamespace(appName)).Get(context.TODO(), appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	_, err = eh.getConfigurationStatus(envName, radixApplication)
	if err != nil {
		return nil, err
	}

	events, err := eh.eventHandler.GetEvents(events.RadixEnvironmentNamespace(radixApplication, envName))
	if err != nil {
		return nil, err
	}

	return events, nil
}

func (eh EnvironmentHandler) getConfigurationStatus(envName string, radixApplication *v1.RadixApplication) (environmentModels.ConfigurationStatus, error) {

	uniqueName := k8sObjectUtils.GetEnvironmentNamespace(radixApplication.Name, envName)

	re, err := eh.radixclient.RadixV1().RadixEnvironments().Get(context.TODO(), uniqueName, metav1.GetOptions{})
	exists := err == nil

	if !exists {
		// does not exist in radix regardless of config
		return 0, environmentModels.NonExistingEnvironment(err, radixApplication.Name, envName)
	}

	if re.Status.Orphaned {
		// does not occur in config but is still an active resource
		return environmentModels.Orphan, nil
	}

	_, err = eh.client.CoreV1().Namespaces().Get(context.TODO(), uniqueName, metav1.GetOptions{})
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

	radixEnvironments, _ := eh.getServiceAccount().RadixClient.RadixV1().RadixEnvironments().List(context.TODO(), metav1.ListOptions{
		LabelSelector: appLabel,
	})

	for _, re := range radixEnvironments.Items {
		if re.Status.Orphaned {
			envNames = append(envNames, re.Spec.EnvName)
		}
	}

	return envNames
}

func (eh EnvironmentHandler) getServiceAccount() models.Account {
	return eh.accounts.ServiceAccount
}

func isAppNamespace(namespace corev1.Namespace) bool {
	environment := namespace.Labels[kube.RadixEnvLabel]
	if !strings.EqualFold(environment, "app") {
		return false
	}

	return true
}

// GetLogs handler for GetLogs
func (eh EnvironmentHandler) GetLogs(appName, envName, podName string, sinceTime *time.Time) (string, error) {
	podHandler := pods.Init(eh.client)
	log, err := podHandler.HandleGetEnvironmentPodLog(appName, envName, podName, "", sinceTime)
	if errors.IsNotFound(err) {
		return "", err
	}

	return log, nil
}

// GetScheduledJobLogs handler for GetScheduledJobLogs
func (eh EnvironmentHandler) GetScheduledJobLogs(appName, envName, scheduledJobName string, sinceTime *time.Time) (string, error) {
	handler := pods.Init(eh.client)
	log, err := handler.HandleGetEnvironmentScheduledJobLog(appName, envName, scheduledJobName, "", sinceTime)
	if err != nil {
		return "", err
	}

	return log, nil
}

//GetComponentEnvVars Get environment variables with metadata for the component
func (eh EnvironmentHandler) GetComponentEnvVars(appName string, envName string, componentName string) ([]environmentModels.EnvVar, error) {
	namespace := crdUtils.GetEnvironmentNamespace(appName, envName)
	rd, err := eh.getActiveDeployment(namespace)
	if err != nil {
		return nil, err
	}
	component := getComponent(rd, componentName)
	if component == nil {
		return nil, fmt.Errorf("component not found by name")
	}
	envVarsConfigMap, err := eh.getConfigMap(err, namespace, kube.GetEnvVarsConfigMapName(componentName))
	if err != nil {
		return nil, err
	}
	envVarsMetadataConfigMap, err := eh.getConfigMap(err, namespace, kube.GetEnvVarsMetadataConfigMapName(componentName))
	if err != nil {
		return nil, err
	}
	envVarsMetadataMap, err := kube.GetEnvVarsMetadataFromConfigMap(envVarsMetadataConfigMap)
	if err != nil {
		return nil, err
	}

	var apiEnvVars []environmentModels.EnvVar
	envVarsMap := component.GetEnvironmentVariables()
	for envVarName, envVar := range envVarsMap {
		apiEnvVar := environmentModels.EnvVar{Name: envVarName, Value: envVar}
		if cmEnvVar, foundCmEnvVar := envVarsConfigMap.Data[envVarName]; foundCmEnvVar {
			apiEnvVar.Value = cmEnvVar
			if envVarMetadata, foundMetadata := envVarsMetadataMap[envVarName]; foundMetadata {
				apiEnvVar.Metadata = environmentModels.EnvVarMetadata{RadixConfigValue: envVarMetadata.RadixConfigValue}
			}
		}
		apiEnvVars = append(apiEnvVars, apiEnvVar)
	}
	return apiEnvVars, nil
}

func (eh EnvironmentHandler) getConfigMap(err error, namespace string, envVarsConfigMapName string) (*corev1.ConfigMap, error) {
	return eh.client.CoreV1().ConfigMaps(namespace).Get(context.TODO(), envVarsConfigMapName, metav1.GetOptions{})
}

func getComponent(rd *v1.RadixDeployment, componentName string) v1.RadixCommonDeployComponent {
	for _, component := range rd.Spec.Components {
		if strings.EqualFold(component.Name, componentName) {
			return &component
		}
	}
	for _, jobComponent := range rd.Spec.Jobs {
		if strings.EqualFold(jobComponent.Name, componentName) {
			return &jobComponent
		}
	}
	return nil
}

func (eh EnvironmentHandler) getActiveDeployment(namespace string) (*v1.RadixDeployment, error) {
	radixDeployments, err := eh.radixclient.RadixV1().RadixDeployments(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, rd := range radixDeployments.Items {
		if rd.Status.ActiveTo.IsZero() {
			return &rd, err
		}
	}
	return nil, nil
}
