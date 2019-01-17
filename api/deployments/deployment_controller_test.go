package deployments

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/equinor/radix-api/api/utils"
	"github.com/stretchr/testify/assert"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	controllertest "github.com/equinor/radix-api/api/test"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

const (
	anyAppName    = "any-app"
	anyCloneURL   = "git@github.com:Equinor/any-app.git"
	anyPodName    = "any-pod"
	anyDeployName = "any-deploy"
	anyEnv        = "any-env"
)

func createGetLogEndpoint(appName, podName string) string {
	return fmt.Sprintf("/api/v1/applications/%s/deployments/any/components/any/replicas/%s/logs", appName, podName)
}

func setupTest() (*commontest.Utils, *controllertest.Utils, kubernetes.Interface, radixclient.Interface) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := fake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient)

	// controllerTestUtils is used for issuing HTTP request and processing responses
	controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, NewDeploymentController())

	return &commonTestUtils, &controllerTestUtils, kubeclient, radixclient
}

func TestGetPodLog_no_radixconfig(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _ := setupTest()

	endpoint := createGetLogEndpoint(anyAppName, anyPodName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 404, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := deploymentModels.NonExistingApplication(nil, anyAppName)

	assert.Equal(t, (expectedError.(*utils.Error)).Message, errorResponse.Message)

}

func TestGetPodLog_No_Pod(t *testing.T) {
	commonTestUtils, controllerTestUtils, _, _ := setupTest()
	endpoint := createGetLogEndpoint(anyAppName, anyPodName)

	commonTestUtils.ApplyApplication(builders.
		ARadixApplication().
		WithAppName(anyAppName))

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 404, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := deploymentModels.NonExistingPod(anyAppName, anyPodName)

	assert.Equal(t, (expectedError.(*utils.Error)).Error(), errorResponse.Error())

}

func TestGetDeployments_Filter_FilterIsApplied(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _ := setupTest()

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName("any-app-1").
		WithEnvironment("prod").
		WithImageTag("abcdef"))

	// Ensure the second image is considered the latest version
	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName("any-app-2").
		WithEnvironment("dev").
		WithImageTag("ghijklm").
		WithCreated(time.Now()))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName("any-app-2").
		WithEnvironment("dev").
		WithImageTag("nopqrst").
		WithCreated(time.Now().AddDate(0, 0, 1)))

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

func TestGetDeployments_OneEnvironment_SortedWithFromTo(t *testing.T) {
	deploymentOneImage := "abcdef"
	deploymentTwoImage := "ghijkl"
	deploymentThreeImage := "mnopqr"
	layout := "2006-01-02T15:04:05.000Z"
	deploymentOneCreated, _ := time.Parse(layout, "2018-11-12T11:45:26.371Z")
	deploymentTwoCreated, _ := time.Parse(layout, "2018-11-12T12:30:14.000Z")
	deploymentThreeCreated, _ := time.Parse(layout, "2018-11-20T09:00:00.000Z")

	// Setup
	commonTestUtils, controllerTestUtils, _, _ := setupTest()
	setupGetDeploymentsTest(commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, []string{"dev"})

	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deployments", anyAppName))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	controllertest.GetResponseBody(response, &deployments)
	assert.Equal(t, 3, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, utils.FormatTimestamp(deploymentThreeCreated), deployments[0].ActiveFrom)
	assert.Equal(t, "", deployments[0].ActiveTo)

	assert.Equal(t, deploymentTwoImage, deployments[1].Name)
	assert.Equal(t, utils.FormatTimestamp(deploymentTwoCreated), deployments[1].ActiveFrom)
	assert.Equal(t, utils.FormatTimestamp(deploymentThreeCreated), deployments[1].ActiveTo)

	assert.Equal(t, deploymentOneImage, deployments[2].Name)
	assert.Equal(t, utils.FormatTimestamp(deploymentOneCreated), deployments[2].ActiveFrom)
	assert.Equal(t, utils.FormatTimestamp(deploymentTwoCreated), deployments[2].ActiveTo)
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
	commonTestUtils, controllerTestUtils, _, _ := setupTest()
	setupGetDeploymentsTest(commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, []string{"dev"})

	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deployments?latest=true", anyAppName))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	controllertest.GetResponseBody(response, &deployments)
	assert.Equal(t, 1, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, utils.FormatTimestamp(deploymentThreeCreated), deployments[0].ActiveFrom)
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
	commonTestUtils, controllerTestUtils, _, _ := setupTest()
	setupGetDeploymentsTest(commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, []string{"dev", "prod"})

	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deployments", anyAppName))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	controllertest.GetResponseBody(response, &deployments)
	assert.Equal(t, 3, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, utils.FormatTimestamp(deploymentThreeCreated), deployments[0].ActiveFrom)
	assert.Equal(t, "", deployments[0].ActiveTo)

	assert.Equal(t, deploymentTwoImage, deployments[1].Name)
	assert.Equal(t, utils.FormatTimestamp(deploymentTwoCreated), deployments[1].ActiveFrom)
	assert.Equal(t, "", deployments[1].ActiveTo)

	assert.Equal(t, deploymentOneImage, deployments[2].Name)
	assert.Equal(t, utils.FormatTimestamp(deploymentOneCreated), deployments[2].ActiveFrom)
	assert.Equal(t, utils.FormatTimestamp(deploymentThreeCreated), deployments[2].ActiveTo)
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
	commonTestUtils, controllerTestUtils, _, _ := setupTest()
	setupGetDeploymentsTest(commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, []string{"dev", "prod"})

	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deployments?latest=true", anyAppName))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	controllertest.GetResponseBody(response, &deployments)
	assert.Equal(t, 2, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, utils.FormatTimestamp(deploymentThreeCreated), deployments[0].ActiveFrom)
	assert.Equal(t, "", deployments[0].ActiveTo)

	assert.Equal(t, deploymentTwoImage, deployments[1].Name)
	assert.Equal(t, utils.FormatTimestamp(deploymentTwoCreated), deployments[1].ActiveFrom)
	assert.Equal(t, "", deployments[1].ActiveTo)
}

func TestPromote_ErrorScenarios_ErrorIsReturned(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _ := setupTest()

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName("1").
		WithAppName("any-app-1").
		WithEnvironment("prod").
		WithImageTag("abcdef"))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName("2").
		WithAppName("any-app-1").
		WithEnvironment("dev").
		WithImageTag("abcdef"))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName("3").
		WithAppName("any-app-2").
		WithEnvironment("dev").
		WithImageTag("abcdef"))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName("4").
		WithAppName("any-app-2").
		WithEnvironment("prod").
		WithImageTag("ghijklm"))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName("5").
		WithAppName("any-app-3").
		WithEnvironment("dev").
		WithImageTag("abcdef"))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName("5").
		WithAppName("any-app-3").
		WithEnvironment("prod").
		WithImageTag("abcdef"))

	createEnvNamespace(kubeclient, "any-app-4", "dev")
	createEnvNamespace(kubeclient, "any-app-4", "prod")

	irrellevantUnderlyingError := errors.New("Any undelying error irrellevant for testing")
	var testScenarios = []struct {
		name               string
		appName            string
		fromEnvironment    string
		imageTag           string
		toEnvironment      string
		deploymentName     string
		excectedReturnCode int
		expectedError      error
	}{
		{"promote non-existing app", "noapp", "dev", "abcdef", "prod", "2", http.StatusNotFound, deploymentModels.NonExistingApplication(irrellevantUnderlyingError, "noapp")},
		{"promote from non-existing environment", "any-app-1", "qa", "abcdef", "prod", "2", http.StatusNotFound, deploymentModels.NonExistingFromEnvironment(irrellevantUnderlyingError)},
		{"promote to non-existing environment", "any-app-1", "dev", "abcdef", "qa", "2", http.StatusNotFound, deploymentModels.NonExistingToEnvironment(irrellevantUnderlyingError)},
		{"promote non-existing deployment", "any-app-2", "dev", "nopqrst", "prod", "non-existing", http.StatusNotFound, deploymentModels.NonExistingDeployment(irrellevantUnderlyingError, "non-existing")},
		{"promote an deployment into environment having already that deployment", "any-app-3", "dev", "abcdef", "prod", "5", http.StatusConflict, nil}, // Error comes from kubernetes API
	}

	for _, scenario := range testScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			parameters := deploymentModels.PromotionParameters{FromEnvironment: scenario.fromEnvironment, ToEnvironment: scenario.toEnvironment}

			deploymentName := scenario.deploymentName
			responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/deployments/%s/promote", scenario.appName, deploymentName), parameters)
			response := <-responseChannel

			assert.Equal(t, scenario.excectedReturnCode, response.Code)
			errorResponse, _ := controllertest.GetErrorResponse(response)

			if scenario.expectedError != nil {
				assert.Equal(t, (scenario.expectedError.(*utils.Error)).Message, errorResponse.Message)
			}
		})
	}
}

func TestPromote_HappyPathScenarios_NewStateIsExpected(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, radixclient := setupTest()
	deployHandler := Init(kubeclient, radixclient)
	deploymentName := "abcdef"

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName(deploymentName).
		WithAppName("any-app-1").
		WithEnvironment("dev").
		WithImageTag("abcdef"))

	// Create prod environment without any deployments
	createEnvNamespace(kubeclient, "any-app-1", "prod")

	var testScenarios = []struct {
		name            string
		appName         string
		fromEnvironment string
		imageTag        string
		toEnvironment   string
		imageExpected   string
	}{
		{"promote single image", "any-app-1", "dev", "abcdef", "prod", ""},
	}

	for _, scenario := range testScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			parameters := deploymentModels.PromotionParameters{FromEnvironment: scenario.fromEnvironment, ToEnvironment: scenario.toEnvironment}

			responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/deployments/%s/promote", scenario.appName, deploymentName), parameters)
			response := <-responseChannel

			assert.Equal(t, http.StatusOK, response.Code)

			if scenario.imageExpected != "" {
				deployments, _ := deployHandler.GetDeploymentsForApplicationEnvironment(scenario.appName, scenario.toEnvironment, false)
				assert.Equal(t, 1, len(deployments))
				assert.Equal(t, deploymentName, deployments[0].Name)
			}
		})
	}
}

func TestGetDeployment_TwoDeploymentsFirstDeployment_ReturnsDeploymentWithComponents(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _ := setupTest()
	anyAppName := "any-app"
	anyEnvironment := "dev"
	anyDeployment1Name := "abcdef"
	anyDeployment2Name := "ghijkl"
	appDeployment1Created, _ := utils.ParseTimestamp("2018-11-12T12:00:00-0000")
	appDeployment2Created, _ := utils.ParseTimestamp("2018-11-14T12:00:00-0000")

	commonTestUtils.ApplyDeployment(builders.
		NewDeploymentBuilder().
		WithAppName(anyAppName).
		WithDeploymentName(anyDeployment1Name).
		WithCreated(appDeployment1Created).
		WithEnvironment(anyEnvironment).
		WithImageTag(anyDeployment1Name).
		WithComponents(
			builders.NewDeployComponentBuilder().
				WithImage("radixdev.azurecr.io/some-image:imagetag").
				WithName("frontend").
				WithPort("http", 8080).
				WithPublic(true).
				WithReplicas(1),
			builders.NewDeployComponentBuilder().
				WithImage("radixdev.azurecr.io/another-image:imagetag").
				WithName("backend").
				WithPublic(false).
				WithReplicas(1)))

	commonTestUtils.ApplyDeployment(builders.
		NewDeploymentBuilder().
		WithAppName(anyAppName).
		WithDeploymentName(anyDeployment2Name).
		WithCreated(appDeployment2Created).
		WithEnvironment(anyEnvironment).
		WithImageTag(anyDeployment2Name).
		WithComponents(
			builders.NewDeployComponentBuilder().
				WithImage("radixdev.azurecr.io/another-second-image:imagetag").
				WithName("backend").
				WithPublic(false).
				WithReplicas(1)))

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deployments/%s", anyAppName, anyDeployment1Name))
	response := <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)

	deployment := deploymentModels.Deployment{}
	controllertest.GetResponseBody(response, &deployment)

	assert.Equal(t, anyDeployment1Name, deployment.Name)
	assert.Equal(t, utils.FormatTimestamp(appDeployment1Created), deployment.ActiveFrom)
	assert.Equal(t, utils.FormatTimestamp(appDeployment2Created), deployment.ActiveTo)
	assert.Equal(t, 2, len(deployment.Components))

}

func TestPromote_WithEnvironmentVariables_NewStateIsExpected(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, radixclient := setupTest()
	deployHandler := Init(kubeclient, radixclient)
	anyAppName := "any-app-2"
	deploymentName := "abcdef"

	// Setup
	// When we have enviroment specific config the deployment should contain the environment variables defined in the config
	devVariable := make(map[string]string)
	prodVariable := make(map[string]string)
	devVariable["DB_HOST"] = "useless-dev"
	prodVariable["DB_HOST"] = "useless-prod"

	environmentVariables := []v1.EnvVars{
		v1.EnvVars{Environment: "dev", Variables: devVariable},
		v1.EnvVars{Environment: "prod", Variables: prodVariable}}

	commonTestUtils.ApplyApplication(builders.
		ARadixApplication().
		WithAppName(anyAppName).
		WithComponents(
			builders.
				NewApplicationComponentBuilder().
				WithName("app").
				WithEnvironmentVariablesMap(environmentVariables)).
		WithEnvironment("dev", "master").
		WithEnvironment("prod", ""))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName(deploymentName).
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithImageTag("abcdef").
		WithComponent(
			builders.NewDeployComponentBuilder().
				WithName("app")))

	// Create prod environment without any deployments
	createEnvNamespace(kubeclient, anyAppName, "prod")

	// Scenario
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/deployments/%s/promote", anyAppName, deploymentName), deploymentModels.PromotionParameters{FromEnvironment: "dev", ToEnvironment: "prod"})
	response := <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)

	deployments, _ := deployHandler.GetDeploymentsForApplicationEnvironment(anyAppName, "prod", false)
	assert.Equal(t, 1, len(deployments), "HandlePromoteToEnvironment - Was not promoted as expected")

	// Get the RD to see if it has merged ok with the RA
	radixDeployment, _ := radixclient.RadixV1().RadixDeployments(builders.GetEnvironmentNamespace(anyAppName, deployments[0].Environment)).Get(deployments[0].Name, metav1.GetOptions{})
	assert.Equal(t, 1, len(radixDeployment.Spec.Components[0].EnvironmentVariables), "HandlePromoteToEnvironment - Was not promoted as expected")
	assert.Equal(t, "useless-prod", radixDeployment.Spec.Components[0].EnvironmentVariables["DB_HOST"], "HandlePromoteToEnvironment - Was not promoted as expected")

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

	kubeclient.CoreV1().Namespaces().Create(&namespace)
}

func setupGetDeploymentsTest(commonTestUtils *commontest.Utils, appName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage string, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated time.Time, environments []string) {
	var environmentOne, environmentTwo string

	if len(environments) == 1 {
		environmentOne = environments[0]
		environmentTwo = environments[0]
	} else {
		environmentOne = environments[0]
		environmentTwo = environments[1]
	}

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName(deploymentOneImage).
		WithAppName(appName).
		WithEnvironment(environmentOne).
		WithImageTag(deploymentOneImage).
		WithCreated(deploymentOneCreated))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName(deploymentTwoImage).
		WithAppName(appName).
		WithEnvironment(environmentTwo).
		WithImageTag(deploymentTwoImage).
		WithCreated(deploymentTwoCreated))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName(deploymentThreeImage).
		WithAppName(appName).
		WithEnvironment(environmentOne).
		WithImageTag(deploymentThreeImage).
		WithCreated(deploymentThreeCreated))
}
