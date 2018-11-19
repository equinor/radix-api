package environments

import (
	"fmt"
	"testing"
	"time"

	deploymentModels "github.com/statoil/radix-api/api/deployments/models"
	controllertest "github.com/statoil/radix-api/api/test"
	"github.com/statoil/radix-api/api/utils"
	commontest "github.com/statoil/radix-operator/pkg/apis/test"
	builders "github.com/statoil/radix-operator/pkg/apis/utils"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func setupTest() (*commontest.Utils, *controllertest.Utils, kubernetes.Interface, radixclient.Interface) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := fake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient)

	// controllerTestUtils is used for issuing HTTP request and processing responses
	controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, NewEnvironmentController())

	return &commonTestUtils, &controllerTestUtils, kubeclient, radixclient
}

func TestGetEnvironmentDeployments_SortedWithFromTo(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _ := setupTest()

	anyAppName := "any-app-1"
	envName := "dev"
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
		WithEnvironment(envName).
		WithImageTag(deploymentOneImage).
		WithCreated(deploymentOneCreated))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(envName).
		WithImageTag(deploymentTwoImage).
		WithCreated(deploymentTwoCreated))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(envName).
		WithImageTag(deploymentThreeImage).
		WithCreated(deploymentThreeCreated))

	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/deployments", anyAppName, envName))
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

func TestGetEnvironmentDeployments_Latest(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _ := setupTest()

	anyAppName := "any-app-1"
	envName := "dev"
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
		WithEnvironment(envName).
		WithImageTag(deploymentOneImage).
		WithCreated(deploymentOneCreated))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(envName).
		WithImageTag(deploymentTwoImage).
		WithCreated(deploymentTwoCreated))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(envName).
		WithImageTag(deploymentThreeImage).
		WithCreated(deploymentThreeCreated))

	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/deployments?latest=true", anyAppName, envName))
	response := <-responseChannel

	deployments := make([]*deploymentModels.ApplicationDeployment, 0)
	controllertest.GetResponseBody(response, &deployments)
	assert.Equal(t, 1, len(deployments))

	assert.Equal(t, builders.GetDeploymentName(anyAppName, deploymentThreeImage), deployments[0].Name)
	assert.Equal(t, utils.FormatTimestamp(deploymentThreeCreated), deployments[0].ActiveFrom)
	assert.Equal(t, "", deployments[0].ActiveTo)
}
