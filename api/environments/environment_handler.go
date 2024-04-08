package environments

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/equinor/radix-api/api/deployments"
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	"github.com/equinor/radix-api/api/events"
	eventModels "github.com/equinor/radix-api/api/events/models"
	"github.com/equinor/radix-api/api/kubequery"
	apimodels "github.com/equinor/radix-api/api/models"
	"github.com/equinor/radix-api/api/pods"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/api/utils/jobscheduler"
	"github.com/equinor/radix-api/api/utils/predicate"
	"github.com/equinor/radix-api/api/utils/tlsvalidation"
	"github.com/equinor/radix-api/models"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	deployUtils "github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	k8sObjectUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
)

// EnvironmentHandlerOptions defines a configuration function
type EnvironmentHandlerOptions func(*EnvironmentHandler)

// WithAccounts configures all EnvironmentHandler fields
func WithAccounts(accounts models.Accounts) EnvironmentHandlerOptions {
	return func(eh *EnvironmentHandler) {
		eh.client = accounts.UserAccount.Client
		eh.radixclient = accounts.UserAccount.RadixClient
		eh.deployHandler = deployments.Init(accounts)
		eh.eventHandler = events.Init(accounts.UserAccount.Client)
		eh.accounts = accounts
		kubeUtil, _ := kube.New(accounts.UserAccount.Client, accounts.UserAccount.RadixClient, accounts.UserAccount.SecretProviderClient)
		eh.kubeUtil = kubeUtil
		kubeUtilsForServiceAccount, _ := kube.New(accounts.ServiceAccount.Client, accounts.ServiceAccount.RadixClient, accounts.ServiceAccount.SecretProviderClient)
		eh.kubeUtilForServiceAccount = kubeUtilsForServiceAccount
		eh.jobSchedulerHandlerFactory = jobscheduler.NewFactory(kubeUtil)
	}
}

// WithEventHandler configures the eventHandler used by EnvironmentHandler
func WithEventHandler(eventHandler events.EventHandler) EnvironmentHandlerOptions {
	return func(eh *EnvironmentHandler) {
		eh.eventHandler = eventHandler
	}
}

// WithTLSValidator configures the tlsValidator used by EnvironmentHandler
func WithTLSValidator(validator tlsvalidation.Validator) EnvironmentHandlerOptions {
	return func(eh *EnvironmentHandler) {
		eh.tlsValidator = validator
	}
}

// WithJobSchedulerHandlerFactory configures the jobSchedulerHandlerFactory used by EnvironmentHandler
func WithJobSchedulerHandlerFactory(jobSchedulerHandlerFactory jobscheduler.HandlerFactoryInterface) EnvironmentHandlerOptions {
	return func(eh *EnvironmentHandler) {
		eh.jobSchedulerHandlerFactory = jobSchedulerHandlerFactory
	}
}

// EnvironmentHandlerFactory defines a factory function for EnvironmentHandler
type EnvironmentHandlerFactory func(accounts models.Accounts) EnvironmentHandler

// NewEnvironmentHandlerFactory creates a new EnvironmentHandlerFactory
func NewEnvironmentHandlerFactory(opts ...EnvironmentHandlerOptions) EnvironmentHandlerFactory {
	return func(accounts models.Accounts) EnvironmentHandler {
		// We must make a new slice and copy values from opts into it.
		// Appending to the original opts will modify its underlying array and cause a memory leak.
		newOpts := make([]EnvironmentHandlerOptions, len(opts), len(opts)+1)
		copy(newOpts, opts)
		newOpts = append(newOpts, WithAccounts(accounts))
		eh := Init(newOpts...)
		return eh
	}
}

// EnvironmentHandler Instance variables
type EnvironmentHandler struct {
	client                     kubernetes.Interface
	radixclient                radixclient.Interface
	deployHandler              deployments.DeployHandler
	eventHandler               events.EventHandler
	accounts                   models.Accounts
	kubeUtil                   *kube.Kube
	kubeUtilForServiceAccount  *kube.Kube
	tlsValidator               tlsvalidation.Validator
	jobSchedulerHandlerFactory jobscheduler.HandlerFactoryInterface
}

var validaStatusesToScaleComponent []string

// Init Constructor.
// Use the WithAccounts configuration function to configure a 'ready to use' EnvironmentHandler.
// EnvironmentHandlerOptions are processed in the sequence they are passed to this function.
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
	rr, err := kubequery.GetRadixRegistration(ctx, eh.accounts.UserAccount.RadixClient, appName)
	if err != nil {
		return nil, err
	}
	ra, err := kubequery.GetRadixApplication(ctx, eh.accounts.UserAccount.RadixClient, appName)
	if err != nil {
		// This is no error, as the application may only have been just registered
		if errors.IsNotFound(err) {
			return []*environmentModels.EnvironmentSummary{}, nil
		}
		return nil, err
	}
	reList, err := kubequery.GetRadixEnvironments(ctx, eh.accounts.ServiceAccount.RadixClient, appName)
	if err != nil {
		return nil, err
	}
	rjList, err := kubequery.GetRadixJobs(ctx, eh.accounts.UserAccount.RadixClient, appName)
	if err != nil {
		return nil, err
	}
	envNames := slice.Map(reList, func(re v1.RadixEnvironment) string { return re.Spec.EnvName })
	rdList, err := kubequery.GetRadixDeploymentsForEnvironments(ctx, eh.accounts.UserAccount.RadixClient, appName, envNames, 10)
	if err != nil {
		return nil, err
	}

	environments := apimodels.BuildEnvironmentSummaryList(rr, ra, reList, rdList, rjList)
	return environments, nil
}

// GetEnvironment Handler for GetEnvironment
func (eh EnvironmentHandler) GetEnvironment(ctx context.Context, appName, envName string) (*environmentModels.Environment, error) {
	rr, err := kubequery.GetRadixRegistration(ctx, eh.accounts.UserAccount.RadixClient, appName)
	if err != nil {
		return nil, err
	}

	ra, err := kubequery.GetRadixApplication(ctx, eh.accounts.UserAccount.RadixClient, appName)
	if err != nil {
		return nil, err
	}
	re, err := kubequery.GetRadixEnvironment(ctx, eh.accounts.ServiceAccount.RadixClient, appName, envName)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, environmentModels.NonExistingEnvironment(err, appName, envName)
		}
		return nil, err
	}
	rdList, err := kubequery.GetRadixDeploymentsForEnvironment(ctx, eh.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return nil, err
	}
	rjList, err := kubequery.GetRadixJobs(ctx, eh.accounts.UserAccount.RadixClient, appName)
	if err != nil {
		return nil, err
	}
	deploymentList, err := kubequery.GetDeploymentsForEnvironment(ctx, eh.accounts.UserAccount.Client, appName, envName)
	if err != nil {
		return nil, err
	}
	componentPodList, err := kubequery.GetPodsForEnvironmentComponents(ctx, eh.accounts.UserAccount.Client, appName, envName)
	if err != nil {
		return nil, err
	}
	hpaList, err := kubequery.GetHorizontalPodAutoscalersForEnvironment(ctx, eh.accounts.UserAccount.Client, appName, envName)
	if err != nil {
		return nil, err
	}

	noJobPayloadReq, err := labels.NewRequirement(kube.RadixSecretTypeLabel, selection.NotEquals, []string{string(kube.RadixSecretJobPayload)})
	if err != nil {
		return nil, err
	}
	secretList, err := kubequery.GetSecretsForEnvironment(ctx, eh.accounts.ServiceAccount.Client, appName, envName, *noJobPayloadReq)
	if err != nil {
		return nil, err
	}
	secretProviderClassList, err := kubequery.GetSecretProviderClassesForEnvironment(ctx, eh.accounts.ServiceAccount.SecretProviderClient, appName, envName)
	if err != nil {
		return nil, err
	}
	eventList, err := kubequery.GetEventsForEnvironment(ctx, eh.accounts.UserAccount.Client, appName, envName)
	if err != nil {
		return nil, err
	}

	env := apimodels.BuildEnvironment(rr, ra, re, rdList, rjList, deploymentList, componentPodList, hpaList, secretList, secretProviderClassList, eventList, eh.tlsValidator)
	return env, nil
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

	_, err = kubequery.GetRadixEnvironment(ctx, eh.accounts.ServiceAccount.RadixClient, appName, envName)
	// _, err = eh.getConfigurationStatus(ctx, envName, radixApplication)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, environmentModels.NonExistingEnvironment(err, appName, envName)
		}
		return nil, err
	}

	environmentEvents, err := eh.eventHandler.GetEvents(ctx, events.RadixEnvironmentNamespace(radixApplication, envName))
	if err != nil {
		return nil, err
	}

	return environmentEvents, nil
}

// getNotOrphanedEnvNames returns a slice of non-unique-names of not-orphaned environments
func (eh EnvironmentHandler) getNotOrphanedEnvNames(ctx context.Context, appName string) ([]string, error) {
	reList, err := kubequery.GetRadixEnvironments(ctx, eh.accounts.ServiceAccount.RadixClient, appName)
	if err != nil {
		return nil, err
	}
	return slice.Map(
		slice.FindAll(reList, predicate.IsNotOrphanEnvironment),
		func(re v1.RadixEnvironment) string { return re.Spec.EnvName },
	), nil
}

func (eh EnvironmentHandler) getServiceAccount() models.Account {
	return eh.accounts.ServiceAccount
}

// GetLogs handler for GetLogs
func (eh EnvironmentHandler) GetLogs(ctx context.Context, appName, envName, podName string, sinceTime *time.Time, logLines *int64, previousLog bool) (io.ReadCloser, error) {
	podHandler := pods.Init(eh.client)
	return podHandler.HandleGetEnvironmentPodLog(ctx, appName, envName, podName, "", sinceTime, logLines, previousLog)
}

// GetScheduledJobLogs handler for GetScheduledJobLogs
func (eh EnvironmentHandler) GetScheduledJobLogs(ctx context.Context, appName, envName, scheduledJobName, replicaName string, sinceTime *time.Time, logLines *int64) (io.ReadCloser, error) {
	handler := pods.Init(eh.client)
	return handler.HandleGetEnvironmentScheduledJobLog(ctx, appName, envName, scheduledJobName, replicaName, "", sinceTime, logLines)
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

	log.Ctx(ctx).Info().Msgf("Stopping components in environment %s, %s", envName, appName)
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

	log.Ctx(ctx).Info().Msgf("Starting components in environment %s, %s", envName, appName)
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

	log.Ctx(ctx).Info().Msgf("Restarting components in environment %s, %s", envName, appName)
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
	environmentNames, err := eh.getNotOrphanedEnvNames(ctx, appName)
	if err != nil {
		return err
	}
	log.Ctx(ctx).Info().Msgf("Stopping components in the application %s", appName)
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
	environmentNames, err := eh.getNotOrphanedEnvNames(ctx, appName)
	if err != nil {
		return err
	}
	log.Ctx(ctx).Info().Msgf("Starting components in the application %s", appName)
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
	environmentNames, err := eh.getNotOrphanedEnvNames(ctx, appName)
	if err != nil {
		return err
	}
	log.Ctx(ctx).Info().Msgf("Restarting components in the application %s", appName)
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
	baseUpdater.environmentConfig = utils.GetComponentEnvironmentConfig(ra, envName, componentName)
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

	if err := commitFunc(updater); err != nil {
		return err
	}
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
