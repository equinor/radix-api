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
		radixRegistations = append(radixRegistations, builder.withRadixRegistration(&rr).BuildApplicationRegistration())
	}

	return radixRegistations, nil
}

// HandleGetApplication handler for GetApplication
func HandleGetApplication(radixclient radixclient.Interface, appName string) (*ApplicationRegistration, error) {
	radixRegistation, err := radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Get(appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	builder := NewBuilder()
	return builder.withRadixRegistration(radixRegistation).BuildApplicationRegistration(), nil
}

// HandleRegisterApplication handler for RegisterApplication
func HandleRegisterApplication(radixclient radixclient.Interface, application ApplicationRegistration) (*ApplicationRegistration, error) {
	radixRegistration, err := buildRadixRegistration(&application)
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

	radixRegistration, err := buildRadixRegistration(&application)
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

	radixRegistration := NewBuilder().withAppRegistration(application).BuildRR()

	pipelineJobSpec := &job.PipelineJob{
		Branch:  branch,
		SSHRepo: radixRegistration.Spec.CloneURL,
	}

	err = job.HandleStartPipelineJob(client, appName, pipelineJobSpec)
	if err != nil {
		return nil, err
	}

	return pipelineJobSpec, nil
}

func buildRadixRegistration(application *ApplicationRegistration) (*v1.RadixRegistration, error) {
	builder := NewBuilder()

	// Only if repository is provided and deploykey is not set by user
	// generate the key
	if strings.TrimSpace(application.Repository) != "" &&
		strings.TrimSpace(application.PublicKey) == "" {
		deployKey, err := utils.GenerateDeployKey()
		if err != nil {
			return nil, err
		}

		application.PublicKey = deployKey.PublicKey
		builder.withPrivateKey(deployKey.PrivateKey)
	}

	radixRegistration := builder.withPublicKey(application.PublicKey).withName(application.Name).withRepository(application.Repository).withSharedSecret(application.SharedSecret).withAdGroups(application.AdGroups).BuildRR()
	return radixRegistration, nil
}

// RegistrationBuilder Handles construction of RR or applicationRegistation
type RegistrationBuilder interface {
	withName(name string) RegistrationBuilder
	withRepository(string) RegistrationBuilder
	withSharedSecret(string) RegistrationBuilder
	withAdGroups([]string) RegistrationBuilder
	withPublicKey(string) RegistrationBuilder
	withPrivateKey(string) RegistrationBuilder
	withCloneURL(string) RegistrationBuilder
	withRadixRegistration(*v1.RadixRegistration) RegistrationBuilder
	withAppRegistration(*ApplicationRegistration) RegistrationBuilder
	BuildRR() *v1.RadixRegistration
	BuildApplicationRegistration() *ApplicationRegistration
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

func (rb *applicationBuilder) withAppRegistration(appRegistration *ApplicationRegistration) RegistrationBuilder {
	rb.withName(appRegistration.Name)
	rb.withRepository(appRegistration.Repository)
	rb.withSharedSecret(appRegistration.SharedSecret)
	rb.withAdGroups(appRegistration.AdGroups)
	rb.withPublicKey(appRegistration.PublicKey)
	return rb
}

func (rb *applicationBuilder) withRadixRegistration(radixRegistration *v1.RadixRegistration) RegistrationBuilder {
	rb.withName(radixRegistration.Name)
	rb.withCloneURL(radixRegistration.Spec.CloneURL)
	rb.withSharedSecret(radixRegistration.Spec.SharedSecret)
	rb.withAdGroups(radixRegistration.Spec.AdGroups)
	rb.withPublicKey(radixRegistration.Spec.DeployKeyPublic)
	rb.withPrivateKey(radixRegistration.Spec.DeployKey)
	return rb
}

func (rb *applicationBuilder) withName(name string) RegistrationBuilder {
	rb.name = name
	return rb
}

func (rb *applicationBuilder) withRepository(repository string) RegistrationBuilder {
	rb.repository = repository
	return rb
}

func (rb *applicationBuilder) withCloneURL(cloneURL string) RegistrationBuilder {
	rb.cloneURL = cloneURL
	return rb
}

func (rb *applicationBuilder) withSharedSecret(sharedSecret string) RegistrationBuilder {
	rb.sharedSecret = sharedSecret
	return rb
}

func (rb *applicationBuilder) withAdGroups(adGroups []string) RegistrationBuilder {
	rb.adGroups = adGroups
	return rb
}

func (rb *applicationBuilder) withPublicKey(publicKey string) RegistrationBuilder {
	rb.publicKey = strings.TrimSuffix(publicKey, "\n")
	return rb
}

func (rb *applicationBuilder) withPrivateKey(privateKey string) RegistrationBuilder {
	rb.privateKey = privateKey
	return rb
}

func (rb *applicationBuilder) BuildRR() *v1.RadixRegistration {
	cloneURL := rb.cloneURL
	if cloneURL == "" {
		cloneURL = getCloneURLFromRepo(rb.repository)
	}

	radixRegistration := &v1.RadixRegistration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "radix.equinor.com/v1",
			Kind:       "RadixRegistration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: rb.name,
		},
		Spec: v1.RadixRegistrationSpec{
			CloneURL:        cloneURL,
			SharedSecret:    rb.sharedSecret,
			DeployKey:       rb.privateKey,
			DeployKeyPublic: rb.publicKey,
			AdGroups:        rb.adGroups,
		},
	}
	return radixRegistration
}

func (rb *applicationBuilder) BuildApplicationRegistration() *ApplicationRegistration {
	repository := rb.repository
	if repository == "" {
		repository = getRepositoryURLFromCloneURL(rb.cloneURL)
	}

	return &ApplicationRegistration{
		Name:         rb.name,
		Repository:   repository,
		SharedSecret: rb.sharedSecret,
		AdGroups:     rb.adGroups,
		PublicKey:    rb.publicKey,
	}
}

// NewBuilder Constructor for application builder
func NewBuilder() RegistrationBuilder {
	return &applicationBuilder{}
}

func filterOnSSHRepo(rr *v1.RadixRegistration, sshURL string) bool {
	filter := true

	if strings.TrimSpace(sshURL) == "" ||
		strings.EqualFold(rr.Spec.CloneURL, sshURL) {
		filter = false
	}

	return filter
}

func getCloneURLFromRepo(repo string) string {
	if repo == "" {
		return ""
	}

	cloneURL := repoPattern.ReplaceAllString(repo, sshURL)
	cloneURL += ".git"
	return cloneURL
}

func getRepositoryURLFromCloneURL(cloneURL string) string {
	if cloneURL == "" {
		return ""
	}

	repoName := strings.TrimSuffix(strings.TrimPrefix(cloneURL, sshURL), ".git")
	repo := fmt.Sprintf("%s%s", repoURL, repoName)
	return repo
}
