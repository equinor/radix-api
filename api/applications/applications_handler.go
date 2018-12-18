package applications

import (
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	applicationModels "github.com/statoil/radix-api/api/applications/models"
	"github.com/statoil/radix-api/api/environments"
	job "github.com/statoil/radix-api/api/jobs"
	jobModels "github.com/statoil/radix-api/api/jobs/models"
	"github.com/statoil/radix-api/api/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/statoil/radix-operator/pkg/apis/application"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"github.com/statoil/radix-operator/pkg/apis/radixvalidators"
	crdUtils "github.com/statoil/radix-operator/pkg/apis/utils"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
)

// ApplicationHandler Instance variables
type ApplicationHandler struct {
	client      kubernetes.Interface
	radixclient radixclient.Interface
	jobHandler  job.JobHandler
}

// Init Constructor
func Init(client kubernetes.Interface, radixclient radixclient.Interface) ApplicationHandler {
	jobHandler := job.Init(client, radixclient)
	return ApplicationHandler{client, radixclient, jobHandler}
}

// GetApplications handler for ShowApplications
func (ah ApplicationHandler) GetApplications(sshRepo string) ([]*applicationModels.ApplicationSummary, error) {
	radixRegistationList, err := ah.radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	applicationJobs, err := ah.jobHandler.GetLatestJobPerApplication()
	if err != nil {
		return nil, err
	}

	applications := make([]*applicationModels.ApplicationSummary, 0)
	for _, rr := range radixRegistationList.Items {
		if filterOnSSHRepo(&rr, sshRepo) {
			continue
		}

		jobSummary := applicationJobs[rr.Name]
		applications = append(applications, &applicationModels.ApplicationSummary{Name: rr.Name, LatestJob: jobSummary})
	}

	return applications, nil
}

// GetApplication handler for GetApplication
func (ah ApplicationHandler) GetApplication(appName string) (*applicationModels.Application, error) {
	radixRegistration, err := ah.radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Get(appName, metav1.GetOptions{})
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

	environmentHandler := environments.Init(ah.client, ah.radixclient)
	environments, err := environmentHandler.GetEnvironmentSummary(appName)
	if err != nil {
		return nil, err
	}

	return &applicationModels.Application{Name: applicationRegistration.Name, Registration: applicationRegistration, Jobs: jobs, Environments: environments}, nil
}

// RegisterApplication handler for RegisterApplication
func (ah ApplicationHandler) RegisterApplication(application applicationModels.ApplicationRegistration) (*applicationModels.ApplicationRegistration, error) {
	// Only if repository is provided and deploykey is not set by user
	// generate the key
	var deployKey *utils.DeployKey
	var err error

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

	err = isValidRegistration(radixRegistration)
	if err != nil {
		return nil, err
	}

	_, err = ah.radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Create(radixRegistration)
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
	existingRegistration, err := ah.radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Get(appName, metav1.GetOptions{})
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

	err = isValidUpdate(existingRegistration)
	if err != nil {
		return nil, err
	}

	_, err = ah.radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Update(existingRegistration)
	if err != nil {
		return nil, err
	}

	return &application, nil
}

// DeleteApplication handler for DeleteApplication
func (ah ApplicationHandler) DeleteApplication(appName string) error {
	// Make check that this is an existing application
	_, err := ah.radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Get(appName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	err = ah.radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Delete(appName, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return nil
}

// TriggerPipeline handler for TriggerPipeline
func (ah ApplicationHandler) TriggerPipeline(appName, pipelineName string, pipelineParameters applicationModels.PipelineParameters) (*jobModels.JobSummary, error) {
	pipeline, err := jobModels.GetPipelineFromName(pipelineName)
	if err != nil {
		return nil, utils.ValidationError("Radix Application Pipeline", fmt.Sprintf("Pipeline %s not supported", pipelineName))
	}

	branch := pipelineParameters.Branch
	commitID := pipelineParameters.CommitID

	if strings.TrimSpace(appName) == "" || strings.TrimSpace(branch) == "" {
		return nil, applicationModels.AppNameAndBranchAreRequiredForStartingPipeline()
	}

	log.Infof("Creating pipeline job for %s on branch %s for commit %s", appName, branch, commitID)
	app, err := ah.GetApplication(appName)
	if err != nil {
		return nil, err
	}

	// Check if branch is mapped
	if !application.IsMagicBranch(branch) {
		config, err := ah.radixclient.RadixV1().RadixApplications(crdUtils.GetAppNamespace(appName)).Get(appName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		application := application.NewApplication(config)
		branchIsMapped, _ := application.IsBranchMappedToEnvironment(branch)

		if !branchIsMapped {
			return nil, applicationModels.UnmatchedBranchToEnvironment(branch)
		}
	}

	jobParameters := &jobModels.JobParameters{
		Branch:   branch,
		CommitID: commitID,
	}

	jobSummary, err := ah.jobHandler.HandleStartPipelineJob(appName, crdUtils.GetGithubCloneURLFromRepo(app.Registration.Repository), pipeline, jobParameters)
	if err != nil {
		return nil, err
	}

	return jobSummary, nil
}

func isValidRegistration(radixRegistration *v1.RadixRegistration) error {
	// Need to use in-cluster client of the API server, because the user might not have enough priviledges
	// to run a full validation
	kubeUtil := utils.NewKubeUtil(false)
	_, inClusterRadixClient := kubeUtil.GetInClusterKubernetesClient()

	_, err := radixvalidators.CanRadixRegistrationBeInserted(inClusterRadixClient, radixRegistration)
	if err != nil {
		return err
	}

	return nil
}

func isValidUpdate(radixRegistration *v1.RadixRegistration) error {
	// Need to use in-cluster client of the API server, because the user might not have enough priviledges
	// to run a full validation
	kubeUtil := utils.NewKubeUtil(false)
	_, inClusterRadixClient := kubeUtil.GetInClusterKubernetesClient()

	_, err := radixvalidators.CanRadixRegistrationBeUpdated(inClusterRadixClient, radixRegistration)
	if err != nil {
		return err
	}

	return err
}

// Builder Handles construction of DTO
type Builder interface {
	withName(name string) Builder
	withRepository(string) Builder
	withSharedSecret(string) Builder
	withAdGroups([]string) Builder
	withPublicKey(string) Builder
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
	return rb
}

func (rb *applicationBuilder) withRadixRegistration(radixRegistration *v1.RadixRegistration) Builder {
	rb.withName(radixRegistration.Name)
	rb.withCloneURL(radixRegistration.Spec.CloneURL)
	rb.withSharedSecret(radixRegistration.Spec.SharedSecret)
	rb.withAdGroups(radixRegistration.Spec.AdGroups)
	rb.withPublicKey(radixRegistration.Spec.DeployKeyPublic)
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

func filterOnSSHRepo(rr *v1.RadixRegistration, sshURL string) bool {
	filter := true

	if strings.TrimSpace(sshURL) == "" ||
		strings.EqualFold(rr.Spec.CloneURL, sshURL) {
		filter = false
	}

	return filter
}
