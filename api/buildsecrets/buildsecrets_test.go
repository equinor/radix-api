package buildsecrets

import (
	"fmt"
	"net/http"
	"testing"

	environmentModels "github.com/equinor/radix-api/api/secrets/models"
	authnmock "github.com/equinor/radix-api/api/utils/token/mock"
	"github.com/golang/mock/gomock"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"

	certclientfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	"github.com/equinor/radix-api/api/buildsecrets/models"
	controllertest "github.com/equinor/radix-api/api/test"
	"github.com/equinor/radix-api/api/utils"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

const (
	clusterName    = "AnyClusterName"
	anyAppName     = "any-app"
	egressIps      = "0.0.0.0"
	subscriptionId = "12347718-c8f8-4995-bfbb-02655ff1f89c"
)

func setupTest(t *testing.T) (*commontest.Utils, *controllertest.Utils, *kubefake.Clientset, *radixfake.Clientset, *kedafake.Clientset) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := radixfake.NewSimpleClientset()
	kedaClient := kedafake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	certClient := certclientfake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient)
	err := commonTestUtils.CreateClusterPrerequisites(clusterName, egressIps, subscriptionId)
	require.NoError(t, err)
	// controllerTestUtils is used for issuing HTTP request and processing responses
	mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
	controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient, certClient, mockValidator, NewBuildSecretsController())

	return &commonTestUtils, &controllerTestUtils, kubeclient, radixclient, kedaClient
}

func TestGetBuildSecrets_ListsAll(t *testing.T) {
	anyBuildSecret1 := "secret1"
	anyBuildSecret2 := "secret2"
	anyBuildSecret3 := "secret3"

	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient, kedaClient := setupTest(t)

	err := utils.ApplyApplicationWithSync(client, radixclient, kedaClient, commonTestUtils,
		builders.ARadixApplication().
			WithAppName(anyAppName).
			WithBuildSecrets(anyBuildSecret1, anyBuildSecret2))
	require.NoError(t, err)

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/buildsecrets", anyAppName))
	response := <-responseChannel

	buildSecrets := make([]models.BuildSecret, 0)
	err = controllertest.GetResponseBody(response, &buildSecrets)
	require.NoError(t, err)
	assert.Equal(t, 2, len(buildSecrets))
	assert.Equal(t, anyBuildSecret1, buildSecrets[0].Name)
	assert.Equal(t, anyBuildSecret2, buildSecrets[1].Name)

	err = utils.ApplyApplicationWithSync(client, radixclient, kedaClient, commonTestUtils,
		builders.ARadixApplication().
			WithAppName(anyAppName).
			WithBuildSecrets(anyBuildSecret1, anyBuildSecret2, anyBuildSecret3))
	require.NoError(t, err)

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/buildsecrets", anyAppName))
	response = <-responseChannel

	buildSecrets = make([]models.BuildSecret, 0)
	err = controllertest.GetResponseBody(response, &buildSecrets)
	require.NoError(t, err)
	assert.Equal(t, 3, len(buildSecrets))
	assert.Equal(t, anyBuildSecret1, buildSecrets[0].Name)
	assert.Equal(t, anyBuildSecret2, buildSecrets[1].Name)
	assert.Equal(t, anyBuildSecret3, buildSecrets[2].Name)

	err = utils.ApplyApplicationWithSync(client, radixclient, kedaClient, commonTestUtils,
		builders.ARadixApplication().
			WithAppName(anyAppName).
			WithBuildSecrets(anyBuildSecret1, anyBuildSecret3))
	require.NoError(t, err)

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/buildsecrets", anyAppName))
	response = <-responseChannel

	buildSecrets = make([]models.BuildSecret, 0)
	err = controllertest.GetResponseBody(response, &buildSecrets)
	require.NoError(t, err)
	assert.Equal(t, 2, len(buildSecrets))
	assert.Equal(t, anyBuildSecret1, buildSecrets[0].Name)
	assert.Equal(t, anyBuildSecret3, buildSecrets[1].Name)
}

func TestUpdateBuildSecret_UpdatedOk(t *testing.T) {
	anyBuildSecret1 := "secret1"

	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient, kedaClient := setupTest(t)

	err := utils.ApplyApplicationWithSync(client, radixclient, kedaClient, commonTestUtils,
		builders.ARadixApplication().
			WithAppName(anyAppName).
			WithBuildSecrets(anyBuildSecret1))
	require.NoError(t, err)

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/buildsecrets", anyAppName))
	response := <-responseChannel

	buildSecrets := make([]models.BuildSecret, 0)
	err = controllertest.GetResponseBody(response, &buildSecrets)
	require.NoError(t, err)
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
	err = controllertest.GetResponseBody(response, &buildSecrets)
	require.NoError(t, err)
	assert.Equal(t, 1, len(buildSecrets))
	assert.Equal(t, anyBuildSecret1, buildSecrets[0].Name)
	assert.Equal(t, models.Consistent.String(), buildSecrets[0].Status)
}
