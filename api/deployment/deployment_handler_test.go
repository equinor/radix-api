package deployment

import (
	"testing"
	"time"

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

	// Ensure the second image is considered the latest version
	save(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp2").withEnvironment("dev").withImageTag("ghijklm").withTime(time.Now()))
	save(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp2").withEnvironment("dev").withImageTag("nopqrst").withTime(time.Now().AddDate(0, 0, 1)))
	save(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp2").withEnvironment("prod").withImageTag("uvwxyza"))

	deployments, _ := HandleGetDeployments(radixclient, "", "", false)
	assert.Equal(t, 4, len(deployments), "GetDeployments - no filter should list all")

	deployments, _ = HandleGetDeployments(radixclient, "anyapp2", "", false)
	assert.Equal(t, 3, len(deployments), "GetDeployments - list all accross all environments")

	deployments, _ = HandleGetDeployments(radixclient, "anyapp2", "dev", false)
	assert.Equal(t, 2, len(deployments), "GetDeployments - list all for environment")

	deployments, _ = HandleGetDeployments(radixclient, "anyapp2", "dev", true)
	assert.Equal(t, 1, len(deployments), "GetDeployments - only list latest in environment")

	deployments, _ = HandleGetDeployments(radixclient, "", "", true)
	assert.Equal(t, 3, len(deployments), "GetDeployments - only list latest for all apps in all environments")

	// TODO : Should these cases lead to errors?
	deployments, _ = HandleGetDeployments(radixclient, "anyapp3", "", true)
	assert.Equal(t, 0, len(deployments), "GetDeployments - non existing app should lead to empty list")

	deployments, _ = HandleGetDeployments(radixclient, "anyapp2", "qa", true)
	assert.Equal(t, 0, len(deployments), "GetDeployments - non existing environment should lead to empty list")
}

func TestPromote_ErrorScenarios_ErrorIsReturned(t *testing.T) {
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	_, err := HandlePromoteEnvironment(kubeclient, radixclient, "", PromotionParameters{FromEnvironment: "dev", ImageTag: "abcdef", ToEnvironment: "prod"})
	assert.Error(t, err, "HandlePromoteEnvironment - Cannot promote empty app")

	_, err = HandlePromoteEnvironment(kubeclient, radixclient, "noapp", PromotionParameters{FromEnvironment: "dev", ImageTag: "abcdef", ToEnvironment: "prod"})
	assert.Error(t, err, "HandlePromoteEnvironment - Cannot promote non-existing app")

	save(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp").withEnvironment("prod").withImageTag("abcdef"))
	_, err = HandlePromoteEnvironment(kubeclient, radixclient, "anyapp", PromotionParameters{FromEnvironment: "dev", ImageTag: "abcdef", ToEnvironment: "prod"})
	assert.Error(t, err, "HandlePromoteEnvironment - Cannot promote from non-existing environment")

	save(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp1").withEnvironment("dev").withImageTag("abcdef"))
	_, err = HandlePromoteEnvironment(kubeclient, radixclient, "anyapp1", PromotionParameters{FromEnvironment: "dev", ImageTag: "abcdef", ToEnvironment: "prod"})
	assert.Error(t, err, "HandlePromoteEnvironment - Cannot promote to non-existing environment")

	save(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp2").withEnvironment("dev").withImageTag("abcdef"))
	save(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp2").withEnvironment("prod").withImageTag("ghijklm"))
	_, err = HandlePromoteEnvironment(kubeclient, radixclient, "anyapp2", PromotionParameters{FromEnvironment: "dev", ImageTag: "nopqrst", ToEnvironment: "prod"})
	assert.Error(t, err, "HandlePromoteEnvironment - Cannot promote non-existing image")

	save(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp3").withEnvironment("dev").withImageTag("abcdef"))
	save(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp3").withEnvironment("prod").withImageTag("abcdef"))
	_, err = HandlePromoteEnvironment(kubeclient, radixclient, "anyapp3", PromotionParameters{FromEnvironment: "dev", ImageTag: "abcdef", ToEnvironment: "prod"})
	assert.Error(t, err, "HandlePromoteEnvironment - Cannot promote an image into environment having already that image")

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
	withTime(time.Time) DeploymentBuilder
	BuildRD() *v1.RadixDeployment
}

type deploymentBuilder struct {
	imageTag    string
	appName     string
	environment string
	time        time.Time
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

func (db *deploymentBuilder) withTime(time time.Time) DeploymentBuilder {
	db.time = time
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
			CreationTimestamp: metav1.Time{Time: db.time},
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
	return &deploymentBuilder{
		time: time.Now(),
	}
}
