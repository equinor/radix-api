package deployments

import (
	"context"
	"fmt"
	radixutils "github.com/equinor/radix-common/utils"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	controllertest "github.com/equinor/radix-api/api/test"
	radixhttp "github.com/equinor/radix-common/net/http"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

const (
	clusterName       = "AnyClusterName"
	containerRegistry = "any.container.registry"
	anyAppName        = "any-app"
	anyCloneURL       = "git@github.com:Equinor/any-app.git"
	anyPodName        = "any-pod"
	anyDeployName     = "any-deploy"
	anyEnv            = "any-env"
)

func createGetLogEndpoint(appName, podName string) string {
	return fmt.Sprintf("/api/v1/applications/%s/deployments/any/components/any/replicas/%s/logs", appName, podName)
}

func setupTest() (*commontest.Utils, *controllertest.Utils, kubernetes.Interface, radixclient.Interface, prometheusclient.Interface) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := fake.NewSimpleClientset()
	prometheusclient := prometheusfake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient)
	commonTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)

	// controllerTestUtils is used for issuing HTTP request and processing responses
	controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, NewDeploymentController())

	return &commonTestUtils, &controllerTestUtils, kubeclient, radixclient, prometheusclient
}

func TestGetPodLog_no_radixconfig(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _ := setupTest()

	endpoint := createGetLogEndpoint(anyAppName, anyPodName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 404, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, controllertest.AppNotFoundErrorMsg(anyAppName), errorResponse.Message)

}

func TestGetPodLog_No_Pod(t *testing.T) {
	commonTestUtils, controllerTestUtils, _, _, _ := setupTest()
	endpoint := createGetLogEndpoint(anyAppName, anyPodName)

	commonTestUtils.ApplyApplication(builders.
		ARadixApplication().
		WithAppName(anyAppName))

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 404, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := deploymentModels.NonExistingPod(anyAppName, anyPodName)

	assert.Equal(t, (expectedError.(*radixhttp.Error)).Error(), errorResponse.Error())

}

func TestGetDeployments_Filter_FilterIsApplied(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _ := setupTest()

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName("any-app-1").
		WithEnvironment("prod").
		WithImageTag("abcdef").
		WithCondition(v1.DeploymentInactive))

	// Ensure the second image is considered the latest version
	firstDeploymentActiveFrom := time.Now()
	secondDeploymentActiveFrom := time.Now().AddDate(0, 0, 1)

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName("any-app-2").
		WithEnvironment("dev").
		WithImageTag("ghijklm").
		WithCreated(firstDeploymentActiveFrom).
		WithCondition(v1.DeploymentInactive).
		WithActiveFrom(firstDeploymentActiveFrom).
		WithActiveTo(secondDeploymentActiveFrom))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName("any-app-2").
		WithEnvironment("dev").
		WithImageTag("nopqrst").
		WithCondition(v1.DeploymentActive).
		WithActiveFrom(secondDeploymentActiveFrom))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName("any-app-2").
		WithEnvironment("prod").
		WithImageTag("uvwxyza"))

	// Test
	var testScenarios = []struct {
		name                   string
		appName                string
		environment            string
		latestOnly             bool
		numDeploymentsExpected int
	}{
		{"list all accross all environments", "any-app-2", "", false, 3},
		{"list all for environment", "any-app-2", "dev", false, 2},
		{"only list latest in environment", "any-app-2", "dev", true, 1},
		{"only list latest for app in all environments", "any-app-2", "", true, 2},
		{"non existing app should lead to empty list", "any-app-3", "", false, 0},
		{"non existing environment should lead to empty list", "any-app-2", "qa", false, 0},
	}

	for _, scenario := range testScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			endpoint := fmt.Sprintf("/api/v1/applications/%s/deployments", scenario.appName)
			queryParameters := ""

			if scenario.environment != "" {
				queryParameters = fmt.Sprintf("?environment=%s", scenario.environment)
			}

			if scenario.latestOnly {
				if queryParameters != "" {
					queryParameters = queryParameters + "&"
				} else {
					queryParameters = "?"
				}

				queryParameters = queryParameters + fmt.Sprintf("latest=%v", scenario.latestOnly)
			}

			responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint+queryParameters)
			response := <-responseChannel

			deployments := make([]*deploymentModels.DeploymentSummary, 0)
			controllertest.GetResponseBody(response, &deployments)
			assert.Equal(t, scenario.numDeploymentsExpected, len(deployments))
		})
	}
}

func TestGetDeployments_NoApplicationRegistered(t *testing.T) {
	_, controllerTestUtils, _, _, _ := setupTest()
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deployments", anyAppName))
	response := <-responseChannel

	assert.Equal(t, 404, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, controllertest.AppNotFoundErrorMsg(anyAppName), errorResponse.Message)
}

func TestGetDeployments_OneEnvironment_SortedWithFromTo(t *testing.T) {
	deploymentOneImage := "abcdef"
	deploymentTwoImage := "ghijkl"
	deploymentThreeImage := "mnopqr"
	layout := "2006-01-02T15:04:05.000Z"
	deploymentOneCreated, _ := time.Parse(layout, "2018-11-12T11:45:26.371Z")
	deploymentTwoCreated, _ := time.Parse(layout, "2018-11-12T12:30:14.000Z")
	deploymentThreeCreated, _ := time.Parse(layout, "2018-11-20T09:00:00.000Z")

	// Setup
	commonTestUtils, controllerTestUtils, _, _, _ := setupTest()
	setupGetDeploymentsTest(commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, []string{"dev"})

	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deployments", anyAppName))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	controllertest.GetResponseBody(response, &deployments)
	assert.Equal(t, 3, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentThreeCreated), deployments[0].ActiveFrom)
	assert.Equal(t, "", deployments[0].ActiveTo)

	assert.Equal(t, deploymentTwoImage, deployments[1].Name)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentTwoCreated), deployments[1].ActiveFrom)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentThreeCreated), deployments[1].ActiveTo)

	assert.Equal(t, deploymentOneImage, deployments[2].Name)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentOneCreated), deployments[2].ActiveFrom)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentTwoCreated), deployments[2].ActiveTo)
}

func TestGetDeployments_OneEnvironment_Latest(t *testing.T) {
	deploymentOneImage := "abcdef"
	deploymentTwoImage := "ghijkl"
	deploymentThreeImage := "mnopqr"
	layout := "2006-01-02T15:04:05.000Z"
	deploymentOneCreated, _ := time.Parse(layout, "2018-11-12T11:45:26.371Z")
	deploymentTwoCreated, _ := time.Parse(layout, "2018-11-12T12:30:14.000Z")
	deploymentThreeCreated, _ := time.Parse(layout, "2018-11-20T09:00:00.000Z")

	// Setup
	commonTestUtils, controllerTestUtils, _, _, _ := setupTest()
	setupGetDeploymentsTest(commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, []string{"dev"})

	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deployments?latest=true", anyAppName))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	controllertest.GetResponseBody(response, &deployments)
	assert.Equal(t, 1, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentThreeCreated), deployments[0].ActiveFrom)
	assert.Equal(t, "", deployments[0].ActiveTo)
}

func TestGetDeployments_TwoEnvironments_SortedWithFromTo(t *testing.T) {
	deploymentOneImage := "abcdef"
	deploymentTwoImage := "ghijkl"
	deploymentThreeImage := "mnopqr"
	layout := "2006-01-02T15:04:05.000Z"
	deploymentOneCreated, _ := time.Parse(layout, "2018-11-12T11:45:26.371Z")
	deploymentTwoCreated, _ := time.Parse(layout, "2018-11-12T12:30:14.000Z")
	deploymentThreeCreated, _ := time.Parse(layout, "2018-11-20T09:00:00.000Z")

	// Setup
	commonTestUtils, controllerTestUtils, _, _, _ := setupTest()
	setupGetDeploymentsTest(commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, []string{"dev", "prod"})

	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deployments", anyAppName))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	controllertest.GetResponseBody(response, &deployments)
	assert.Equal(t, 3, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentThreeCreated), deployments[0].ActiveFrom)
	assert.Equal(t, "", deployments[0].ActiveTo)

	assert.Equal(t, deploymentTwoImage, deployments[1].Name)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentTwoCreated), deployments[1].ActiveFrom)
	assert.Equal(t, "", deployments[1].ActiveTo)

	assert.Equal(t, deploymentOneImage, deployments[2].Name)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentOneCreated), deployments[2].ActiveFrom)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentThreeCreated), deployments[2].ActiveTo)
}

func TestGetDeployments_TwoEnvironments_Latest(t *testing.T) {
	deploymentOneImage := "abcdef"
	deploymentTwoImage := "ghijkl"
	deploymentThreeImage := "mnopqr"
	layout := "2006-01-02T15:04:05.000Z"
	deploymentOneCreated, _ := time.Parse(layout, "2018-11-12T11:45:26.371Z")
	deploymentTwoCreated, _ := time.Parse(layout, "2018-11-12T12:30:14.000Z")
	deploymentThreeCreated, _ := time.Parse(layout, "2018-11-20T09:00:00.000Z")

	// Setup
	commonTestUtils, controllerTestUtils, _, _, _ := setupTest()
	setupGetDeploymentsTest(commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, []string{"dev", "prod"})

	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deployments?latest=true", anyAppName))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	controllertest.GetResponseBody(response, &deployments)
	assert.Equal(t, 2, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentThreeCreated), deployments[0].ActiveFrom)
	assert.Equal(t, "", deployments[0].ActiveTo)

	assert.Equal(t, deploymentTwoImage, deployments[1].Name)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentTwoCreated), deployments[1].ActiveFrom)
	assert.Equal(t, "", deployments[1].ActiveTo)
}

func TestGetDeployment_NoApplicationRegistered(t *testing.T) {
	_, controllerTestUtils, _, _, _ := setupTest()
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deployments/%s", anyAppName, anyDeployName))
	response := <-responseChannel

	assert.Equal(t, 404, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, controllertest.AppNotFoundErrorMsg(anyAppName), errorResponse.Message)
}

func TestGetDeployment_TwoDeploymentsFirstDeployment_ReturnsDeploymentWithComponents(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _ := setupTest()
	anyAppName := "any-app"
	anyEnvironment := "dev"
	anyDeployment1Name := "abcdef"
	anyDeployment2Name := "ghijkl"
	appDeployment1Created, _ := radixutils.ParseTimestamp("2018-11-12T12:00:00Z")
	appDeployment2Created, _ := radixutils.ParseTimestamp("2018-11-14T12:00:00Z")

	commonTestUtils.ApplyDeployment(builders.
		NewDeploymentBuilder().
		WithRadixApplication(
			builders.ARadixApplication().
				WithAppName(anyAppName)).
		WithAppName(anyAppName).
		WithDeploymentName(anyDeployment1Name).
		WithCreated(appDeployment1Created).
		WithCondition(v1.DeploymentInactive).
		WithActiveFrom(appDeployment1Created).
		WithActiveTo(appDeployment2Created).
		WithEnvironment(anyEnvironment).
		WithImageTag(anyDeployment1Name).
		WithJobComponents(
			builders.NewDeployJobComponentBuilder().WithName("job1"),
			builders.NewDeployJobComponentBuilder().WithName("job2"),
		).
		WithComponents(
			builders.NewDeployComponentBuilder().
				WithImage("radixdev.azurecr.io/some-image:imagetag").
				WithName("frontend").
				WithPort("http", 8080).
				WithPublic(true).
				WithReplicas(commontest.IntPtr(1)),
			builders.NewDeployComponentBuilder().
				WithImage("radixdev.azurecr.io/another-image:imagetag").
				WithName("backend").
				WithPublic(false).
				WithReplicas(commontest.IntPtr(1))))

	commonTestUtils.ApplyDeployment(builders.
		NewDeploymentBuilder().
		WithRadixApplication(
			builders.ARadixApplication().
				WithAppName(anyAppName)).
		WithAppName(anyAppName).
		WithDeploymentName(anyDeployment2Name).
		WithCreated(appDeployment2Created).
		WithCondition(v1.DeploymentActive).
		WithActiveFrom(appDeployment2Created).
		WithEnvironment(anyEnvironment).
		WithImageTag(anyDeployment2Name).
		WithJobComponents(
			builders.NewDeployJobComponentBuilder().WithName("job1"),
		).
		WithComponents(
			builders.NewDeployComponentBuilder().
				WithImage("radixdev.azurecr.io/another-second-image:imagetag").
				WithName("backend").
				WithPublic(false).
				WithReplicas(commontest.IntPtr(1))))

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deployments/%s", anyAppName, anyDeployment1Name))
	response := <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)

	deployment := deploymentModels.Deployment{}
	controllertest.GetResponseBody(response, &deployment)

	assert.Equal(t, anyDeployment1Name, deployment.Name)
	assert.Equal(t, radixutils.FormatTimestamp(appDeployment1Created), deployment.ActiveFrom)
	assert.Equal(t, radixutils.FormatTimestamp(appDeployment2Created), deployment.ActiveTo)
	assert.Equal(t, 4, len(deployment.Components))

}

func createAppNamespace(kubeclient kubernetes.Interface, appName string) string {
	ns := builders.GetAppNamespace(appName)
	createNamespace(kubeclient, ns)
	return ns
}

func createEnvNamespace(kubeclient kubernetes.Interface, appName, environment string) string {
	ns := builders.GetEnvironmentNamespace(appName, environment)
	createNamespace(kubeclient, ns)
	return ns
}

func createNamespace(kubeclient kubernetes.Interface, ns string) {
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}

	kubeclient.CoreV1().Namespaces().Create(context.TODO(), &namespace, metav1.CreateOptions{})
}

func setupGetDeploymentsTest(commonTestUtils *commontest.Utils, appName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage string, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated time.Time, environments []string) {
	var environmentOne, environmentTwo string
	var deploymentOneActiveTo, deploymentTwoActiveTo time.Time
	var deploymentTwoCondition v1.RadixDeployCondition

	if len(environments) == 1 {
		environmentOne = environments[0]
		environmentTwo = environments[0]
		deploymentOneActiveTo = deploymentTwoCreated
		deploymentTwoActiveTo = deploymentThreeCreated
		deploymentTwoCondition = v1.DeploymentInactive

	} else {
		environmentOne = environments[0]
		environmentTwo = environments[1]
		deploymentOneActiveTo = deploymentThreeCreated
		deploymentTwoCondition = v1.DeploymentActive
	}

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName(deploymentOneImage).
		WithAppName(appName).
		WithEnvironment(environmentOne).
		WithImageTag(deploymentOneImage).
		WithCreated(deploymentOneCreated).
		WithCondition(v1.DeploymentInactive).
		WithActiveFrom(deploymentOneCreated).
		WithActiveTo(deploymentOneActiveTo))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName(deploymentTwoImage).
		WithAppName(appName).
		WithEnvironment(environmentTwo).
		WithImageTag(deploymentTwoImage).
		WithCreated(deploymentTwoCreated).
		WithCondition(deploymentTwoCondition).
		WithActiveFrom(deploymentTwoCreated).
		WithActiveTo(deploymentTwoActiveTo))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName(deploymentThreeImage).
		WithAppName(appName).
		WithEnvironment(environmentOne).
		WithImageTag(deploymentThreeImage).
		WithCreated(deploymentThreeCreated).
		WithCondition(v1.DeploymentActive).
		WithActiveFrom(deploymentThreeCreated))
}
