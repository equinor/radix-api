package applications

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	"github.com/equinor/radix-api/api/environments"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	job "github.com/equinor/radix-api/api/jobs"
	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/utils"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/equinor/radix-api/models"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	jobPipeline "github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	k8sObjectUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
)

// ApplicationHandler Instance variables
type ApplicationHandler struct {
	userAccount    models.Account
	serviceAccount models.Account
	jobHandler     job.JobHandler
}

// Init Constructor
func Init(
	client kubernetes.Interface,
	radixClient radixclient.Interface,
	inClusterClient kubernetes.Interface,
	inClusterRadixClient radixclient.Interface) ApplicationHandler {

	jobHandler := job.Init(client, radixClient, inClusterClient, inClusterRadixClient)
	return ApplicationHandler{
		userAccount: models.Account{
			Client:      client,
			RadixClient: radixClient,
		},
		serviceAccount: models.Account{
			Client:      inClusterClient,
			RadixClient: inClusterRadixClient,
		},
		jobHandler: jobHandler}
}

// GetApplication handler for GetApplication
func (ah ApplicationHandler) GetApplication(appName string) (*applicationModels.Application, error) {
	radixRegistration, err := ah.serviceAccount.RadixClient.RadixV1().RadixRegistrations().Get(appName, metav1.GetOptions{})
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

	environmentHandler := environments.Init(ah.userAccount.Client, ah.userAccount.RadixClient)
	environments, err := environmentHandler.GetEnvironmentSummary(appName)
	if err != nil {
		return nil, err
	}

	appAlias, err := ah.getAppAlias(appName, environments)
	if err != nil {
		return nil, err
	}

	return &applicationModels.Application{Name: applicationRegistration.Name, Registration: applicationRegistration, Jobs: jobs, Environments: environments, AppAlias: appAlias}, nil
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

	radixRegistration, err := NewBuilder().withAppRegistration(&application).withDeployKey(deployKey).BuildRR()
	if err != nil {
		return nil, err
	}

	err = ah.isValidRegistration(radixRegistration)
	if err != nil {
		return nil, err
	}

	_, err = ah.userAccount.RadixClient.RadixV1().RadixRegistrations().Create(radixRegistration)
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
	existingRegistration, err := ah.userAccount.RadixClient.RadixV1().RadixRegistrations().Get(appName, metav1.GetOptions{})
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

	err = ah.isValidUpdate(existingRegistration)
	if err != nil {
		return nil, err
	}

	_, err = ah.userAccount.RadixClient.RadixV1().RadixRegistrations().Update(existingRegistration)
	if err != nil {
		return nil, err
	}

	return &application, nil
}

// ModifyRegistrationDetails handler for ModifyRegistrationDetails
func (ah ApplicationHandler) ModifyRegistrationDetails(appName string, patchRequest applicationModels.ApplicationPatchRequest) (*applicationModels.ApplicationRegistration, error) {
	// Make check that this is an existing application
	existingRegistration, err := ah.userAccount.RadixClient.RadixV1().RadixRegistrations().Get(appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Only these fields can change over time
	if !k8sObjectUtils.ArrayEqualElements(existingRegistration.Spec.AdGroups, patchRequest.AdGroups) {
		existingRegistration.Spec.AdGroups = patchRequest.AdGroups

		err = ah.isValidUpdate(existingRegistration)
		if err != nil {
			return nil, err
		}

		_, err = ah.userAccount.RadixClient.RadixV1().RadixRegistrations().Update(existingRegistration)
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
	_, err := ah.userAccount.RadixClient.RadixV1().RadixRegistrations().Get(appName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	err = ah.userAccount.RadixClient.RadixV1().RadixRegistrations().Delete(appName, &metav1.DeleteOptions{})
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
		supportedPipelines = append(supportedPipelines, pipeline.Name)
	}

	return supportedPipelines
}

// TriggerPipeline handler for TriggerPipeline
func (ah ApplicationHandler) TriggerPipeline(appName, pipelineName string, r *http.Request) (*jobModels.JobSummary, error) {
	var jobSummary *jobModels.JobSummary
	var err error

	switch pipelineName {
	case string(v1.Build), string(v1.BuildDeploy):
		var pipelineParameters applicationModels.PipelineParametersBuild
		if err = json.NewDecoder(r.Body).Decode(&pipelineParameters); err != nil {
			return nil, err
		}
		jobSummary, err = ah.TriggerPipelineBuild(appName, pipelineName, pipelineParameters)
	case string(v1.Promote):
		var pipelineParameters applicationModels.PipelineParametersPromote
		if err = json.NewDecoder(r.Body).Decode(&pipelineParameters); err != nil {
			return nil, err
		}
		jobSummary, err = ah.triggerPipelinePromote(appName, pipelineParameters)
	default:
		return nil, utils.ValidationError("Radix Application Pipeline", fmt.Sprintf("Pipeline %s not supported", pipelineName))
	}

	if err != nil {
		return nil, err
	}

	return jobSummary, nil
}

// TriggerPipelineBuild Triggers pipeline for an application
func (ah ApplicationHandler) TriggerPipelineBuild(appName, pipelineName string, pipelineParameters applicationModels.PipelineParametersBuild) (*jobModels.JobSummary, error) {
	branch := pipelineParameters.Branch
	commitID := pipelineParameters.CommitID

	if strings.TrimSpace(appName) == "" || strings.TrimSpace(branch) == "" {
		return nil, applicationModels.AppNameAndBranchAreRequiredForStartingPipeline()
	}

	log.Infof("Creating build pipeline job for %s on branch %s for commit %s", appName, branch, commitID)
	app, err := ah.GetApplication(appName)
	if err != nil {
		return nil, err
	}

	// Check if branch is mapped
	if !applicationconfig.IsMagicBranch(branch) {
		registration, err := ah.userAccount.RadixClient.RadixV1().RadixRegistrations().Get(appName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		config, err := ah.userAccount.RadixClient.RadixV1().RadixApplications(crdUtils.GetAppNamespace(appName)).Get(appName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		application, err := applicationconfig.NewApplicationConfig(ah.userAccount.Client, ah.userAccount.RadixClient, registration, config)
		if err != nil {
			return nil, err
		}

		branchIsMapped, _ := application.IsBranchMappedToEnvironment(branch)

		if !branchIsMapped {
			return nil, applicationModels.UnmatchedBranchToEnvironment(branch)
		}
	}

	jobParameters := &jobModels.JobParameters{
		Branch:    branch,
		CommitID:  commitID,
		PushImage: pipelineParameters.PushImageToContainerRegistry(),
	}

	pipeline, err := jobPipeline.GetPipelineFromName(pipelineName)
	if err != nil {
		return nil, err
	}

	jobSummary, err := ah.jobHandler.HandleStartPipelineJob(appName, crdUtils.GetGithubCloneURLFromRepo(app.Registration.Repository), pipeline, jobParameters)
	if err != nil {
		return nil, err
	}

	return jobSummary, nil
}

func (ah ApplicationHandler) triggerPipelinePromote(appName string, pipelineParameters applicationModels.PipelineParametersPromote) (*jobModels.JobSummary, error) {
	deploymentName := pipelineParameters.DeploymentName
	fromEnvironment := pipelineParameters.FromEnvironment
	toEnvironment := pipelineParameters.ToEnvironment

	if strings.TrimSpace(deploymentName) == "" || strings.TrimSpace(fromEnvironment) == "" || strings.TrimSpace(toEnvironment) == "" {
		return nil, utils.ValidationError("Radix Application Pipeline", "Deployment name, from environment and to environment are required for \"promote\" pipeline")
	}

	log.Infof("Creating promote pipeline job for %s using deployment %s from environment %s into environment %s", appName, deploymentName, fromEnvironment, toEnvironment)
	app, err := ah.GetApplication(appName)
	if err != nil {
		return nil, err
	}

	jobParameters := &jobModels.JobParameters{
		DeploymentName:  deploymentName,
		FromEnvironment: fromEnvironment,
		ToEnvironment:   toEnvironment,
	}

	pipeline, err := jobPipeline.GetPipelineFromName("promote")
	if err != nil {
		return nil, err
	}

	jobSummary, err := ah.jobHandler.HandleStartPipelineJob(appName, crdUtils.GetGithubCloneURLFromRepo(app.Registration.Repository), pipeline, jobParameters)
	if err != nil {
		return nil, err
	}

	return jobSummary, nil
}

func (ah ApplicationHandler) isValidRegistration(radixRegistration *v1.RadixRegistration) error {
	// Need to use in-cluster client of the API server, because the user might not have enough priviledges
	// to run a full validation
	_, err := radixvalidators.CanRadixRegistrationBeInserted(ah.serviceAccount.RadixClient, radixRegistration)
	if err != nil {
		return err
	}

	return nil
}

func (ah ApplicationHandler) isValidUpdate(radixRegistration *v1.RadixRegistration) error {
	// Need to use in-cluster client of the API server, because the user might not have enough priviledges
	// to run a full validation
	_, err := radixvalidators.CanRadixRegistrationBeUpdated(ah.serviceAccount.RadixClient, radixRegistration)
	if err != nil {
		return err
	}

	return err
}

func (ah ApplicationHandler) getAppAlias(appName string, environments []*environmentModels.EnvironmentSummary) (*applicationModels.ApplicationAlias, error) {
	for _, environment := range environments {
		environmentNamespace := k8sObjectUtils.GetEnvironmentNamespace(appName, environment.Name)

		ingresses, err := ah.userAccount.Client.ExtensionsV1beta1().Ingresses(environmentNamespace).List(metav1.ListOptions{
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

// Builder Handles construction of DTO
type Builder interface {
	withName(name string) Builder
	withRepository(string) Builder
	withSharedSecret(string) Builder
	withAdGroups([]string) Builder
	withPublicKey(string) Builder
	withPrivateKey(string) Builder
	withCloneURL(string) Builder
	withDeployKey(*utils.DeployKey) Builder
	withAppRegistration(appRegistration *applicationModels.ApplicationRegistration) Builder
	withRadixRegistration(*v1.RadixRegistration) Builder
	Build() applicationModels.ApplicationRegistration
	BuildRR() (*v1.RadixRegistration, error)
}

type applicationBuilder struct {
	name         string
	repository   string
	sharedSecret string
	adGroups     []string
	publicKey    string
	privateKey   string
	cloneURL     string
}

func (rb *applicationBuilder) withAppRegistration(appRegistration *applicationModels.ApplicationRegistration) Builder {
	rb.withName(appRegistration.Name)
	rb.withRepository(appRegistration.Repository)
	rb.withSharedSecret(appRegistration.SharedSecret)
	rb.withAdGroups(appRegistration.AdGroups)
	rb.withPublicKey(appRegistration.PublicKey)
	rb.withPrivateKey(appRegistration.PrivateKey)
	return rb
}

func (rb *applicationBuilder) withRadixRegistration(radixRegistration *v1.RadixRegistration) Builder {
	rb.withName(radixRegistration.Name)
	rb.withCloneURL(radixRegistration.Spec.CloneURL)
	rb.withSharedSecret(radixRegistration.Spec.SharedSecret)
	rb.withAdGroups(radixRegistration.Spec.AdGroups)
	rb.withPublicKey(radixRegistration.Spec.DeployKeyPublic)

	// Private part of key should never be returned
	return rb
}

func (rb *applicationBuilder) withName(name string) Builder {
	rb.name = name
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
	}
}
