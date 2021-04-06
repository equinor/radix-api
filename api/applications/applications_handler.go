package applications

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	"github.com/equinor/radix-api/api/environments"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	job "github.com/equinor/radix-api/api/jobs"
	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/utils"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/equinor/radix-api/models"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	jobPipeline "github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	k8sObjectUtils "github.com/equinor/radix-operator/pkg/apis/utils"
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
}

// Init Constructor
func Init(accounts models.Accounts) ApplicationHandler {
	jobHandler := job.Init(accounts)
	return ApplicationHandler{
		accounts:           accounts,
		jobHandler:         jobHandler,
		environmentHandler: environments.Init(environments.WithAccounts(accounts)),
	}
}

func (ah ApplicationHandler) getUserAccount() models.Account {
	return ah.accounts.UserAccount
}

func (ah ApplicationHandler) getServiceAccount() models.Account {
	return ah.accounts.ServiceAccount
}

// GetApplication handler for GetApplication
func (ah ApplicationHandler) GetApplication(appName string) (*applicationModels.Application, error) {
	radixRegistration, err := ah.getServiceAccount().RadixClient.RadixV1().RadixRegistrations().Get(appName, metav1.GetOptions{})
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
func (ah ApplicationHandler) RegenerateMachineUserToken(appName string) (*applicationModels.MachineUser, error) {
	log.Debugf("regenerate machine user token for app: %s", appName)
	namespace := crdUtils.GetAppNamespace(appName)
	machineUserSA, err := ah.getMachineUserServiceAccount(appName, namespace)
	if err != nil {
		return nil, err
	}
	if len(machineUserSA.Secrets) == 0 {
		return nil, fmt.Errorf("Unable to get secrets on machine user service account")
	}

	tokenName := machineUserSA.Secrets[0].Name
	log.Debugf("delete service account for app %s and machine user token: %s", appName, tokenName)
	err = ah.getUserAccount().Client.CoreV1().Secrets(namespace).Delete(tokenName, &metav1.DeleteOptions{})

	queryTimeout := time.NewTimer(time.Duration(5) * time.Second)
	queryInterval := time.Tick(time.Duration(1) * time.Second)
	for {
		select {
		case <-queryInterval:
			machineUser, err := ah.getMachineUserForApp(appName)
			if err == nil {
				return machineUser, nil
			}
			log.Debugf("waiting to get machine user for app %s of namespace %s, error: %v", appName, namespace, err)
		case <-queryTimeout.C:
			return nil, fmt.Errorf("Timeout getting user machine token secret")
		}
	}
}

// RegisterApplication handler for RegisterApplication
func (ah ApplicationHandler) RegisterApplication(application applicationModels.ApplicationRegistration) (*applicationModels.ApplicationRegistration, error) {
	// Only if repository is provided and deploykey is not set by user
	// generate the key
	var deployKey *utils.DeployKey
	var err error

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

	radixRegistration, err := NewBuilder().withAppRegistration(&application).withDeployKey(deployKey).withCreator(creator).BuildRR()
	if err != nil {
		return nil, err
	}

	err = ah.isValidRegistration(radixRegistration)
	if err != nil {
		return nil, err
	}

	_, err = ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Create(radixRegistration)
	if err != nil {
		return nil, err
	}

	return &application, nil
}

// ChangeRegistrationDetails handler for ChangeRegistrationDetails
func (ah ApplicationHandler) ChangeRegistrationDetails(appName string, application applicationModels.ApplicationRegistration) (*applicationModels.ApplicationRegistration, error) {
	if appName != application.Name {
		return nil, utils.ValidationError("Radix Registration", fmt.Sprintf("App name %s does not correspond with application name %s", appName, application.Name))
	}

	// Make check that this is an existing application
	existingRegistration, err := ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Get(appName, metav1.GetOptions{})
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

	radixRegistration, err := NewBuilder().withAppRegistration(&application).withDeployKey(deployKey).BuildRR()
	if err != nil {
		return nil, err
	}

	// Only these fields can change over time
	existingRegistration.Spec.CloneURL = radixRegistration.Spec.CloneURL
	existingRegistration.Spec.SharedSecret = radixRegistration.Spec.SharedSecret
	existingRegistration.Spec.DeployKey = radixRegistration.Spec.DeployKey
	existingRegistration.Spec.AdGroups = radixRegistration.Spec.AdGroups
	existingRegistration.Spec.Owner = radixRegistration.Spec.Owner
	existingRegistration.Spec.WBS = radixRegistration.Spec.WBS
	existingRegistration.Spec.ConfigBranch = radixRegistration.Spec.ConfigBranch

	err = ah.isValidUpdate(existingRegistration)
	if err != nil {
		return nil, err
	}

	_, err = ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Update(existingRegistration)
	if err != nil {
		return nil, err
	}

	return &application, nil
}

// ModifyRegistrationDetails handler for ModifyRegistrationDetails
func (ah ApplicationHandler) ModifyRegistrationDetails(appName string, patchRequest applicationModels.ApplicationPatchRequest) (*applicationModels.ApplicationRegistration, error) {
	// Make check that this is an existing application
	existingRegistration, err := ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Get(appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	payload := []patch{}

	runUpdate := false
	// Only these fields can change over time
	if patchRequest.AdGroups != nil && len(*patchRequest.AdGroups) > 0 && !k8sObjectUtils.ArrayEqualElements(existingRegistration.Spec.AdGroups, *patchRequest.AdGroups) {
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

	if runUpdate {
		err = ah.isValidUpdate(existingRegistration)
		if err != nil {
			return nil, err
		}

		payloadBytes, _ := json.Marshal(payload)
		_, err = ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Patch(existingRegistration.GetName(), types.JSONPatchType, payloadBytes)
		if err != nil {
			return nil, err
		}
	}

	application := NewBuilder().withRadixRegistration(existingRegistration).Build()
	return &application, nil
}

// DeleteApplication handler for DeleteApplication
func (ah ApplicationHandler) DeleteApplication(appName string) error {
	// Make check that this is an existing application
	_, err := ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Get(appName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	err = ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Delete(appName, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return nil
}

// GetSupportedPipelines handler for GetSupportedPipelines
func (ah ApplicationHandler) GetSupportedPipelines() []string {
	supportedPipelines := make([]string, 0)
	pipelines := jobPipeline.GetSupportedPipelines()
	for _, pipeline := range pipelines {
		supportedPipelines = append(supportedPipelines, string(pipeline.Type))
	}

	return supportedPipelines
}

// TriggerPipelineBuild Triggers build pipeline for an application
func (ah ApplicationHandler) TriggerPipelineBuild(appName string, r *http.Request) (*jobModels.JobSummary, error) {
	pipelineName := "build"
	jobSummary, err := ah.triggerPipelineBuildOrBuildDeploy(appName, pipelineName, r)
	if err != nil {
		return nil, err
	}
	return jobSummary, nil
}

// TriggerPipelineBuildDeploy Triggers build-deploy pipeline for an application
func (ah ApplicationHandler) TriggerPipelineBuildDeploy(appName string, r *http.Request) (*jobModels.JobSummary, error) {
	pipelineName := "build-deploy"
	jobSummary, err := ah.triggerPipelineBuildOrBuildDeploy(appName, pipelineName, r)
	if err != nil {
		return nil, err
	}
	return jobSummary, nil
}

// TriggerPipelinePromote Triggers promote pipeline for an application
func (ah ApplicationHandler) TriggerPipelinePromote(appName string, r *http.Request) (*jobModels.JobSummary, error) {
	var pipelineParameters applicationModels.PipelineParametersPromote
	if err := json.NewDecoder(r.Body).Decode(&pipelineParameters); err != nil {
		return nil, err
	}

	deploymentName := pipelineParameters.DeploymentName
	fromEnvironment := pipelineParameters.FromEnvironment
	toEnvironment := pipelineParameters.ToEnvironment

	if strings.TrimSpace(deploymentName) == "" || strings.TrimSpace(fromEnvironment) == "" || strings.TrimSpace(toEnvironment) == "" {
		return nil, utils.ValidationError("Radix Application Pipeline", "Deployment name, from environment and to environment are required for \"promote\" pipeline")
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
func (ah ApplicationHandler) TriggerPipelineDeploy(appName string, r *http.Request) (*jobModels.JobSummary, error) {
	var pipelineParameters applicationModels.PipelineParametersDeploy
	if err := json.NewDecoder(r.Body).Decode(&pipelineParameters); err != nil {
		return nil, err
	}

	toEnvironment := pipelineParameters.ToEnvironment

	if strings.TrimSpace(toEnvironment) == "" {
		return nil, utils.ValidationError("Radix Application Pipeline", "To environment is required for \"deploy\" pipeline")
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

func (ah ApplicationHandler) triggerPipelineBuildOrBuildDeploy(appName, pipelineName string, r *http.Request) (*jobModels.JobSummary, error) {
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

	radixRegistration, err := ah.getServiceAccount().RadixClient.RadixV1().RadixRegistrations().Get(appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Check if branch is mapped
	if !applicationconfig.IsConfigBranch(branch, radixRegistration) {
		application, err := utils.CreateApplicationConfig(ah.getUserAccount().Client, ah.getUserAccount().RadixClient, appName)
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

func (ah ApplicationHandler) isValidRegistration(radixRegistration *v1.RadixRegistration) error {
	// Need to use in-cluster client of the API server, because the user might not have enough priviledges
	// to run a full validation
	_, err := radixvalidators.CanRadixRegistrationBeInserted(ah.getServiceAccount().RadixClient, radixRegistration)
	if err != nil {
		return err
	}

	return nil
}

func (ah ApplicationHandler) isValidUpdate(radixRegistration *v1.RadixRegistration) error {
	// Need to use in-cluster client of the API server, because the user might not have enough priviledges
	// to run a full validation
	_, err := radixvalidators.CanRadixRegistrationBeUpdated(ah.getServiceAccount().RadixClient, radixRegistration)
	if err != nil {
		return err
	}

	return err
}

func (ah ApplicationHandler) getAppAlias(appName string, environments []*environmentModels.EnvironmentSummary) (*applicationModels.ApplicationAlias, error) {
	for _, environment := range environments {
		environmentNamespace := k8sObjectUtils.GetEnvironmentNamespace(appName, environment.Name)

		ingresses, err := ah.getUserAccount().Client.NetworkingV1beta1().Ingresses(environmentNamespace).List(metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", "radix-app-alias", "true"),
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

func (ah ApplicationHandler) getMachineUserForApp(appName string) (*applicationModels.MachineUser, error) {
	namespace := crdUtils.GetAppNamespace(appName)

	log.Debugf("get service account for machine user in app %s of namespace %s", appName, namespace)
	machineUserSA, err := ah.getMachineUserServiceAccount(appName, namespace)
	if err != nil {
		return nil, err
	}

	if len(machineUserSA.Secrets) == 0 {
		return nil, fmt.Errorf("Unable to get secrets on machine user service account")
	}

	tokenName := machineUserSA.Secrets[0].Name
	log.Debugf("get secrets for machine user token %s in app %s of namespace %s", tokenName, appName, namespace)
	token, err := ah.getUserAccount().Client.CoreV1().Secrets(namespace).Get(tokenName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	tokenStringData := string(token.Data["token"])
	log.Debugf("token length: %v", len(tokenStringData))
	var tokenString *string
	tokenString = &tokenStringData

	return &applicationModels.MachineUser{
		Token: *tokenString,
	}, nil
}

func (ah ApplicationHandler) getMachineUserServiceAccount(appName, namespace string) (*corev1.ServiceAccount, error) {
	machineUserName := defaults.GetMachineUserRoleName(appName)
	log.Debugf("get service account for app %s in namespace %s and machine user: %s", appName, namespace, machineUserName)
	machineUserSA, err := ah.getServiceAccount().Client.CoreV1().ServiceAccounts(namespace).Get(machineUserName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return machineUserSA, nil
}

// Builder Handles construction of DTO
type Builder interface {
	withName(name string) Builder
	withOwner(owner string) Builder
	withCreator(creator string) Builder
	withRepository(string) Builder
	withSharedSecret(string) Builder
	withAdGroups([]string) Builder
	withPublicKey(string) Builder
	withPrivateKey(string) Builder
	withCloneURL(string) Builder
	withDeployKey(*utils.DeployKey) Builder
	withMachineUser(bool) Builder
	withWBS(string) Builder
	withConfigBranch(string) Builder
	withAppRegistration(appRegistration *applicationModels.ApplicationRegistration) Builder
	withRadixRegistration(*v1.RadixRegistration) Builder
	Build() applicationModels.ApplicationRegistration
	BuildRR() (*v1.RadixRegistration, error)
}

type applicationBuilder struct {
	name         string
	owner        string
	creator      string
	repository   string
	sharedSecret string
	adGroups     []string
	publicKey    string
	privateKey   string
	cloneURL     string
	machineUser  bool
	wbs          string
	configBranch string
}

func (rb *applicationBuilder) withAppRegistration(appRegistration *applicationModels.ApplicationRegistration) Builder {
	rb.withName(appRegistration.Name)
	rb.withRepository(appRegistration.Repository)
	rb.withSharedSecret(appRegistration.SharedSecret)
	rb.withAdGroups(appRegistration.AdGroups)
	rb.withPublicKey(appRegistration.PublicKey)
	rb.withPrivateKey(appRegistration.PrivateKey)
	rb.withOwner(appRegistration.Owner)
	rb.withWBS(appRegistration.WBS)
	rb.withConfigBranch(appRegistration.ConfigBranch)
	return rb
}

func (rb *applicationBuilder) withRadixRegistration(radixRegistration *v1.RadixRegistration) Builder {
	rb.withName(radixRegistration.Name)
	rb.withCloneURL(radixRegistration.Spec.CloneURL)
	rb.withSharedSecret(radixRegistration.Spec.SharedSecret)
	rb.withAdGroups(radixRegistration.Spec.AdGroups)
	rb.withPublicKey(radixRegistration.Spec.DeployKeyPublic)
	rb.withOwner(radixRegistration.Spec.Owner)
	rb.withCreator(radixRegistration.Spec.Creator)
	rb.withMachineUser(radixRegistration.Spec.MachineUser)
	rb.withWBS(radixRegistration.Spec.WBS)
	rb.withConfigBranch(radixRegistration.Spec.ConfigBranch)

	// Private part of key should never be returned
	return rb
}

func (rb *applicationBuilder) withName(name string) Builder {
	rb.name = name
	return rb
}

func (rb *applicationBuilder) withOwner(owner string) Builder {
	rb.owner = owner
	return rb
}

func (rb *applicationBuilder) withCreator(creator string) Builder {
	rb.creator = creator
	return rb
}

func (rb *applicationBuilder) withRepository(repository string) Builder {
	rb.repository = repository
	return rb
}

func (rb *applicationBuilder) withCloneURL(cloneURL string) Builder {
	rb.cloneURL = cloneURL
	return rb
}

func (rb *applicationBuilder) withSharedSecret(sharedSecret string) Builder {
	rb.sharedSecret = sharedSecret
	return rb
}

func (rb *applicationBuilder) withAdGroups(adGroups []string) Builder {
	rb.adGroups = adGroups
	return rb
}

func (rb *applicationBuilder) withPublicKey(publicKey string) Builder {
	rb.publicKey = strings.TrimSuffix(publicKey, "\n")
	return rb
}

func (rb *applicationBuilder) withPrivateKey(privateKey string) Builder {
	rb.privateKey = strings.TrimSuffix(privateKey, "\n")
	return rb
}

func (rb *applicationBuilder) withDeployKey(deploykey *utils.DeployKey) Builder {
	if deploykey != nil {
		rb.publicKey = deploykey.PublicKey
		rb.privateKey = deploykey.PrivateKey
	}

	return rb
}

func (rb *applicationBuilder) withMachineUser(machineUser bool) Builder {
	rb.machineUser = machineUser
	return rb
}

func (rb *applicationBuilder) withWBS(wbs string) Builder {
	rb.wbs = wbs
	return rb
}

func (rb *applicationBuilder) withConfigBranch(configBranch string) Builder {
	rb.configBranch = configBranch
	return rb
}

func (rb *applicationBuilder) Build() applicationModels.ApplicationRegistration {
	repository := rb.repository
	if repository == "" {
		repository = crdUtils.GetGithubRepositoryURLFromCloneURL(rb.cloneURL)
	}

	return applicationModels.ApplicationRegistration{
		Name:         rb.name,
		Repository:   repository,
		SharedSecret: rb.sharedSecret,
		AdGroups:     rb.adGroups,
		PublicKey:    rb.publicKey,
		PrivateKey:   rb.privateKey,
		Owner:        rb.owner,
		Creator:      rb.creator,
		MachineUser:  rb.machineUser,
		WBS:          rb.wbs,
		ConfigBranch: rb.configBranch,
	}
}

func (rb *applicationBuilder) BuildRR() (*v1.RadixRegistration, error) {
	builder := crdUtils.NewRegistrationBuilder()

	radixRegistration := builder.
		WithPublicKey(rb.publicKey).
		WithPrivateKey(rb.privateKey).
		WithName(rb.name).
		WithRepository(rb.repository).
		WithSharedSecret(rb.sharedSecret).
		WithAdGroups(rb.adGroups).
		WithOwner(rb.owner).
		WithCreator(rb.creator).
		WithMachineUser(rb.machineUser).
		WithWBS(rb.wbs).
		WithConfigBranch(rb.configBranch).
		BuildRR()

	return radixRegistration, nil
}

// NewBuilder Constructor for application builder
func NewBuilder() Builder {
	return &applicationBuilder{}
}

// AnApplicationRegistration Constructor for application builder with test values
func AnApplicationRegistration() Builder {
	return &applicationBuilder{
		name:         "my-app",
		repository:   "https://github.com/Equinor/my-app",
		sharedSecret: "AnySharedSecret",
		adGroups:     []string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"},
		owner:        "a_test_user@equinor.com",
		creator:      "a_test_user@equinor.com",
		wbs:          "T.O123A.AZ.45678",
		configBranch: "main",
	}
}

// RegenerateDeployKey Regenerates deploy key and secret and returns the new key
func (ah ApplicationHandler) RegenerateDeployKey(appName string, regenerateDeployKeyAndSecretData applicationModels.RegenerateDeployKeyAndSecretData) (*applicationModels.DeployKeyAndSecret, error) {
	// Make check that this is an existing application and user has access to it
	existingRegistration, err := ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Get(appName, metav1.GetOptions{})
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

	_, err = ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Update(existingRegistration)
	if err != nil {
		return nil, err
	}
	return &applicationModels.DeployKeyAndSecret{
		PublicDeployKey: deployKey.PublicKey,
		SharedSecret:    sharedKey,
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
