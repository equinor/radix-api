package platform

import (
	"github.com/Sirupsen/logrus"
	"github.com/statoil/radix-api/api/job"
	"github.com/statoil/radix-api/api/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
)

// HandleGetRegistations handler for GetRegistations
func HandleGetRegistations(radixclient radixclient.Interface) ([]ApplicationRegistration, error) {
	radixRegistationList, err := radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	radixRegistations := make([]ApplicationRegistration, len(radixRegistationList.Items))
	for i, rr := range radixRegistationList.Items {
		builder := NewBuilder()
		radixRegistations[i] = builder.withName(rr.Name).withRepository(rr.Spec.Repository).withSharedSecret(rr.Spec.SharedSecret).withAdGroups(rr.Spec.AdGroups).BuildRegistration()
	}

	return radixRegistations, nil
}

// HandleGetRegistation handler for GetRegistation
func HandleGetRegistation(radixclient radixclient.Interface, appName string) (ApplicationRegistration, error) {
	radixRegistation, err := radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Get(appName, metav1.GetOptions{})
	if err != nil {
		return ApplicationRegistration{}, err
	}

	builder := NewBuilder()
	return builder.withRepository(radixRegistation.Spec.Repository).withSharedSecret(radixRegistation.Spec.SharedSecret).withAdGroups(radixRegistation.Spec.AdGroups).BuildRegistration(), nil
}

// HandleCreateRegistation handler for CreateRegistation
func HandleCreateRegistation(radixclient radixclient.Interface, registration ApplicationRegistration) (*ApplicationRegistration, error) {
	err := validate(registration)
	if err != nil {
		return nil, err
	}

	deployKey, err := utils.GenerateDeployKey()
	if err != nil {
		return nil, err
	}

	builder := NewBuilder()
	radixRegistration, err := builder.withName(registration.Name).withRepository(registration.Repository).withSharedSecret(registration.SharedSecret).withAdGroups(registration.AdGroups).withPrivateKey(deployKey.PrivateKey).BuildRR()
	if err != nil {
		return nil, err
	}

	_, err = radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Create(radixRegistration)
	if err != nil {
		return nil, err
	}

	registration.PublicKey = deployKey.PublicKey
	return &registration, nil
}

// HandleDeleteRegistation handler for DeleteRegistation
func HandleDeleteRegistation(radixclient radixclient.Interface, appName string) error {
	logrus.Infof("Deleting app with name %s", appName)
	return nil
}

// HandleCreateApplicationPipelineJob handler for CreateApplicationPipelineJob
func HandleCreateApplicationPipelineJob(client kubernetes.Interface, radixclient radixclient.Interface, appName, branch string) error {
	logrus.Infof("Creating pipeline job for %s", appName)
	registration, err := HandleGetRegistation(radixclient, appName)
	if err != nil {
		return err
	}

	builder := NewBuilder()
	radixRegistration, err := builder.withRepository(registration.Repository).withSharedSecret(registration.SharedSecret).withAdGroups(registration.AdGroups).BuildRR()
	if err != nil {
		return err
	}

	pipelineJobSpec := &job.PipelineJob{
		AppName: radixRegistration.GetName(),
		Branch:  branch,
		SSHRepo: radixRegistration.Spec.CloneURL,
	}

	job.HandleCreatePipelineJob(client, pipelineJobSpec)

	return nil
}

func validate(registration ApplicationRegistration) error {
	if registration.Name == "" {
		return utils.ValidationError("Radix Registration", "Name is required")
	}

	if registration.Repository == "" {
		return utils.ValidationError("Radix Registration", "Repository is required")
	}

	b := repoPattern.MatchString(registration.Repository)
	if !b {
		return utils.ValidationError("Radix Registration", "Repo string does not match the expected pattern")
	}

	return nil
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
	BuildRegistration() ApplicationRegistration
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

func (rb *registrationBuilder) BuildRegistration() ApplicationRegistration {
	return ApplicationRegistration{
		Name:         rb.name,
		Repository:   rb.repository,
		SharedSecret: rb.sharedSecret,
		AdGroups:     rb.adGroups,
		PublicKey:    "",
	}
}

// NewBuilder Constructor for registration builder
func NewBuilder() RegistrationBuilder {
	return &registrationBuilder{}
}

func getCloneURLFromRepo(repo string) (string, error) {
	cloneURL := repoPattern.ReplaceAllString(repo, sshURL)
	cloneURL += ".git"
	return cloneURL, nil
}
