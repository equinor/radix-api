package deployment

import (
	"testing"
	"time"

	"github.com/statoil/radix-api/api/utils"

	radix "github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

func TestGetDeployments_Filter_FilterIsApplied(t *testing.T) {
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	apply(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp1").withEnvironment("prod").withImageTag("abcdef"))

	// Ensure the second image is considered the latest version
	apply(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp2").withEnvironment("dev").withImageTag("ghijklm").withCreated(time.Now()))
	apply(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp2").withEnvironment("dev").withImageTag("nopqrst").withCreated(time.Now().AddDate(0, 0, 1)))
	apply(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp2").withEnvironment("prod").withImageTag("uvwxyza"))

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
	assert.Equal(t, "App name is required", (err.(*utils.Error)).Message, "HandlePromoteEnvironment - Unexpected message")

	_, err = HandlePromoteEnvironment(kubeclient, radixclient, "noapp", PromotionParameters{FromEnvironment: "dev", ImageTag: "abcdef", ToEnvironment: "prod"})
	assert.Error(t, err, "HandlePromoteEnvironment - Cannot promote non-existing app")

	apply(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp").withEnvironment("prod").withImageTag("abcdef"))
	_, err = HandlePromoteEnvironment(kubeclient, radixclient, "anyapp", PromotionParameters{FromEnvironment: "dev", ImageTag: "abcdef", ToEnvironment: "prod"})
	assert.Error(t, err, "HandlePromoteEnvironment - Cannot promote from non-existing environment")
	assert.Equal(t, "Non existing from environment", (err.(*utils.Error)).Message, "HandlePromoteEnvironment - Unexpected message")

	apply(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp1").withEnvironment("dev").withImageTag("abcdef"))
	_, err = HandlePromoteEnvironment(kubeclient, radixclient, "anyapp1", PromotionParameters{FromEnvironment: "dev", ImageTag: "abcdef", ToEnvironment: "prod"})
	assert.Error(t, err, "HandlePromoteEnvironment - Cannot promote to non-existing environment")
	assert.Equal(t, "Non existing to environment", (err.(*utils.Error)).Message, "HandlePromoteEnvironment - Unexpected message")

	apply(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp2").withEnvironment("dev").withImageTag("abcdef"))
	apply(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp2").withEnvironment("prod").withImageTag("ghijklm"))
	_, err = HandlePromoteEnvironment(kubeclient, radixclient, "anyapp2", PromotionParameters{FromEnvironment: "dev", ImageTag: "nopqrst", ToEnvironment: "prod"})
	assert.Error(t, err, "HandlePromoteEnvironment - Cannot promote non-existing image")
	assert.Equal(t, "Non existing image", (err.(*utils.Error)).Message, "HandlePromoteEnvironment - Unexpected message")

	apply(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp3").withEnvironment("dev").withImageTag("abcdef"))
	apply(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp3").withEnvironment("prod").withImageTag("abcdef"))
	_, err = HandlePromoteEnvironment(kubeclient, radixclient, "anyapp3", PromotionParameters{FromEnvironment: "dev", ImageTag: "abcdef", ToEnvironment: "prod"})
	assert.Error(t, err, "HandlePromoteEnvironment - Cannot promote an image into environment having already that image")

	createNamespace(kubeclient, "anyapp4", "dev")
	createNamespace(kubeclient, "anyapp4", "prod")
	_, err = HandlePromoteEnvironment(kubeclient, radixclient, "anyapp4", PromotionParameters{FromEnvironment: "dev", ToEnvironment: "prod"})
	assert.Error(t, err, "HandlePromoteEnvironment - Cannot promote non-existing image")
	assert.Equal(t, "No latest deployment was found", (err.(*utils.Error)).Message, "HandlePromoteEnvironment - Unexpected message")
}

func TestPromote_HappyPathScenarios_NewStateIsExpected(t *testing.T) {
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	apply(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp").withEnvironment("dev").withImageTag("abcdef"))

	// Create prod environment without any deployments
	createNamespace(kubeclient, "anyapp", "prod")
	_, err := HandlePromoteEnvironment(kubeclient, radixclient, "anyapp", PromotionParameters{FromEnvironment: "dev", ImageTag: "abcdef", ToEnvironment: "prod"})
	assert.NoError(t, err, "HandlePromoteEnvironment - Unexpected error")

	deployments, _ := HandleGetDeployments(radixclient, "anyapp", "prod", false)
	assert.Equal(t, 1, len(deployments), "HandlePromoteEnvironment - Promoted as expected")

	// Ensure the second image is considered the latest version
	apply(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp1").withEnvironment("dev").withImageTag("ghijklm").withCreated(time.Now()))
	apply(kubeclient, radixclient, NewDeploymentBuilder().withAppName("anyapp1").withEnvironment("dev").withImageTag("nopqrst").withCreated(time.Now().AddDate(0, 0, 1)))

	createNamespace(kubeclient, "anyapp1", "prod")

	// No image tag should promote latest
	_, err = HandlePromoteEnvironment(kubeclient, radixclient, "anyapp1", PromotionParameters{FromEnvironment: "dev", ToEnvironment: "prod"})
	assert.NoError(t, err, "HandlePromoteEnvironment - Unexpected error")

	deployments, _ = HandleGetDeployments(radixclient, "anyapp1", "prod", false)
	assert.Equal(t, 1, len(deployments), "HandlePromoteEnvironment - Promoted as expected")
	assert.Equal(t, getDeploymentName("anyapp1", "nopqrst"), deployments[0].Name, "HandlePromoteEnvironment - Unexpected image promoted")
}

func apply(kubeclient *kubernetes.Clientset, radixclient *radix.Clientset, builder Builder) {
	rd := builder.BuildRD()

	ns := createNamespace(kubeclient, rd.Spec.AppName, rd.Spec.Environment)
	radixclient.RadixV1().RadixDeployments(ns).Create(rd)
}

func createNamespace(kubeclient *kubernetes.Clientset, appName, environment string) string {
	ns := getNamespaceForApplicationEnvironment(appName, environment)
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}

	kubeclient.CoreV1().Namespaces().Create(&namespace)
	return ns
}
