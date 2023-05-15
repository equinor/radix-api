package applications

import (
	"context"
	"encoding/json"
	"fmt"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"net/http"
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
}

// NewApplicationHandler Constructor
func NewApplicationHandler(accounts models.Accounts, config ApplicationHandlerConfig) ApplicationHandler {
	return ApplicationHandler{
		accounts:           accounts,
		jobHandler:         job.Init(accounts, deployments.Init(accounts)),
		environmentHandler: environments.Init(environments.WithAccounts(accounts)),
		config:             config,
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

	return &applicationModels.Application{
		Name:         applicationRegistration.Name,
		Registration: applicationRegistration,
		Jobs:         jobs,
		Environments: environments,
		AppAlias:     appAlias,
		Owner:        applicationRegistration.Owner,
		Creator:      applicationRegistration.Creator}, nil
}

// RegenerateMachineUserToken Deletes the secret holding the token to force refresh and returns the new token
func (ah *ApplicationHandler) RegenerateMachineUserToken(appName string) (*applicationModels.MachineUser, error) {
	log.Debugf("regenerate machine user token for app: %s", appName)
	namespace := crdUtils.GetAppNamespace(appName)
	machineUserSA, err := ah.getMachineUserServiceAccount(appName, namespace)
	if err != nil {
		return nil, err
	}
	if len(machineUserSA.Secrets) == 0 {
		return nil, fmt.Errorf("unable to get secrets on machine user service account")
	}

	tokenName := machineUserSA.Secrets[0].Name
	log.Debugf("delete service account for app %s and machine user token: %s", appName, tokenName)
	if err := ah.getUserAccount().Client.CoreV1().Secrets(namespace).Delete(context.TODO(), tokenName, metav1.DeleteOptions{}); err != nil {
		return nil, err
	}

	queryTimeout := time.NewTimer(time.Duration(5) * time.Second)
	queryInterval := time.NewTicker(time.Second)
	for {
		select {
		case <-queryInterval.C:
			machineUser, err := ah.getMachineUserForApp(appName)
			if err == nil {
				return machineUser, nil
			}
			log.Debugf("waiting to get machine user for app %s of namespace %s, error: %v", appName, namespace, err)
		case <-queryTimeout.C:
			return nil, fmt.Errorf("timeout getting user machine token secret")
		}
	}
}

// RegisterApplication handler for RegisterApplication
func (ah *ApplicationHandler) RegisterApplication(applicationRegistrationRequest applicationModels.ApplicationRegistrationRequest) (*applicationModels.ApplicationRegistrationUpsertResponse, error) {
	var err error

	application := applicationRegistrationRequest.ApplicationRegistration

	creator, err := ah.accounts.GetOriginator()
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
	if len(application.SharedSecret) == 0 {
		application.SharedSecret = radixutils.RandString(20)
		log.Debugf("There is no Shared Secret specified for the registering application - a random Shared Secret has been generated")
	}

	radixRegistration, err := NewBuilder().
		withAppRegistration(application).
		withCreator(creator).
		withRadixConfigFullName(application.RadixConfigFullName).
		BuildRR()
	if err != nil {
		return nil, err
	}

	err = ah.isValidRegistrationInsert(radixRegistration)
	if err != nil {
		return nil, err
	}

	if !applicationRegistrationRequest.AcknowledgeWarnings {
		if upsertResponse, err := ah.getRegistrationInsertResponseForWarnings(radixRegistration); upsertResponse != nil || err != nil {
			return upsertResponse, err
		}
	}

	radixRegistration, err = ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Create(context.TODO(), radixRegistration, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	newApplication := NewBuilder().withRadixRegistration(radixRegistration).Build()
	return &applicationModels.ApplicationRegistrationUpsertResponse{
		ApplicationRegistration: &newApplication,
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
	currentRegistration, err := ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	radixRegistration, err := NewBuilder().withAppRegistration(application).BuildRR()
	if err != nil {
		return nil, err
	}

	updatedRegistration := currentRegistration.DeepCopy()

	// Only these fields can change over time
	updatedRegistration.Spec.CloneURL = radixRegistration.Spec.CloneURL
	updatedRegistration.Spec.SharedSecret = radixRegistration.Spec.SharedSecret
	updatedRegistration.Spec.AdGroups = radixRegistration.Spec.AdGroups
	updatedRegistration.Spec.Owner = radixRegistration.Spec.Owner
	updatedRegistration.Spec.WBS = radixRegistration.Spec.WBS
	updatedRegistration.Spec.ConfigurationItem = radixRegistration.Spec.ConfigurationItem
	updatedRegistration.Spec.ConfigBranch = radixRegistration.Spec.ConfigBranch
	updatedRegistration.Spec.RadixConfigFullName = radixRegistration.Spec.RadixConfigFullName

	err = ah.isValidRegistrationUpdate(updatedRegistration, currentRegistration)
	if err != nil {
		return nil, err
	}

	needToRevalidateWarnings := updatedRegistration.Spec.CloneURL != currentRegistration.Spec.CloneURL
	if needToRevalidateWarnings && !applicationRegistrationRequest.AcknowledgeWarnings {
		if upsertResponse, err := ah.getRegistrationUpdateResponseForWarnings(radixRegistration); upsertResponse != nil || err != nil {
			return upsertResponse, err
		}
	}
	updatedRegistration, err = ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Update(context.TODO(), updatedRegistration, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	updatedApplication := NewBuilder().withRadixRegistration(updatedRegistration).Build()
	return &applicationModels.ApplicationRegistrationUpsertResponse{
		ApplicationRegistration: &updatedApplication,
	}, nil
}

// ModifyRegistrationDetails handler for ModifyRegistrationDetails
func (ah *ApplicationHandler) ModifyRegistrationDetails(appName string, applicationRegistrationPatchRequest applicationModels.ApplicationRegistrationPatchRequest) (*applicationModels.ApplicationRegistrationUpsertResponse, error) {
	// Make check that this is an existing application
	currentRegistration, err := ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	payload := []patch{}
	runUpdate := false
	updatedRegistration := currentRegistration.DeepCopy()

	// Only these fields can change over time
	patchRequest := applicationRegistrationPatchRequest.ApplicationRegistrationPatch
	if patchRequest.AdGroups != nil && !radixutils.ArrayEqualElements(currentRegistration.Spec.AdGroups, *patchRequest.AdGroups) {
		updatedRegistration.Spec.AdGroups = *patchRequest.AdGroups
		payload = append(payload, patch{Op: "replace", Path: "/spec/adGroups", Value: *patchRequest.AdGroups})
		runUpdate = true
	}

	if patchRequest.Owner != nil && *patchRequest.Owner != "" {
		updatedRegistration.Spec.Owner = *patchRequest.Owner
		payload = append(payload, patch{Op: "replace", Path: "/spec/owner", Value: *patchRequest.Owner})
		runUpdate = true
	}

	if patchRequest.Repository != nil && *patchRequest.Repository != "" {
		cloneURL := crdUtils.GetGithubCloneURLFromRepo(*patchRequest.Repository)
		updatedRegistration.Spec.CloneURL = cloneURL
		payload = append(payload, patch{Op: "replace", Path: "/spec/cloneURL", Value: cloneURL})
		runUpdate = true
	}

	if patchRequest.MachineUser != nil && *patchRequest.MachineUser != currentRegistration.Spec.MachineUser {
		if *patchRequest.MachineUser {
			return nil, fmt.Errorf("machine user token is deprecated. Please use AD Service principal access token https://radix.equinor.com/guides/deploy-only/#ad-service-principal-access-token")
		}
		updatedRegistration.Spec.MachineUser = *patchRequest.MachineUser
		payload = append(payload, patch{Op: "replace", Path: "/spec/machineUser", Value: patchRequest.MachineUser})
		runUpdate = true
	}

	if patchRequest.WBS != nil && *patchRequest.WBS != "" {
		updatedRegistration.Spec.WBS = *patchRequest.WBS
		payload = append(payload, patch{Op: "replace", Path: "/spec/wbs", Value: *patchRequest.WBS})
		runUpdate = true
	}

	if patchRequest.ConfigBranch != nil {
		if trimmedBranch := strings.TrimSpace(*patchRequest.ConfigBranch); trimmedBranch != "" {
			updatedRegistration.Spec.ConfigBranch = trimmedBranch
			payload = append(payload, patch{Op: "replace", Path: "/spec/configBranch", Value: trimmedBranch})
			runUpdate = true
		}
	}

	if setConfigBranchToFallbackWhenEmpty(updatedRegistration) {
		payload = append(payload, patch{Op: "replace", Path: "/spec/configBranch", Value: applicationconfig.ConfigBranchFallback})
		runUpdate = true
	}

	radixConfigFullName := cleanFileFullName(patchRequest.RadixConfigFullName)
	if len(radixConfigFullName) > 0 && !strings.EqualFold(radixConfigFullName, currentRegistration.Spec.RadixConfigFullName) {
		err := radixvalidators.ValidateRadixConfigFullName(radixConfigFullName)
		if err != nil {
			return nil, err
		}
		updatedRegistration.Spec.RadixConfigFullName = radixConfigFullName
		payload = append(payload, patch{Op: "replace", Path: "/spec/radixConfigFullName", Value: radixConfigFullName})
		runUpdate = true
	}

	if patchRequest.ConfigurationItem != nil {
		if trimmedConfigurationItem := strings.TrimSpace(*patchRequest.ConfigurationItem); trimmedConfigurationItem != "" {
			updatedRegistration.Spec.ConfigurationItem = trimmedConfigurationItem
			payload = append(payload, patch{Op: "replace", Path: "/spec/configurationItem", Value: trimmedConfigurationItem})
			runUpdate = true
		}
	}

	if runUpdate {
		err = ah.isValidRegistrationUpdate(updatedRegistration, currentRegistration)
		if err != nil {
			return nil, err
		}

		needToRevalidateWarnings := currentRegistration.Spec.CloneURL != updatedRegistration.Spec.CloneURL
		if needToRevalidateWarnings && !applicationRegistrationPatchRequest.AcknowledgeWarnings {
			if upsertResponse, err := ah.getRegistrationUpdateResponseForWarnings(updatedRegistration); upsertResponse != nil || err != nil {
				return upsertResponse, err
			}
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}

		updatedRegistration, err = ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Patch(context.TODO(), updatedRegistration.GetName(), types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
		if err != nil {
			return nil, err
		}
	}

	updatedApplication := NewBuilder().withRadixRegistration(updatedRegistration).Build()
	return &applicationModels.ApplicationRegistrationUpsertResponse{
		ApplicationRegistration: &updatedApplication,
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

	log.Infof("Creating build pipeline job for %s on branch %s for commit %s", appName, branch, commitID)

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

func (ah *ApplicationHandler) isValidRegistrationInsert(radixRegistration *v1.RadixRegistration) error {
	// Need to use in-cluster client of the API server, because the user might not have enough priviledges
	// to run a full validation
	return radixvalidators.CanRadixRegistrationBeInserted(ah.getServiceAccount().RadixClient, radixRegistration, ah.getAdditionalRadixRegistrationInsertValidators()...)
}

func (ah *ApplicationHandler) isValidRegistrationUpdate(updatedRegistration, currentRegistration *v1.RadixRegistration) error {
	return radixvalidators.CanRadixRegistrationBeUpdated(updatedRegistration, ah.getAdditionalRadixRegistrationUpdateValidators(currentRegistration)...)
}

func (ah *ApplicationHandler) getAdditionalRadixRegistrationInsertValidators() []radixvalidators.RadixRegistrationValidator {
	var validators []radixvalidators.RadixRegistrationValidator

	if ah.config.RequireAppConfigurationItem {
		validators = append(validators, radixvalidators.RequireConfigurationItem)
	}

	if ah.config.RequireAppADGroups {
		validators = append(validators, radixvalidators.RequireAdGroups)
	}

	return validators
}

func (ah *ApplicationHandler) getAdditionalRadixRegistrationUpdateValidators(currentRegistration *v1.RadixRegistration) []radixvalidators.RadixRegistrationValidator {
	var validators []radixvalidators.RadixRegistrationValidator

	if ah.config.RequireAppConfigurationItem && currentRegistration != nil && len(currentRegistration.Spec.ConfigurationItem) > 0 {
		validators = append(validators, radixvalidators.RequireConfigurationItem)
	}

	if ah.config.RequireAppADGroups && currentRegistration != nil && len(currentRegistration.Spec.AdGroups) > 0 {
		validators = append(validators, radixvalidators.RequireAdGroups)
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
func (ah *ApplicationHandler) RegenerateDeployKey(appName string, regenerateDeployKeyAndSecretData applicationModels.RegenerateDeployKeyAndSecretData) error {
	// Make check that this is an existing application and user has access to it
	currentRegistration, err := ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	sharedKey := strings.TrimSpace(regenerateDeployKeyAndSecretData.SharedSecret)
	updatedRegistration := currentRegistration.DeepCopy()
	if len(sharedKey) != 0 {
		updatedRegistration.Spec.SharedSecret = sharedKey
	}

	// Deleting SSH keys from RRs where these deprecated fields are populated
	updatedRegistration.Spec.DeployKeyPublic = ""
	updatedRegistration.Spec.DeployKey = ""

	setConfigBranchToFallbackWhenEmpty(updatedRegistration)

	err = ah.isValidRegistrationUpdate(updatedRegistration, currentRegistration)
	if err != nil {
		return err
	}

	_, err = ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Update(context.TODO(), updatedRegistration, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	if regenerateDeployKeyAndSecretData.PrivateKey != "" {
		// Deriving the public key from the private key in order to test it for validity
		_, err := crdUtils.DeriveDeployKeyFromPrivateKey(regenerateDeployKeyAndSecretData.PrivateKey)
		if err != nil {
			return fmt.Errorf("failed to derive public key from private key: %v", err)
		}
		exisingSecret, err := ah.getUserAccount().Client.CoreV1().Secrets(crdUtils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		newSecret := exisingSecret.DeepCopy()
		newSecret.Data[defaults.GitPrivateKeySecretKey] = []byte(regenerateDeployKeyAndSecretData.PrivateKey)
		_, err = ah.getUserAccount().Client.CoreV1().Secrets(crdUtils.GetAppNamespace(appName)).Update(context.TODO(), newSecret, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	} else {
		// Deleting the secret with the private key. This triggers the RR to be reconciled and the new key to be generated
		err = ah.getUserAccount().Client.CoreV1().Secrets(crdUtils.GetAppNamespace(appName)).Delete(context.TODO(), defaults.GitPrivateKeySecretName, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (ah *ApplicationHandler) GetDeployKeyAndSecret(appName string) (*applicationModels.DeployKeyAndSecret, error) {
	cm, err := ah.getUserAccount().Client.CoreV1().ConfigMaps(crdUtils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPublicKeyConfigMapName, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	}
	publicKey := ""
	if cm != nil {
		publicKey = cm.Data[defaults.GitPublicKeyConfigMapKey]
	}
	rr, err := ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	sharedSecret := rr.Spec.SharedSecret
	return &applicationModels.DeployKeyAndSecret{
		PublicDeployKey: publicKey,
		SharedSecret:    sharedSecret,
	}, nil
}

func setConfigBranchToFallbackWhenEmpty(existingRegistration *v1.RadixRegistration) bool {
	// HACK ConfigBranch is required, so we set it to "master" if empty to support existing apps registered before ConfigBranch was introduced
	if len(strings.TrimSpace(existingRegistration.Spec.ConfigBranch)) > 0 {
		return false
	}
	existingRegistration.Spec.ConfigBranch = applicationconfig.ConfigBranchFallback
	return true
}
