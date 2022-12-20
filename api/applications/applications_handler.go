package applications

import (
	"context"
	"encoding/json"
	"fmt"
	"k8s.io/apimachinery/pkg/labels"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	"github.com/equinor/radix-api/api/deployments"
	"github.com/equinor/radix-api/api/environments"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	job "github.com/equinor/radix-api/api/jobs"
	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/api/utils/labelselector"
	"github.com/equinor/radix-api/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	jobPipeline "github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type patch struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

// ApplicationHandler Instance variables
type ApplicationHandler struct {
	jobHandler         job.JobHandler
	environmentHandler environments.EnvironmentHandler
	accounts           models.Accounts
	config             ApplicationHandlerConfig
	kubeUtil           *kube.Kube
}

// NewApplicationHandler Constructor
func NewApplicationHandler(accounts models.Accounts, config ApplicationHandlerConfig) ApplicationHandler {
	kubeUtil, _ := kube.New(accounts.UserAccount.Client, accounts.UserAccount.RadixClient, accounts.UserAccount.SecretProviderClient)
	return ApplicationHandler{
		accounts:           accounts,
		jobHandler:         job.Init(accounts, deployments.Init(accounts)),
		environmentHandler: environments.Init(environments.WithAccounts(accounts)),
		config:             config,
		kubeUtil:           kubeUtil,
	}
}

func (ah *ApplicationHandler) getUserAccount() models.Account {
	return ah.accounts.UserAccount
}

func (ah *ApplicationHandler) getServiceAccount() models.Account {
	return ah.accounts.ServiceAccount
}

// GetApplication handler for GetApplication
func (ah *ApplicationHandler) GetApplication(appName string) (*applicationModels.Application, error) {
	radixRegistration, err := ah.getServiceAccount().RadixClient.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	applicationRegistrationBuilder := NewBuilder()
	applicationRegistration := applicationRegistrationBuilder.
		withRadixRegistration(radixRegistration).
		Build()

	jobs, err := ah.jobHandler.GetApplicationJobs(appName)
	if err != nil {
		return nil, err
	}

	environments, err := ah.environmentHandler.GetEnvironmentSummary(appName)
	if err != nil {
		return nil, err
	}

	appAlias, err := ah.getAppAlias(appName, environments)
	if err != nil {
		return nil, err
	}

	machineUserTokenExpiration, err := ah.getMachineUserTokenExpiration(appName)
	if err != nil {
		return nil, err
	}

	return &applicationModels.Application{
		Name:                       applicationRegistration.Name,
		Registration:               applicationRegistration,
		Jobs:                       jobs,
		Environments:               environments,
		AppAlias:                   appAlias,
		Owner:                      applicationRegistration.Owner,
		Creator:                    applicationRegistration.Creator,
		MachineUserTokenExpiration: machineUserTokenExpiration}, nil
}

// RegenerateMachineUserToken Deletes the secret holding the token to force refresh and returns the new token
func (ah *ApplicationHandler) RegenerateMachineUserToken(appName string, expiryDays int) (*applicationModels.MachineUser, error) {
	log.Debugf("regenerate machine user token for app: %s", appName)
	namespace := crdUtils.GetAppNamespace(appName)
	machineUserSA, err := ah.getMachineUserServiceAccount(appName, namespace)
	if err != nil {
		return nil, err
	}

	secretConfig := getMachineUserTokenSecretConfig(appName, machineUserSA.Name, expiryDays)
	err = ah.deleteExistingMachineUserTokenSecrets(namespace)

	_, err = ah.getUserAccount().Client.CoreV1().Secrets(namespace).Create(context.TODO(), secretConfig, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	queryTimeout := time.NewTimer(time.Duration(5) * time.Second)
	queryInterval := time.NewTicker(time.Second)
	for {
		select {
		case <-queryInterval.C:
			secret, err := ah.getUserAccount().Client.CoreV1().Secrets(namespace).Get(context.TODO(), secretConfig.Name, metav1.GetOptions{})
			if err == nil {
				return &applicationModels.MachineUser{
					Token:               string(secret.Data[corev1.ServiceAccountTokenKey]),
					ExpirationTimestamp: secret.Annotations[defaults.ExpirationTimestampAnnotation],
				}, nil
			}
			log.Debugf("waiting to get machine user for app %s of namespace %s, error: %v", appName, namespace, err)
		case <-queryTimeout.C:
			return nil, fmt.Errorf("timeout getting user machine token secret")
		}
	}
}

func getMachineUserTokenSecretConfig(appName, machineUserServiceAccountName string, expiryDays int) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", crdUtils.GetServiceAccountSecretNamePrefix(machineUserServiceAccountName), uuid.New()),
			Annotations: map[string]string{
				corev1.ServiceAccountNameKey:                  machineUserServiceAccountName,
				defaults.ImmutableCreationTimestampAnnotation: time.Now().Format(time.RFC3339),
				defaults.ExpirationTimestampAnnotation:        time.Now().Add(time.Duration(expiryDays*24) * time.Hour).Format(time.RFC3339),
			},
			Labels: getMachineUserTokenSecretLabels(appName),
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}
}

// RegisterApplication handler for RegisterApplication
func (ah *ApplicationHandler) RegisterApplication(applicationRegistrationRequest applicationModels.ApplicationRegistrationRequest) (*applicationModels.ApplicationRegistrationUpsertResponse, error) {
	// Only if repository is provided and deploykey is not set by user
	// generate the key
	var deployKey *utils.DeployKey
	var err error

	application := applicationRegistrationRequest.ApplicationRegistration
	if (strings.TrimSpace(application.PublicKey) == "" && strings.TrimSpace(application.PrivateKey) != "") ||
		(strings.TrimSpace(application.PublicKey) != "" && strings.TrimSpace(application.PrivateKey) == "") {
		return nil, applicationModels.OnePartOfDeployKeyIsNotAllowed()
	}

	if strings.TrimSpace(application.Repository) != "" &&
		strings.TrimSpace(application.PublicKey) == "" {
		deployKey, err = utils.GenerateDeployKey()
		if err != nil {
			return nil, err
		}

		application.PublicKey = deployKey.PublicKey
		application.PrivateKey = deployKey.PrivateKey
	}
	creator, err := ah.accounts.GetUserAccountUserPrincipleName()
	if err != nil {
		return nil, err
	}

	application.RadixConfigFullName = cleanFileFullName(application.RadixConfigFullName)
	if len(application.RadixConfigFullName) > 0 {
		err = radixvalidators.ValidateRadixConfigFullName(application.RadixConfigFullName)
		if err != nil {
			return nil, err
		}
	}

	radixRegistration, err := NewBuilder().
		withAppRegistration(application).
		withDeployKey(deployKey).
		withCreator(creator).
		withRadixConfigFullName(application.RadixConfigFullName).
		BuildRR()
	if err != nil {
		return nil, err
	}

	err = ah.isValidRegistration(radixRegistration)
	if err != nil {
		return nil, err
	}

	if !applicationRegistrationRequest.AcknowledgeWarnings {
		if upsertResponse, err := ah.getRegistrationInsertResponseForWarnings(radixRegistration); upsertResponse != nil || err != nil {
			return upsertResponse, err
		}
	}

	_, err = ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Create(context.TODO(), radixRegistration, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return &applicationModels.ApplicationRegistrationUpsertResponse{
		ApplicationRegistration: application,
	}, nil
}

func (ah *ApplicationHandler) getRegistrationInsertResponseForWarnings(radixRegistration *v1.RadixRegistration) (*applicationModels.ApplicationRegistrationUpsertResponse, error) {
	warnings, err := ah.getRegistrationInsertWarnings(radixRegistration)
	if err != nil {
		return nil, err
	}
	if len(warnings) != 0 {
		return &applicationModels.ApplicationRegistrationUpsertResponse{Warnings: warnings}, nil
	}
	return nil, nil
}

func (ah *ApplicationHandler) getRegistrationUpdateResponseForWarnings(radixRegistration *v1.RadixRegistration) (*applicationModels.ApplicationRegistrationUpsertResponse, error) {
	warnings, err := ah.getRegistrationUpdateWarnings(radixRegistration)
	if err != nil {
		return nil, err
	}
	if len(warnings) != 0 {
		return &applicationModels.ApplicationRegistrationUpsertResponse{Warnings: warnings}, nil
	}
	return nil, nil
}

func cleanFileFullName(fileFullName string) string {
	return strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(fileFullName), "\\", "/"), "/")
}

// ChangeRegistrationDetails handler for ChangeRegistrationDetails
func (ah *ApplicationHandler) ChangeRegistrationDetails(appName string, applicationRegistrationRequest applicationModels.ApplicationRegistrationRequest) (*applicationModels.ApplicationRegistrationUpsertResponse, error) {
	application := applicationRegistrationRequest.ApplicationRegistration
	if appName != application.Name {
		return nil, radixhttp.ValidationError("Radix Registration", fmt.Sprintf("App name %s does not correspond with application name %s", appName, application.Name))
	}

	// Make check that this is an existing application
	existingRegistration, err := ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Only if repository is provided and deploykey is not set by user
	// generate the key
	var deployKey *utils.DeployKey

	if strings.TrimSpace(application.Repository) != "" &&
		strings.TrimSpace(application.PublicKey) == "" {
		deployKey, err = utils.GenerateDeployKey()
		if err != nil {
			return nil, err
		}

		application.PublicKey = deployKey.PublicKey
	}

	radixRegistration, err := NewBuilder().withAppRegistration(application).withDeployKey(deployKey).BuildRR()
	if err != nil {
		return nil, err
	}

	currentCloneURL := existingRegistration.Spec.CloneURL //currently repository is not changed here, but maybe later
	// Only these fields can change over time
	existingRegistration.Spec.CloneURL = radixRegistration.Spec.CloneURL
	existingRegistration.Spec.SharedSecret = radixRegistration.Spec.SharedSecret
	existingRegistration.Spec.DeployKey = radixRegistration.Spec.DeployKey
	existingRegistration.Spec.AdGroups = radixRegistration.Spec.AdGroups
	existingRegistration.Spec.Owner = radixRegistration.Spec.Owner
	existingRegistration.Spec.WBS = radixRegistration.Spec.WBS
	existingRegistration.Spec.ConfigBranch = radixRegistration.Spec.ConfigBranch
	existingRegistration.Spec.RadixConfigFullName = radixRegistration.Spec.RadixConfigFullName

	err = ah.isValidUpdate(existingRegistration)
	if err != nil {
		return nil, err
	}

	needToRevalidateWarnings := currentCloneURL != existingRegistration.Spec.CloneURL
	if needToRevalidateWarnings && !applicationRegistrationRequest.AcknowledgeWarnings {
		if upsertResponse, err := ah.getRegistrationUpdateResponseForWarnings(radixRegistration); upsertResponse != nil || err != nil {
			return upsertResponse, err
		}
	}
	_, err = ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Update(context.TODO(), existingRegistration, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return &applicationModels.ApplicationRegistrationUpsertResponse{
		ApplicationRegistration: application,
	}, nil
}

// ModifyRegistrationDetails handler for ModifyRegistrationDetails
func (ah *ApplicationHandler) ModifyRegistrationDetails(appName string, applicationRegistrationPatchRequest applicationModels.ApplicationRegistrationPatchRequest) (*applicationModels.ApplicationRegistrationUpsertResponse, error) {
	// Make check that this is an existing application
	existingRegistration, err := ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	payload := []patch{}

	runUpdate := false
	// Only these fields can change over time
	patchRequest := applicationRegistrationPatchRequest.ApplicationRegistrationPatch
	if patchRequest.AdGroups != nil && len(*patchRequest.AdGroups) > 0 && !radixutils.ArrayEqualElements(existingRegistration.Spec.AdGroups, *patchRequest.AdGroups) {
		existingRegistration.Spec.AdGroups = *patchRequest.AdGroups
		payload = append(payload, patch{Op: "replace", Path: "/spec/adGroups", Value: *patchRequest.AdGroups})
		runUpdate = true
	} else if patchRequest.AdGroups != nil && len(*patchRequest.AdGroups) == 0 {
		existingRegistration.Spec.AdGroups = nil
		payload = append(payload, patch{Op: "replace", Path: "/spec/adGroups", Value: nil})
		runUpdate = true
	}

	if patchRequest.Owner != nil && *patchRequest.Owner != "" {
		existingRegistration.Spec.Owner = *patchRequest.Owner
		payload = append(payload, patch{Op: "replace", Path: "/spec/owner", Value: *patchRequest.Owner})
		runUpdate = true
	}

	currentCloneURL := existingRegistration.Spec.CloneURL
	if patchRequest.Repository != nil && *patchRequest.Repository != "" {
		cloneURL := crdUtils.GetGithubCloneURLFromRepo(*patchRequest.Repository)
		existingRegistration.Spec.CloneURL = cloneURL
		payload = append(payload, patch{Op: "replace", Path: "/spec/cloneURL", Value: cloneURL})
		runUpdate = true
	}

	if patchRequest.MachineUser != nil && *patchRequest.MachineUser != existingRegistration.Spec.MachineUser {
		existingRegistration.Spec.MachineUser = *patchRequest.MachineUser
		payload = append(payload, patch{Op: "replace", Path: "/spec/machineUser", Value: patchRequest.MachineUser})
		runUpdate = true
	}

	if patchRequest.WBS != nil && *patchRequest.WBS != "" {
		existingRegistration.Spec.WBS = *patchRequest.WBS
		payload = append(payload, patch{Op: "replace", Path: "/spec/wbs", Value: *patchRequest.WBS})
		runUpdate = true
	}

	if patchRequest.ConfigBranch != nil {
		if trimmedBranch := strings.TrimSpace(*patchRequest.ConfigBranch); trimmedBranch != "" {
			existingRegistration.Spec.ConfigBranch = trimmedBranch
			payload = append(payload, patch{Op: "replace", Path: "/spec/configBranch", Value: trimmedBranch})
			runUpdate = true
		}
	}

	if setConfigBranchToFallbackWhenEmpty(existingRegistration) {
		payload = append(payload, patch{Op: "replace", Path: "/spec/configBranch", Value: applicationconfig.ConfigBranchFallback})
		runUpdate = true
	}

	radixConfigFullName := cleanFileFullName(patchRequest.RadixConfigFullName)
	if len(radixConfigFullName) > 0 && !strings.EqualFold(radixConfigFullName, existingRegistration.Spec.RadixConfigFullName) {
		err := radixvalidators.ValidateRadixConfigFullName(radixConfigFullName)
		if err != nil {
			return nil, err
		}
		existingRegistration.Spec.RadixConfigFullName = radixConfigFullName
		payload = append(payload, patch{Op: "replace", Path: "/spec/radixConfigFullName", Value: radixConfigFullName})
		runUpdate = true
	}

	if patchRequest.ConfigurationItem != nil {
		if trimmedConfigurationItem := strings.TrimSpace(*patchRequest.ConfigurationItem); trimmedConfigurationItem != "" {
			existingRegistration.Spec.ConfigurationItem = trimmedConfigurationItem
			payload = append(payload, patch{Op: "replace", Path: "/spec/configurationItem", Value: trimmedConfigurationItem})
			runUpdate = true
		}
	}

	if runUpdate {
		err = ah.isValidUpdate(existingRegistration)
		if err != nil {
			return nil, err
		}

		needToRevalidateWarnings := currentCloneURL != existingRegistration.Spec.CloneURL
		if needToRevalidateWarnings && !applicationRegistrationPatchRequest.AcknowledgeWarnings {
			if upsertResponse, err := ah.getRegistrationUpdateResponseForWarnings(existingRegistration); upsertResponse != nil || err != nil {
				return upsertResponse, err
			}
		}

		payloadBytes, _ := json.Marshal(payload)
		_, err = ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Patch(context.TODO(), existingRegistration.GetName(), types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
		if err != nil {
			return nil, err
		}
	}

	application := NewBuilder().withRadixRegistration(existingRegistration).Build()
	return &applicationModels.ApplicationRegistrationUpsertResponse{
		ApplicationRegistration: &application,
	}, nil
}

// DeleteApplication handler for DeleteApplication
func (ah *ApplicationHandler) DeleteApplication(appName string) error {
	// Make check that this is an existing application
	_, err := ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	err = ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Delete(context.TODO(), appName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return nil
}

// GetSupportedPipelines handler for GetSupportedPipelines
func (ah *ApplicationHandler) GetSupportedPipelines() []string {
	supportedPipelines := make([]string, 0)
	pipelines := jobPipeline.GetSupportedPipelines()
	for _, pipeline := range pipelines {
		supportedPipelines = append(supportedPipelines, string(pipeline.Type))
	}

	return supportedPipelines
}

// TriggerPipelineBuild Triggers build pipeline for an application
func (ah *ApplicationHandler) TriggerPipelineBuild(appName string, r *http.Request) (*jobModels.JobSummary, error) {
	pipelineName := "build"
	jobSummary, err := ah.triggerPipelineBuildOrBuildDeploy(appName, pipelineName, r)
	if err != nil {
		return nil, err
	}
	return jobSummary, nil
}

// TriggerPipelineBuildDeploy Triggers build-deploy pipeline for an application
func (ah *ApplicationHandler) TriggerPipelineBuildDeploy(appName string, r *http.Request) (*jobModels.JobSummary, error) {
	pipelineName := "build-deploy"
	jobSummary, err := ah.triggerPipelineBuildOrBuildDeploy(appName, pipelineName, r)
	if err != nil {
		return nil, err
	}
	return jobSummary, nil
}

// TriggerPipelinePromote Triggers promote pipeline for an application
func (ah *ApplicationHandler) TriggerPipelinePromote(appName string, r *http.Request) (*jobModels.JobSummary, error) {
	var pipelineParameters applicationModels.PipelineParametersPromote
	if err := json.NewDecoder(r.Body).Decode(&pipelineParameters); err != nil {
		return nil, err
	}

	deploymentName := pipelineParameters.DeploymentName
	fromEnvironment := pipelineParameters.FromEnvironment
	toEnvironment := pipelineParameters.ToEnvironment

	if strings.TrimSpace(deploymentName) == "" || strings.TrimSpace(fromEnvironment) == "" || strings.TrimSpace(toEnvironment) == "" {
		return nil, radixhttp.ValidationError("Radix Application Pipeline", "Deployment name, from environment and to environment are required for \"promote\" pipeline")
	}

	log.Infof("Creating promote pipeline job for %s using deployment %s from environment %s into environment %s", appName, deploymentName, fromEnvironment, toEnvironment)

	jobParameters := pipelineParameters.MapPipelineParametersPromoteToJobParameter()

	pipeline, err := jobPipeline.GetPipelineFromName("promote")
	if err != nil {
		return nil, err
	}

	jobSummary, err := ah.jobHandler.HandleStartPipelineJob(appName, pipeline, jobParameters)
	if err != nil {
		return nil, err
	}

	return jobSummary, nil
}

// TriggerPipelineDeploy Triggers deploy pipeline for an application
func (ah *ApplicationHandler) TriggerPipelineDeploy(appName string, r *http.Request) (*jobModels.JobSummary, error) {
	var pipelineParameters applicationModels.PipelineParametersDeploy
	if err := json.NewDecoder(r.Body).Decode(&pipelineParameters); err != nil {
		return nil, err
	}

	toEnvironment := pipelineParameters.ToEnvironment

	if strings.TrimSpace(toEnvironment) == "" {
		return nil, radixhttp.ValidationError("Radix Application Pipeline", "To environment is required for \"deploy\" pipeline")
	}

	log.Infof("Creating deploy pipeline job for %s into environment %s", appName, toEnvironment)

	pipeline, err := jobPipeline.GetPipelineFromName("deploy")
	if err != nil {
		return nil, err
	}

	jobParameters := pipelineParameters.MapPipelineParametersDeployToJobParameter()

	jobSummary, err := ah.jobHandler.HandleStartPipelineJob(appName, pipeline, jobParameters)
	if err != nil {
		return nil, err
	}

	return jobSummary, nil
}

func (ah *ApplicationHandler) triggerPipelineBuildOrBuildDeploy(appName, pipelineName string, r *http.Request) (*jobModels.JobSummary, error) {
	var pipelineParameters applicationModels.PipelineParametersBuild
	if err := json.NewDecoder(r.Body).Decode(&pipelineParameters); err != nil {
		return nil, err
	}

	branch := pipelineParameters.Branch
	commitID := pipelineParameters.CommitID

	if strings.TrimSpace(appName) == "" || strings.TrimSpace(branch) == "" {
		return nil, applicationModels.AppNameAndBranchAreRequiredForStartingPipeline()
	}

	log.Infof("Creating build pipeline job for %s on branch %s for commit %s", appName, branch, commitID)

	radixRegistration, err := ah.getServiceAccount().RadixClient.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Check if branch is mapped
	if !applicationconfig.IsConfigBranch(branch, radixRegistration) {
		application, err := utils.CreateApplicationConfig(ah.getUserAccount().Client, ah.getUserAccount().RadixClient, ah.getUserAccount().SecretProviderClient, appName)
		if err != nil {
			return nil, err
		}
		isThereAnythingToDeploy, _ := application.IsThereAnythingToDeploy(branch)

		if !isThereAnythingToDeploy {
			return nil, applicationModels.UnmatchedBranchToEnvironment(branch)
		}
	}

	jobParameters := pipelineParameters.MapPipelineParametersBuildToJobParameter()

	pipeline, err := jobPipeline.GetPipelineFromName(pipelineName)
	if err != nil {
		return nil, err
	}

	jobSummary, err := ah.jobHandler.HandleStartPipelineJob(appName, pipeline, jobParameters)
	if err != nil {
		return nil, err
	}

	return jobSummary, nil
}

func (ah *ApplicationHandler) getRegistrationInsertWarnings(radixRegistration *v1.RadixRegistration) ([]string, error) {
	return radixvalidators.GetRadixRegistrationBeInsertedWarnings(ah.getServiceAccount().RadixClient, radixRegistration)
}

func (ah *ApplicationHandler) getRegistrationUpdateWarnings(radixRegistration *v1.RadixRegistration) ([]string, error) {
	return radixvalidators.GetRadixRegistrationBeUpdatedWarnings(ah.getServiceAccount().RadixClient, radixRegistration)
}

func (ah *ApplicationHandler) isValidRegistration(radixRegistration *v1.RadixRegistration) error {
	// Need to use in-cluster client of the API server, because the user might not have enough priviledges
	// to run a full validation
	return radixvalidators.CanRadixRegistrationBeInserted(ah.getServiceAccount().RadixClient, radixRegistration, ah.getAdditionalRadixRegistrationValidators()...)
}

func (ah *ApplicationHandler) isValidUpdate(radixRegistration *v1.RadixRegistration) error {
	return radixvalidators.CanRadixRegistrationBeUpdated(radixRegistration, ah.getAdditionalRadixRegistrationValidators()...)
}

func (ah *ApplicationHandler) getAdditionalRadixRegistrationValidators() []radixvalidators.RadixRegistrationValidator {
	var validators []radixvalidators.RadixRegistrationValidator

	if ah.config.RequireAppConfigurationItem {
		validators = append(validators, radixvalidators.RequireConfigurationItem)
	}

	return validators
}

func (ah *ApplicationHandler) getAppAlias(appName string, environments []*environmentModels.EnvironmentSummary) (*applicationModels.ApplicationAlias, error) {
	for _, environment := range environments {
		environmentNamespace := crdUtils.GetEnvironmentNamespace(appName, environment.Name)

		ingresses, err := ah.getUserAccount().Client.NetworkingV1().Ingresses(environmentNamespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelselector.ForIsAppAlias().String(),
		})

		if err != nil {
			return nil, err
		}

		if len(ingresses.Items) > 0 {
			// Will only be one alias, if any exists
			componentName := ingresses.Items[0].Labels[kube.RadixComponentLabel]
			environmentName := environment.Name
			url := ingresses.Items[0].Spec.Rules[0].Host
			return &applicationModels.ApplicationAlias{ComponentName: componentName, EnvironmentName: environmentName, URL: url}, nil
		}
	}

	return nil, nil
}

func (ah *ApplicationHandler) getMachineUserForApp(appName string) (*applicationModels.MachineUser, error) {
	namespace := crdUtils.GetAppNamespace(appName)

	log.Debugf("get service account for machine user in app %s of namespace %s", appName, namespace)
	machineUserSA, err := ah.getMachineUserServiceAccount(appName, namespace)
	if err != nil {
		return nil, err
	}

	if len(machineUserSA.Secrets) == 0 {
		return nil, fmt.Errorf("unable to get secrets on machine user service account")
	}

	tokenName := machineUserSA.Secrets[0].Name
	log.Debugf("get secrets for machine user token %s in app %s of namespace %s", tokenName, appName, namespace)
	token, err := ah.getUserAccount().Client.CoreV1().Secrets(namespace).Get(context.TODO(), tokenName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	tokenStringData := string(token.Data["token"])
	log.Debugf("token length: %v", len(tokenStringData))
	tokenString := &tokenStringData

	return &applicationModels.MachineUser{
		Token: *tokenString,
	}, nil
}

func (ah *ApplicationHandler) getMachineUserServiceAccount(appName, namespace string) (*corev1.ServiceAccount, error) {
	machineUserName := defaults.GetMachineUserRoleName(appName)
	log.Debugf("get service account for app %s in namespace %s and machine user: %s", appName, namespace, machineUserName)
	machineUserSA, err := ah.getServiceAccount().Client.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), machineUserName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return machineUserSA, nil
}

// RegenerateDeployKey Regenerates deploy key and secret and returns the new key
func (ah *ApplicationHandler) RegenerateDeployKey(appName string, regenerateDeployKeyAndSecretData applicationModels.RegenerateDeployKeyAndSecretData) (*applicationModels.DeployKeyAndSecret, error) {
	// Make check that this is an existing application and user has access to it
	existingRegistration, err := ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	sharedKey := strings.TrimSpace(regenerateDeployKeyAndSecretData.SharedSecret)
	if len(sharedKey) == 0 {
		return nil, fmt.Errorf("shared secret cannot be empty")
	}
	deployKey, err := utils.GenerateDeployKey()
	if err != nil {
		return nil, err
	}

	existingRegistration.Spec.DeployKey = deployKey.PrivateKey
	existingRegistration.Spec.DeployKeyPublic = deployKey.PublicKey
	existingRegistration.Spec.SharedSecret = sharedKey
	setConfigBranchToFallbackWhenEmpty(existingRegistration)

	err = ah.isValidUpdate(existingRegistration)
	if err != nil {
		return nil, err
	}

	_, err = ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Update(context.TODO(), existingRegistration, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	return &applicationModels.DeployKeyAndSecret{
		PublicDeployKey: deployKey.PublicKey,
		SharedSecret:    sharedKey,
	}, nil
}

func (ah *ApplicationHandler) deleteExistingMachineUserTokenSecrets(appName string) error {
	namespace := crdUtils.GetAppNamespace(appName)
	existingSecrets, err := ah.kubeUtil.ListSecretsWithSelector(namespace, labels.Set(getMachineUserTokenSecretLabels(appName)).String())
	if err != nil {
		return err
	}
	for _, existingSecret := range existingSecrets {
		err = ah.kubeUtil.DeleteSecret(namespace, existingSecret.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ah *ApplicationHandler) getMachineUserTokenExpiration(appName string) (string, error) {
	existingSecrets, err := ah.kubeUtil.ListSecretsWithSelector(crdUtils.GetAppNamespace(appName), labels.Set(getMachineUserTokenSecretLabels(appName)).String())
	if err != nil {
		return "", err
	}
	if len(existingSecrets) == 0 {
		return "", nil
	}
	if len(existingSecrets) == 1 {
		return existingSecrets[0].ObjectMeta.Annotations[defaults.ExpirationTimestampAnnotation], nil
	}
	log.Warnf("more than one machine user token secret found for app %s", appName)
	existingSecretsSortedDesc := sortSecretsByCreatedDesc(existingSecrets)
	return existingSecretsSortedDesc[0].ObjectMeta.Annotations[defaults.ExpirationTimestampAnnotation], nil
}

func getMachineUserTokenSecretLabels(appName string) map[string]string {
	return map[string]string{
		defaults.RadixMachineUserTokenSecretAnnotation: strconv.FormatBool(true),
		kube.RadixAppLabel: appName,
	}
}

func setConfigBranchToFallbackWhenEmpty(existingRegistration *v1.RadixRegistration) bool {
	// HACK ConfigBranch is required, so we set it to "master" if empty to support existing apps registered before ConfigBranch was introduced
	if len(strings.TrimSpace(existingRegistration.Spec.ConfigBranch)) > 0 {
		return false
	}
	existingRegistration.Spec.ConfigBranch = applicationconfig.ConfigBranchFallback
	return true
}

func sortSecretsByCreatedDesc(secrets []*corev1.Secret) []*corev1.Secret {
	sort.Slice(secrets, func(i, j int) bool {
		return isCreatedBefore(secrets[j], secrets[i])
	})
	return secrets
}

func isCreatedAfter(secret1 *corev1.Secret, secret2 *corev1.Secret) bool {
	secret1CreatedStr := secret1.ObjectMeta.Annotations[defaults.ImmutableCreationTimestampAnnotation]
	secret1TimeStamp, err := time.Parse(time.RFC3339, secret1CreatedStr)
	if err != nil {
		return true
	}
	secret2CreatedStr := secret2.ObjectMeta.Annotations[defaults.ImmutableCreationTimestampAnnotation]
	secret2TimeStamp, err := time.Parse(time.RFC3339, secret2CreatedStr)
	if err != nil {
		return false
	}
	return secret1TimeStamp.After(secret2TimeStamp)
}

func isCreatedBefore(secret1 *corev1.Secret, secret2 *corev1.Secret) bool {
	return !isCreatedAfter(secret1, secret2)
}
