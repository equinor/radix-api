package deployments

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/statoil/radix-api/api/utils"
	"github.com/stretchr/testify/assert"

	deploymentModels "github.com/statoil/radix-api/api/deployments/models"
	controllertest "github.com/statoil/radix-api/api/test"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	commontest "github.com/statoil/radix-operator/pkg/apis/test"
	builders "github.com/statoil/radix-operator/pkg/apis/utils"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

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

			deployments := make([]*deploymentModels.ApplicationDeployment, 0)
			controllertest.GetResponseBody(response, &deployments)
			assert.Equal(t, scenario.numDeploymentsExpected, len(deployments))
		})
	}
}

func TestGetDeployments_SortedWithFromTo(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _ := setupTest()

	anyAppName := "any-app-1"
	deploymentOneImage := "abcdef"
	deploymentTwoImage := "ghijkl"
	deploymentThreeImage := "mnopqr"

	layout := "2006-01-02T15:04:05.000Z"
	deploymentOneCreated, _ := time.Parse(layout, "2018-11-12T11:45:26.371Z")
	deploymentTwoCreated, _ := time.Parse(layout, "2018-11-12T12:30:14.000Z")
	deploymentThreeCreated, _ := time.Parse(layout, "2018-11-20T09:00:00.000Z")

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithImageTag(deploymentOneImage).
		WithCreated(deploymentOneCreated))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithImageTag(deploymentTwoImage).
		WithCreated(deploymentTwoCreated))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithImageTag(deploymentThreeImage).
		WithCreated(deploymentThreeCreated))

	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deployments", "any-app-1"))
	response := <-responseChannel

	deployments := make([]*deploymentModels.ApplicationDeployment, 0)
	controllertest.GetResponseBody(response, &deployments)
	assert.Equal(t, 3, len(deployments))

	assert.Equal(t, builders.GetDeploymentName(anyAppName, deploymentThreeImage), deployments[0].Name)
	assert.Equal(t, utils.FormatTimestamp(deploymentThreeCreated), deployments[0].ActiveFrom)
	assert.Equal(t, "", deployments[0].ActiveTo)

	assert.Equal(t, builders.GetDeploymentName(anyAppName, deploymentTwoImage), deployments[1].Name)
	assert.Equal(t, utils.FormatTimestamp(deploymentTwoCreated), deployments[1].ActiveFrom)
	assert.Equal(t, utils.FormatTimestamp(deploymentThreeCreated), deployments[1].ActiveTo)

	assert.Equal(t, builders.GetDeploymentName(anyAppName, deploymentOneImage), deployments[2].Name)
	assert.Equal(t, utils.FormatTimestamp(deploymentOneCreated), deployments[2].ActiveFrom)
	assert.Equal(t, utils.FormatTimestamp(deploymentTwoCreated), deployments[2].ActiveTo)

}

func TestPromote_ErrorScenarios_ErrorIsReturned(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _ := setupTest()

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName("any-app-1").
		WithEnvironment("prod").
		WithImageTag("abcdef"))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName("any-app-1").
		WithEnvironment("dev").
		WithImageTag("abcdef"))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName("any-app-2").
		WithEnvironment("dev").
		WithImageTag("abcdef"))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName("any-app-2").
		WithEnvironment("prod").
		WithImageTag("ghijklm"))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName("any-app-3").
		WithEnvironment("dev").
		WithImageTag("abcdef"))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
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
		excectedReturnCode int
		expectedError      error
	}{
		{"promote non-existing app", "noapp", "dev", "abcdef", "prod", http.StatusNotFound, nonExistingApplication(irrellevantUnderlyingError, "noapp")},
		{"promote from non-existing environment", "any-app-1", "qa", "abcdef", "prod", http.StatusNotFound, nonExistingFromEnvironment(irrellevantUnderlyingError)},
		{"promote to non-existing environment", "any-app-1", "dev", "abcdef", "qa", http.StatusNotFound, nonExistingToEnvironment(irrellevantUnderlyingError)},
		{"promote non-existing image", "any-app-2", "dev", "nopqrst", "prod", http.StatusNotFound, nonExistingDeployment(irrellevantUnderlyingError)},
		{"promote an image into environment having already that image", "any-app-3", "dev", "abcdef", "prod", http.StatusConflict, nil}, // Error comes from kubernetes API
	}

	for _, scenario := range testScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			parameters := deploymentModels.PromotionParameters{FromEnvironment: scenario.fromEnvironment, ToEnvironment: scenario.toEnvironment}

			deploymentName := builders.GetDeploymentName(scenario.appName, scenario.imageTag)
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

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
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

			deploymentName := builders.GetDeploymentName(scenario.appName, scenario.imageTag)
			responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/deployments/%s/promote", scenario.appName, deploymentName), parameters)
			response := <-responseChannel

			assert.Equal(t, http.StatusOK, response.Code)

			if scenario.imageExpected != "" {
				deployments, _ := HandleGetDeployments(radixclient, scenario.appName, scenario.toEnvironment, false)
				assert.Equal(t, 1, len(deployments))
				assert.Equal(t, deploymentName, deployments[0].Name)
			}
		})
	}
}

func TestPromote_WithEnvironmentVariables_NewStateIsExpected(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, radixclient := setupTest()

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
		WithAppName("any-app-2").
		WithComponents(
			builders.
				NewApplicationComponentBuilder().
				WithName("app").
				WithEnvironmentVariablesMap(environmentVariables)).
		WithEnvironment("dev", "master").
		WithEnvironment("prod", ""))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName("any-app-2").
		WithEnvironment("dev").
		WithImageTag("abcdef").
		WithComponent(
			builders.NewDeployComponentBuilder().
				WithName("app")))

	// Create prod environment without any deployments
	createEnvNamespace(kubeclient, "any-app-2", "prod")

	// Scenario
	deploymentName := builders.GetDeploymentName("any-app-2", "abcdef")
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/deployments/%s/promote", "any-app-2", deploymentName), deploymentModels.PromotionParameters{FromEnvironment: "dev", ToEnvironment: "prod"})
	response := <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)

	deployments, _ := HandleGetDeployments(radixclient, "any-app-2", "prod", false)
	assert.Equal(t, 1, len(deployments), "HandlePromoteToEnvironment - Was not promoted as expected")

	// Get the RD to see if it has merged ok with the RA
	radixDeployment, _ := radixclient.RadixV1().RadixDeployments(builders.GetEnvironmentNamespace(deployments[0].AppName, deployments[0].Environment)).Get(deployments[0].Name, metav1.GetOptions{})
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
