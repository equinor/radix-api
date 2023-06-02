package environments

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/equinor/radix-api/api/deployments"
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	"github.com/equinor/radix-api/api/events"
	eventModels "github.com/equinor/radix-api/api/events/models"
	"github.com/equinor/radix-api/api/pods"
	"github.com/equinor/radix-api/api/secrets"
	"github.com/equinor/radix-api/api/utils/labelselector"
	"github.com/equinor/radix-api/models"
	radixutils "github.com/equinor/radix-common/utils"
	configUtils "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	deployUtils "github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	k8sObjectUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	"go.elastic.co/apm"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// EnvironmentHandlerOptions defines a configuration function
type EnvironmentHandlerOptions func(*EnvironmentHandler)

// WithAccounts configures all EnvironmentHandler fields
func WithAccounts(accounts models.Accounts) EnvironmentHandlerOptions {
	return func(eh *EnvironmentHandler) {
		eh.client = accounts.UserAccount.Client
		eh.radixclient = accounts.UserAccount.RadixClient
		eh.inClusterClient = accounts.ServiceAccount.Client
		eh.deployHandler = deployments.Init(accounts)
		eh.secretHandler = secrets.Init(secrets.WithAccounts(accounts))
		eh.eventHandler = events.Init(accounts.UserAccount.Client)
		eh.accounts = accounts
		kubeUtil, _ := kube.New(accounts.UserAccount.Client, accounts.UserAccount.RadixClient, accounts.UserAccount.SecretProviderClient)
		eh.kubeUtil = kubeUtil
		kubeUtilsForServiceAccount, _ := kube.New(accounts.ServiceAccount.Client, accounts.ServiceAccount.RadixClient, accounts.ServiceAccount.SecretProviderClient)
		eh.kubeUtilForServiceAccount = kubeUtilsForServiceAccount
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
	client                    kubernetes.Interface
	radixclient               radixclient.Interface
	inClusterClient           kubernetes.Interface
	deployHandler             deployments.DeployHandler
	secretHandler             secrets.SecretHandler
	eventHandler              events.EventHandler
	accounts                  models.Accounts
	kubeUtil                  *kube.Kube
	kubeUtilForServiceAccount *kube.Kube
}

var validaStatusesToScaleComponent []string

// Init Constructor.
// Use the WithAccounts configuration function to configure a 'ready to use' EnvironmentHandler.
// EnvironmentHandlerOptions are processed in the seqeunce they are passed to this function.
func Init(opts ...EnvironmentHandlerOptions) EnvironmentHandler {
	validaStatusesToScaleComponent = []string{deploymentModels.ConsistentComponent.String(), deploymentModels.StoppedComponent.String()}

	eh := EnvironmentHandler{}

	for _, opt := range opts {
		opt(&eh)
	}

	return eh
}

// GetEnvironmentSummary handles api calls and returns a slice of EnvironmentSummary data for each environment
func (eh EnvironmentHandler) GetEnvironmentSummary(ctx context.Context, appName string) ([]*environmentModels.EnvironmentSummary, error) {
	span, ctx := apm.StartSpan(ctx, fmt.Sprintf("GetEnvironmentSummary (appName=%s)", appName), "EnvironmentHandler")
	defer span.End()
	type ChannelData struct {
		position int
		summary  *environmentModels.EnvironmentSummary
	}

	radixApplication, err := eh.getRadixApplicationInAppNamespace(ctx, appName)
	if err != nil {
		// This is no error, as the application may only have been just registered
		return []*environmentModels.EnvironmentSummary{}, nil
	}

	var g errgroup.Group
	g.SetLimit(10)

	envSize := len(radixApplication.Spec.Environments)
	envChan := make(chan *ChannelData, envSize)
	for i, environment := range radixApplication.Spec.Environments {
		environment := environment
		i := i
		g.Go(func() error {
			summary, err := eh.getEnvironmentSummary(ctx, radixApplication, environment)
			if err == nil {
				envChan <- &ChannelData{position: i, summary: summary}
			}
			return err
		})
	}

	err = g.Wait()
	close(envChan)
	if err != nil {
		return nil, err
	}

	orphanedEnvironments, err := eh.getOrphanedEnvironments(ctx, appName, radixApplication)
	if err != nil {
		return nil, err
	}

	environments := make([]*environmentModels.EnvironmentSummary, envSize)
	for env := range envChan {
		environments[env.position] = env.summary
	}
	environments = append(environments, orphanedEnvironments...)

	return environments, nil
}

// GetEnvironment Handler for GetEnvironment
func (eh EnvironmentHandler) GetEnvironment(ctx context.Context, appName, envName string) (*environmentModels.Environment, error) {
	radixApplication, err := eh.getRadixApplicationInAppNamespace(ctx, appName)
	if err != nil {
		return nil, err
	}

	configurationStatus, err := eh.getConfigurationStatus(ctx, envName, radixApplication)
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

	deploymentSummaries, err := eh.deployHandler.GetDeploymentsForApplicationEnvironment(ctx, appName, envName, false)

	if err != nil {
		return nil, err
	}

	// data-transfer-object for serialization
	environmentDto := &environmentModels.Environment{
		Name:          envName,
		BranchMapping: buildFrom,
		Status:        configurationStatus.String(),
		Deployments:   deploymentSummaries,
	}

	if len(deploymentSummaries) > 0 {
		deployment, err := eh.deployHandler.GetDeploymentWithName(ctx, appName, deploymentSummaries[0].Name)
		if err != nil {
			return nil, err
		}

		environmentDto.ActiveDeployment = deployment

		deploymentSecrets, err := eh.secretHandler.GetSecretsForDeployment(ctx, appName, envName, deployment.Name)
		if err != nil {
			return nil, err
		}

		environmentDto.Secrets = deploymentSecrets
	}

	return environmentDto, nil
}

// CreateEnvironment Handler for CreateEnvironment. Creates an environment if it does not exist
func (eh EnvironmentHandler) CreateEnvironment(ctx context.Context, appName, envName string) (*v1.RadixEnvironment, error) {
	// ensure application exists
	rr, err := eh.radixclient.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// idempotent creation of RadixEnvironment
	re, err := eh.getServiceAccount().RadixClient.RadixV1().RadixEnvironments().Create(ctx, k8sObjectUtils.
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
func (eh EnvironmentHandler) DeleteEnvironment(ctx context.Context, appName, envName string) error {
	uniqueName := k8sObjectUtils.GetEnvironmentNamespace(appName, envName)
	re, err := eh.getRadixEnvironment(ctx, uniqueName)
	if err != nil {
		return err
	}

	if !re.Status.Orphaned {
		// Must be removed from radix config first
		return environmentModels.CannotDeleteNonOrphanedEnvironment(appName, envName)
	}

	// idempotent removal of RadixEnvironment
	err = eh.getServiceAccount().RadixClient.RadixV1().RadixEnvironments().Delete(ctx, uniqueName, metav1.DeleteOptions{})
	// if an error is anything other than not-found, return it
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

// GetEnvironmentEvents Handler for GetEnvironmentEvents
func (eh EnvironmentHandler) GetEnvironmentEvents(ctx context.Context, appName, envName string) ([]*eventModels.Event, error) {
	radixApplication, err := eh.getRadixApplicationInAppNamespace(ctx, appName)
	if err != nil {
		return nil, err
	}

	_, err = eh.getConfigurationStatus(ctx, envName, radixApplication)
	if err != nil {
		return nil, err
	}

	environmentEvents, err := eh.eventHandler.GetEvents(ctx, events.RadixEnvironmentNamespace(radixApplication, envName))
	if err != nil {
		return nil, err
	}

	return environmentEvents, nil
}

func (eh EnvironmentHandler) getConfigurationStatus(ctx context.Context, envName string, radixApplication *v1.RadixApplication) (environmentModels.ConfigurationStatus, error) {
	span, ctx := apm.StartSpan(ctx, fmt.Sprintf("getConfigurationStatus (appName=%s, envName=%s)", radixApplication.Name, envName), "EnvironmentHandler")
	defer span.End()
	uniqueName := k8sObjectUtils.GetEnvironmentNamespace(radixApplication.Name, envName)

	re, err := eh.getRadixEnvironment(ctx, uniqueName)
	exists := err == nil

	if !exists {
		// does not exist in radix regardless of config
		return environmentModels.Pending, environmentModels.NonExistingEnvironment(err, radixApplication.Name, envName)
	}

	if re.Status.Orphaned {
		// does not occur in config but is still an active resource
		return environmentModels.Orphan, nil
	}

	_, err = eh.inClusterClient.CoreV1().Namespaces().Get(ctx, uniqueName, metav1.GetOptions{})
	if err != nil {
		return environmentModels.Pending, nil
	}

	// exists and has underlying resources
	return environmentModels.Consistent, nil
}

func (eh EnvironmentHandler) getEnvironmentSummary(ctx context.Context, app *v1.RadixApplication, env v1.Environment) (*environmentModels.EnvironmentSummary, error) {
	span, ctx := apm.StartSpan(ctx, fmt.Sprintf("getEnvironmentSummary (appName=%s, envName=%s)", app.Name, env.Name), "EnvironmentHandler")
	defer span.End()
	environmentSummary := &environmentModels.EnvironmentSummary{
		Name:          env.Name,
		BranchMapping: env.Build.From,
	}

	deploymentSummaries, err := eh.deployHandler.GetDeploymentsForApplicationEnvironment(ctx, app.Name, env.Name, true)
	if err != nil {
		return nil, err
	}

	configurationStatus, _ := eh.getConfigurationStatus(ctx, env.Name, app)
	environmentSummary.Status = configurationStatus.String()

	if len(deploymentSummaries) == 1 {
		environmentSummary.ActiveDeployment = deploymentSummaries[0]
	}

	return environmentSummary, nil
}

func (eh EnvironmentHandler) getOrphanEnvironmentSummary(ctx context.Context, appName string, envName string) (*environmentModels.EnvironmentSummary, error) {
	deploymentSummaries, err := eh.deployHandler.GetDeploymentsForApplicationEnvironment(ctx, appName, envName, true)
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
func (eh EnvironmentHandler) getOrphanedEnvironments(ctx context.Context, appName string, radixApplication *v1.RadixApplication) ([]*environmentModels.EnvironmentSummary, error) {
	span, ctx := apm.StartSpan(ctx, fmt.Sprintf("getOrphanedEnvironments (appName=%s)", appName), "EnvironmentHandler")
	defer span.End()
	orphanedEnvironments := make([]*environmentModels.EnvironmentSummary, 0)

	for _, name := range eh.getOrphanedEnvNames(ctx, radixApplication) {
		summary, err := eh.getOrphanEnvironmentSummary(ctx, appName, name)
		if err != nil {
			return nil, err
		}

		orphanedEnvironments = append(orphanedEnvironments, summary)
	}

	return orphanedEnvironments, nil
}

// getOrphanedEnvNames returns a slice of non-unique-names of orphaned environments
func (eh EnvironmentHandler) getOrphanedEnvNames(ctx context.Context, app *v1.RadixApplication) []string {
	return eh.getEnvironments(ctx, app, true)
}

// getNotOrphanedEnvNames returns a slice of non-unique-names of not-orphaned environments
func (eh EnvironmentHandler) getNotOrphanedEnvNames(ctx context.Context, app *v1.RadixApplication) []string {
	return eh.getEnvironments(ctx, app, false)
}

func (eh EnvironmentHandler) getEnvironments(ctx context.Context, app *v1.RadixApplication, isOrphaned bool) []string {
	envNames := make([]string, 0)

	radixEnvironments, _ := eh.getServiceAccount().RadixClient.RadixV1().RadixEnvironments().List(ctx, metav1.ListOptions{
		LabelSelector: labelselector.ForApplication(app.Name).String(),
	})

	for _, re := range radixEnvironments.Items {
		if re.Status.Orphaned == isOrphaned {
			envNames = append(envNames, re.Spec.EnvName)
		}
	}

	return envNames
}

func (eh EnvironmentHandler) getServiceAccount() models.Account {
	return eh.accounts.ServiceAccount
}

// GetLogs handler for GetLogs
func (eh EnvironmentHandler) GetLogs(ctx context.Context, appName, envName, podName string, sinceTime *time.Time, logLines *int64, previousLog bool) (io.ReadCloser, error) {
	podHandler := pods.Init(eh.client)
	logger, err := podHandler.HandleGetEnvironmentPodLog(ctx, appName, envName, podName, "", sinceTime, logLines, previousLog)
	if errors.IsNotFound(err) {
		return nil, err
	}

	return logger, nil
}

// GetScheduledJobLogs handler for GetScheduledJobLogs
func (eh EnvironmentHandler) GetScheduledJobLogs(ctx context.Context, appName, envName, scheduledJobName string, sinceTime *time.Time, logLines *int64) (io.ReadCloser, error) {
	handler := pods.Init(eh.client)
	return handler.HandleGetEnvironmentScheduledJobLog(ctx, appName, envName, scheduledJobName, "", sinceTime, logLines)
}

// GetAuxiliaryResourcePodLog handler for GetAuxiliaryResourcePodLog
func (eh EnvironmentHandler) GetAuxiliaryResourcePodLog(ctx context.Context, appName, envName, componentName, auxType, podName string, sinceTime *time.Time, logLines *int64) (io.ReadCloser, error) {
	podHandler := pods.Init(eh.client)
	return podHandler.HandleGetEnvironmentAuxiliaryResourcePodLog(ctx, appName, envName, componentName, auxType, podName, sinceTime, logLines)
}

// StopEnvironment Stops all components in the environment
func (eh EnvironmentHandler) StopEnvironment(ctx context.Context, appName, envName string) error {
	_, radixDeployment, err := eh.getRadixDeployment(ctx, appName, envName)
	if err != nil {
		return err
	}

	log.Infof("Stopping components in environment %s, %s", envName, appName)
	for _, deployComponent := range radixDeployment.Spec.Components {
		err := eh.StopComponent(ctx, appName, envName, deployComponent.GetName(), true)
		if err != nil {
			return err
		}
	}
	return nil
}

// StartEnvironment Starts all components in the environment
func (eh EnvironmentHandler) StartEnvironment(ctx context.Context, appName, envName string) error {
	_, radixDeployment, err := eh.getRadixDeployment(ctx, appName, envName)
	if err != nil {
		return err
	}

	log.Infof("Starting components in environment %s, %s", envName, appName)
	for _, deployComponent := range radixDeployment.Spec.Components {
		err := eh.StartComponent(ctx, appName, envName, deployComponent.GetName(), true)
		if err != nil {
			return err
		}
	}
	return nil
}

// RestartEnvironment Restarts all components in the environment
func (eh EnvironmentHandler) RestartEnvironment(ctx context.Context, appName, envName string) error {
	_, radixDeployment, err := eh.getRadixDeployment(ctx, appName, envName)
	if err != nil {
		return err
	}

	log.Infof("Restarting components in environment %s, %s", envName, appName)
	for _, deployComponent := range radixDeployment.Spec.Components {
		err := eh.RestartComponent(ctx, appName, envName, deployComponent.GetName(), true)
		if err != nil {
			return err
		}
	}
	return nil
}

// StopApplication Stops all components in all environments of the application
func (eh EnvironmentHandler) StopApplication(ctx context.Context, appName string) error {
	radixApplication, err := eh.getRadixApplicationInAppNamespace(ctx, appName)
	if err != nil {
		return err
	}

	environmentNames := eh.getNotOrphanedEnvNames(ctx, radixApplication)
	log.Infof("Stopping components in the application %s", appName)
	for _, environmentName := range environmentNames {
		err := eh.StopEnvironment(ctx, appName, environmentName)
		if err != nil {
			return err
		}
	}
	return nil
}

// StartApplication Starts all components in all environments of the application
func (eh EnvironmentHandler) StartApplication(ctx context.Context, appName string) error {
	radixApplication, err := eh.getRadixApplicationInAppNamespace(ctx, appName)
	if err != nil {
		return err
	}

	environmentNames := eh.getNotOrphanedEnvNames(ctx, radixApplication)
	log.Infof("Starting components in the application %s", appName)
	for _, environmentName := range environmentNames {
		err := eh.StartEnvironment(ctx, appName, environmentName)
		if err != nil {
			return err
		}
	}
	return nil
}

// RestartApplication Restarts all components in all environments of the application
func (eh EnvironmentHandler) RestartApplication(ctx context.Context, appName string) error {
	radixApplication, err := eh.getRadixApplicationInAppNamespace(ctx, appName)
	if err != nil {
		return err
	}

	environmentNames := eh.getNotOrphanedEnvNames(ctx, radixApplication)
	log.Infof("Restarting components in the application %s", appName)
	for _, environmentName := range environmentNames {
		err := eh.RestartEnvironment(ctx, appName, environmentName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (eh EnvironmentHandler) getRadixCommonComponentUpdater(ctx context.Context, appName, envName, componentName string) (radixDeployCommonComponentUpdater, error) {
	deploymentSummary, rd, err := eh.getRadixDeployment(ctx, appName, envName)
	if err != nil {
		return nil, err
	}
	baseUpdater := &baseComponentUpdater{
		appName:         appName,
		envName:         envName,
		componentName:   componentName,
		radixDeployment: rd,
	}
	var updater radixDeployCommonComponentUpdater
	var componentToPatch v1.RadixCommonDeployComponent
	componentIndex, componentToPatch := deployUtils.GetDeploymentComponent(rd, componentName)
	if !radixutils.IsNil(componentToPatch) {
		updater = &radixDeployComponentUpdater{base: baseUpdater}
	} else {
		componentIndex, componentToPatch = deployUtils.GetDeploymentJobComponent(rd, componentName)
		if radixutils.IsNil(componentToPatch) {
			return nil, environmentModels.NonExistingComponent(appName, componentName)
		}
		updater = &radixDeployJobComponentUpdater{base: baseUpdater}
	}

	baseUpdater.componentIndex = componentIndex
	baseUpdater.componentToPatch = componentToPatch

	ra, _ := eh.getRadixApplicationInAppNamespace(ctx, appName)
	baseUpdater.environmentConfig = configUtils.GetComponentEnvironmentConfig(ra, envName, componentName)
	baseUpdater.componentState, err = deployments.GetComponentStateFromSpec(ctx, eh.client, appName, deploymentSummary, rd.Status, baseUpdater.environmentConfig, componentToPatch)
	if err != nil {
		return nil, err
	}
	return updater, nil
}

func (eh EnvironmentHandler) commit(ctx context.Context, updater radixDeployCommonComponentUpdater, commitFunc func(updater radixDeployCommonComponentUpdater) error) error {
	rd := updater.getRadixDeployment()
	oldJSON, err := json.Marshal(rd)
	if err != nil {
		return err
	}

	commitFunc(updater)
	newJSON, err := json.Marshal(rd)
	if err != nil {
		return err
	}
	err = eh.patch(ctx, rd.GetNamespace(), rd.GetName(), oldJSON, newJSON)
	if err != nil {
		return err
	}
	return nil
}
