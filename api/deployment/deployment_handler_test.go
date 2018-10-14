package deployment

import (
	"testing"

	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	radix "github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

func TestGetDeployments_Filter_FilterIsApplied(t *testing.T) {
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	save(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp1").withEnvironment("prod").withImageTag("abcdef"))
	save(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp2").withEnvironment("dev").withImageTag("ghijklm"))
	save(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp2").withEnvironment("prod").withImageTag("nopqlmn"))

	deployments, _ := HandleGetDeployments(radixclient, "", "", false)
	assert.Equal(t, 3, len(deployments), "GetDeployments - expected to be listed")

	deployments, _ = HandleGetDeployments(radixclient, "anyapp2", "", false)
	assert.Equal(t, 2, len(deployments), "GetDeployments - expected to be listed")

	deployments, _ = HandleGetDeployments(radixclient, "anyapp2", "dev", false)
	assert.Equal(t, 1, len(deployments), "GetDeployments - expected to be listed")
}

func save(kubeclient *kubernetes.Clientset, radixclient *radix.Clientset, builder DeploymentBuilder) {
	rd := builder.BuildRD()

	ns := getNameSpaceForApplicationEnvironment(rd.Spec.AppName, rd.Spec.Environment)
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}

	kubeclient.CoreV1().Namespaces().Create(&namespace)
	radixclient.RadixV1().RadixDeployments(ns).Create(rd)
}

// TODO: Should this be a test functionality or moved into main code?
// DeploymentBuilder Handles construction of RD
type DeploymentBuilder interface {
	withImageTag(string) DeploymentBuilder
	withAppName(string) DeploymentBuilder
	withEnvironment(string) DeploymentBuilder
	BuildRD() *v1.RadixDeployment
}

type deploymentBuilder struct {
	imageTag    string
	appName     string
	environment string
}

func (db *deploymentBuilder) withImageTag(imageTag string) DeploymentBuilder {
	db.imageTag = imageTag
	return db
}

func (db *deploymentBuilder) withAppName(appName string) DeploymentBuilder {
	db.appName = appName
	return db
}

func (db *deploymentBuilder) withEnvironment(environment string) DeploymentBuilder {
	db.environment = environment
	return db
}

func (db *deploymentBuilder) BuildRD() *v1.RadixDeployment {
	radixDeployment := &v1.RadixDeployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "radix.equinor.com/v1",
			Kind:       "RadixDeployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      getDeploymentName(db.appName, db.imageTag),
			Namespace: getNameSpaceForApplicationEnvironment(db.appName, db.environment),
			Labels: map[string]string{
				"radixApp": db.appName,
				"env":      db.environment,
			},
		},
		Spec: v1.RadixDeploymentSpec{
			AppName:     db.appName,
			Environment: db.environment,
		},
	}
	return radixDeployment
}

// NewDeploymentBuilder Constructor for deployment builder
func NewDeploymentBuilder() DeploymentBuilder {
	return &deploymentBuilder{}
}
