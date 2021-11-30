package buildsecrets

import (
	"fmt"
	environmentModels "github.com/equinor/radix-api/api/secrets/models"
	"net/http"
	"testing"

	"github.com/equinor/radix-api/api/buildsecrets/models"
	controllertest "github.com/equinor/radix-api/api/test"
	"github.com/equinor/radix-api/api/utils"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

const (
	clusterName       = "AnyClusterName"
	containerRegistry = "any.container.registry"
	anyAppName        = "any-app"
)

func setupTest() (*commontest.Utils, *controllertest.Utils, kubernetes.Interface, radixclient.Interface) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := fake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient)
	commonTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)

	// controllerTestUtils is used for issuing HTTP request and processing responses
	controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, NewBuildSecretsController())

	return &commonTestUtils, &controllerTestUtils, kubeclient, radixclient
}

func TestGetBuildSecrets_ListsAll(t *testing.T) {
	anyBuildSecret1 := "secret1"
	anyBuildSecret2 := "secret2"
	anyBuildSecret3 := "secret3"

	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient := setupTest()

	utils.ApplyApplicationWithSync(client, radixclient, commonTestUtils,
		builders.ARadixApplication().
			WithAppName(anyAppName).
			WithBuildSecrets(anyBuildSecret1, anyBuildSecret2))

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/buildsecrets", anyAppName))
	response := <-responseChannel

	buildSecrets := make([]models.BuildSecret, 0)
	controllertest.GetResponseBody(response, &buildSecrets)
	assert.Equal(t, 2, len(buildSecrets))
	assert.Equal(t, anyBuildSecret1, buildSecrets[0].Name)
	assert.Equal(t, anyBuildSecret2, buildSecrets[1].Name)

	utils.ApplyApplicationWithSync(client, radixclient, commonTestUtils,
		builders.ARadixApplication().
			WithAppName(anyAppName).
			WithBuildSecrets(anyBuildSecret1, anyBuildSecret2, anyBuildSecret3))

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/buildsecrets", anyAppName))
	response = <-responseChannel

	buildSecrets = make([]models.BuildSecret, 0)
	controllertest.GetResponseBody(response, &buildSecrets)
	assert.Equal(t, 3, len(buildSecrets))
	assert.Equal(t, anyBuildSecret1, buildSecrets[0].Name)
	assert.Equal(t, anyBuildSecret2, buildSecrets[1].Name)
	assert.Equal(t, anyBuildSecret3, buildSecrets[2].Name)

	utils.ApplyApplicationWithSync(client, radixclient, commonTestUtils,
		builders.ARadixApplication().
			WithAppName(anyAppName).
			WithBuildSecrets(anyBuildSecret1, anyBuildSecret3))

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/buildsecrets", anyAppName))
	response = <-responseChannel

	buildSecrets = make([]models.BuildSecret, 0)
	controllertest.GetResponseBody(response, &buildSecrets)
	assert.Equal(t, 2, len(buildSecrets))
	assert.Equal(t, anyBuildSecret1, buildSecrets[0].Name)
	assert.Equal(t, anyBuildSecret3, buildSecrets[1].Name)
}

func TestUpdateBuildSecret_UpdatedOk(t *testing.T) {
	anyBuildSecret1 := "secret1"

	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient := setupTest()

	utils.ApplyApplicationWithSync(client, radixclient, commonTestUtils,
		builders.ARadixApplication().
			WithAppName(anyAppName).
			WithBuildSecrets(anyBuildSecret1))

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/buildsecrets", anyAppName))
	response := <-responseChannel

	buildSecrets := make([]models.BuildSecret, 0)
	controllertest.GetResponseBody(response, &buildSecrets)
	assert.Equal(t, 1, len(buildSecrets))
	assert.Equal(t, anyBuildSecret1, buildSecrets[0].Name)
	assert.Equal(t, models.Pending.String(), buildSecrets[0].Status)

	parameters := environmentModels.SecretParameters{
		SecretValue: "anyValue",
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s/buildsecrets/%s", anyAppName, anyBuildSecret1), parameters)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/buildsecrets", anyAppName))
	response = <-responseChannel

	buildSecrets = make([]models.BuildSecret, 0)
	controllertest.GetResponseBody(response, &buildSecrets)
	assert.Equal(t, 1, len(buildSecrets))
	assert.Equal(t, anyBuildSecret1, buildSecrets[0].Name)
	assert.Equal(t, models.Consistent.String(), buildSecrets[0].Status)
}
