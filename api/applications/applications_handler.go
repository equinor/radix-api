package applications

import (
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	ac "github.com/statoil/radix-api/api/admissioncontrollers"
	job "github.com/statoil/radix-api/api/jobs"
	"github.com/statoil/radix-api/api/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/statoil/radix-operator/pkg/apis/utils"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
)

// Pipeline Enumeration of the different pipelines we support
type Pipeline int

const (
	// BuildDeploy Will do build based on docker file and deploy to mapped environment
	BuildDeploy Pipeline = iota

	// end marker of the enum
	numPipelines
)

func (p Pipeline) String() string {
	return [...]string{"build-deploy"}[p]
}

func getPipeline(name string) (Pipeline, error) {
	for pipeline := BuildDeploy; pipeline < numPipelines; pipeline++ {
		if pipeline.String() == name {
			return pipeline, nil
		}
	}

	return numPipelines, fmt.Errorf("No pipeline found by name %s", name)
}

// HandleGetApplications handler for ShowApplications
func HandleGetApplications(radixclient radixclient.Interface, sshRepo string) ([]*ApplicationRegistration, error) {
	radixRegistationList, err := radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	radixRegistations := make([]*ApplicationRegistration, 0)
	for _, rr := range radixRegistationList.Items {
		if filterOnSSHRepo(&rr, sshRepo) {
			continue
		}

		builder := NewBuilder()
		radixRegistations = append(radixRegistations, builder.
			withRadixRegistration(&rr).
			BuildApplicationRegistration())
	}

	return radixRegistations, nil
}

// HandleGetApplication handler for GetApplication
func HandleGetApplication(radixclient radixclient.Interface, appName string) (*ApplicationRegistration, error) {
	radixRegistration, err := radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Get(appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	builder := NewBuilder()
	return builder.
		withRadixRegistration(radixRegistration).
		BuildApplicationRegistration(), nil
}

// HandleRegisterApplication handler for RegisterApplication
func HandleRegisterApplication(radixclient radixclient.Interface, application ApplicationRegistration) (*ApplicationRegistration, error) {
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

	_, err = ac.CanRadixRegistrationBeInserted(radixclient, radixRegistration)
	if err != nil {
		return nil, err
	}

	_, err = radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Create(radixRegistration)
	if err != nil {
		return nil, err
	}

	return &application, nil
}

// HandleChangeRegistrationDetails handler for ChangeRegistrationDetails
func HandleChangeRegistrationDetails(radixclient radixclient.Interface, appName string, application ApplicationRegistration) (*ApplicationRegistration, error) {
	if appName != application.Name {
		return nil, utils.ValidationError("Radix Registration", fmt.Sprintf("App name %s does not correspond with application name %s", appName, application.Name))
	}

	// Make check that this is an existing application
	existingRegistration, err := radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Get(appName, metav1.GetOptions{})
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

	_, err = ac.CanRadixRegistrationBeUpdated(radixclient, radixRegistration)
	if err != nil {
		return nil, err
	}

	_, err = radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Update(existingRegistration)
	if err != nil {
		return nil, err
	}

	return &application, nil
}

// HandleDeleteApplication handler for DeleteApplication
func HandleDeleteApplication(radixclient radixclient.Interface, appName string) error {
	log.Infof("Deleting app with name %s", appName)
	return nil
}

// HandleTriggerPipeline handler for TriggerPipeline
func HandleTriggerPipeline(client kubernetes.Interface, radixclient radixclient.Interface, appName, pipelineName string, pipelineParameters PipelineParameters) (*job.PipelineJob, error) {
	_, err := getPipeline(pipelineName)
	if err != nil {
		return nil, utils.ValidationError("Radix Registration Pipeline", fmt.Sprintf("Pipeline %s not supported", pipelineName))
	}

	branch := pipelineParameters.Branch

	if strings.TrimSpace(appName) == "" || strings.TrimSpace(branch) == "" {
		return nil, utils.ValidationError("Radix Registration Pipeline", "App name and branch is required")
	}

	log.Infof("Creating pipeline job for %s", appName)
	application, err := HandleGetApplication(radixclient, appName)
	if err != nil {
		return nil, err
	}

	radixRegistration := crdUtils.NewRegistrationBuilder().
		WithName(application.Name).
		WithRepository(application.Repository).
		WithSharedSecret(application.SharedSecret).
		WithAdGroups(application.AdGroups).
		WithPublicKey(application.PublicKey).
		BuildRR()

	pipelineJobSpec := &job.PipelineJob{
		Branch:  branch,
		SSHRepo: radixRegistration.Spec.CloneURL,
		Commit:  pipelineParameters.Commit,
	}

	err = job.HandleStartPipelineJob(client, appName, pipelineJobSpec)
	if err != nil {
		return nil, err
	}

	return pipelineJobSpec, nil
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
	withAppRegistration(appRegistration *ApplicationRegistration) Builder
	withRadixRegistration(*v1.RadixRegistration) Builder
	BuildApplicationRegistration() *ApplicationRegistration
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

func (rb *applicationBuilder) withAppRegistration(appRegistration *ApplicationRegistration) Builder {
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

func (rb *applicationBuilder) BuildApplicationRegistration() *ApplicationRegistration {
	repository := rb.repository
	if repository == "" {
		repository = crdUtils.GetGithubRepositoryURLFromCloneURL(rb.cloneURL)
	}

	return &ApplicationRegistration{
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

// ABuilder Constructor for application builder with test values
func ABuilder() Builder {
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
