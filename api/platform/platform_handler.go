package platform

import (
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-api/api/job"
	"github.com/statoil/radix-api/api/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
)

// HandleGetRegistations handler for GetRegistations
func HandleGetRegistations(radixclient radixclient.Interface, sshRepo string) ([]*ApplicationRegistration, error) {
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
		radixRegistations = append(radixRegistations, builder.withName(rr.Name).withRepository(rr.Spec.Repository).withSharedSecret(rr.Spec.SharedSecret).withAdGroups(rr.Spec.AdGroups).BuildRegistration())
	}

	return radixRegistations, nil
}

// HandleGetRegistation handler for GetRegistation
func HandleGetRegistation(radixclient radixclient.Interface, appName string) (*ApplicationRegistration, error) {
	radixRegistation, err := radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Get(appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	builder := NewBuilder()
	return builder.withName(radixRegistation.Name).withRepository(radixRegistation.Spec.Repository).withSharedSecret(radixRegistation.Spec.SharedSecret).withAdGroups(radixRegistation.Spec.AdGroups).BuildRegistration(), nil
}

// HandleCreateRegistation handler for CreateRegistation
func HandleCreateRegistation(radixclient radixclient.Interface, registration ApplicationRegistration) (*ApplicationRegistration, error) {
	radixRegistration, err := buildRadixRegistration(&registration)
	if err != nil {
		return nil, err
	}

	_, err = radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Create(radixRegistration)
	if err != nil {
		return nil, err
	}

	return &registration, nil
}

// HandleUpdateRegistation handler for UpdateRegistation
func HandleUpdateRegistation(radixclient radixclient.Interface, appName string, registration ApplicationRegistration) (*ApplicationRegistration, error) {
	if appName != registration.Name {
		return nil, utils.ValidationError("Radix Registration", fmt.Sprintf("App name %s does not correspond with registration name %s", appName, registration.Name))
	}

	// Make check that this is an existing registration
	existingRegistration, err := radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Get(appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	radixRegistration, err := buildRadixRegistration(&registration)
	if err != nil {
		return nil, err
	}

	// Only these fields can change over time
	existingRegistration.Spec.CloneURL = radixRegistration.Spec.CloneURL
	existingRegistration.Spec.SharedSecret = radixRegistration.Spec.SharedSecret
	existingRegistration.Spec.DeployKey = radixRegistration.Spec.DeployKey
	existingRegistration.Spec.AdGroups = radixRegistration.Spec.AdGroups

	_, err = radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Update(existingRegistration)
	if err != nil {
		return nil, err
	}

	return &registration, nil
}

// HandleDeleteRegistation handler for DeleteRegistation
func HandleDeleteRegistation(radixclient radixclient.Interface, appName string) error {
	log.Infof("Deleting app with name %s", appName)
	return nil
}

// HandleCreateApplicationPipelineJob handler for CreateApplicationPipelineJob
func HandleCreateApplicationPipelineJob(client kubernetes.Interface, radixclient radixclient.Interface, appName, branch string) (*job.PipelineJob, error) {
	log.Infof("Creating pipeline job for %s", appName)
	registration, err := HandleGetRegistation(radixclient, appName)
	if err != nil {
		return nil, err
	}

	builder := NewBuilder()
	radixRegistration, err := builder.withName(registration.Name).withRepository(registration.Repository).withSharedSecret(registration.SharedSecret).withAdGroups(registration.AdGroups).BuildRR()
	if err != nil {
		return nil, err
	}

	pipelineJobSpec := &job.PipelineJob{
		AppName: radixRegistration.GetName(),
		Branch:  branch,
		SSHRepo: radixRegistration.Spec.CloneURL,
	}

	err = job.HandleCreatePipelineJob(client, pipelineJobSpec)
	if err != nil {
		return nil, err
	}

	return pipelineJobSpec, nil
}

func buildRadixRegistration(registration *ApplicationRegistration) (*v1.RadixRegistration, error) {
	builder := NewBuilder()

	// Only if repository is provided and deploykey is not set by user
	// generate the key
	if strings.TrimSpace(registration.Repository) != "" &&
		strings.TrimSpace(registration.PublicKey) == "" {
		deployKey, err := utils.GenerateDeployKey()
		if err != nil {
			return nil, err
		}

		registration.PublicKey = deployKey.PublicKey
		builder.withPrivateKey(deployKey.PrivateKey)
	}

	radixRegistration, err := builder.withName(registration.Name).withRepository(registration.Repository).withSharedSecret(registration.SharedSecret).withAdGroups(registration.AdGroups).BuildRR()
	if err != nil {
		return nil, err
	}

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
	BuildRR() (*v1.RadixRegistration, error)
	BuildRegistration() *ApplicationRegistration
}

type registrationBuilder struct {
	name         string
	repository   string
	sharedSecret string
	adGroups     []string
	publicKey    string
	privateKey   string
}

func (rb *registrationBuilder) withName(name string) RegistrationBuilder {
	rb.name = name
	return rb
}

func (rb *registrationBuilder) withRepository(repository string) RegistrationBuilder {
	rb.repository = repository
	return rb
}

func (rb *registrationBuilder) withSharedSecret(sharedSecret string) RegistrationBuilder {
	rb.sharedSecret = sharedSecret
	return rb
}

func (rb *registrationBuilder) withAdGroups(adGroups []string) RegistrationBuilder {
	rb.adGroups = adGroups
	return rb
}

func (rb *registrationBuilder) withPublicKey(publicKey string) RegistrationBuilder {
	rb.publicKey = publicKey
	return rb
}

func (rb *registrationBuilder) withPrivateKey(privateKey string) RegistrationBuilder {
	rb.privateKey = privateKey
	return rb
}

func (rb *registrationBuilder) BuildRR() (*v1.RadixRegistration, error) {
	cloneURL, err := getCloneURLFromRepo(rb.repository)
	if err != nil {
		return nil, err
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
			Repository:   rb.repository,
			CloneURL:     cloneURL,
			SharedSecret: rb.sharedSecret,
			DeployKey:    rb.privateKey,
			AdGroups:     rb.adGroups,
		},
	}
	return radixRegistration, nil
}

func (rb *registrationBuilder) BuildRegistration() *ApplicationRegistration {
	return &ApplicationRegistration{
		Name:         rb.name,
		Repository:   rb.repository,
		SharedSecret: rb.sharedSecret,
		AdGroups:     rb.adGroups,
		PublicKey:    rb.publicKey,
	}
}

// NewBuilder Constructor for registration builder
func NewBuilder() RegistrationBuilder {
	return &registrationBuilder{}
}

func filterOnSSHRepo(rr *v1.RadixRegistration, sshURL string) bool {
	filter := true

	if strings.TrimSpace(sshURL) == "" ||
		strings.EqualFold(rr.Spec.CloneURL, sshURL) {
		filter = false
	}

	return filter
}

func getCloneURLFromRepo(repo string) (string, error) {
	cloneURL := repoPattern.ReplaceAllString(repo, sshURL)
	cloneURL += ".git"
	return cloneURL, nil
}
