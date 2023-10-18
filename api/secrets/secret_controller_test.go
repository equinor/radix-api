package secrets

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/equinor/radix-api/api/events"
	secretModels "github.com/equinor/radix-api/api/secrets/models"
	controllertest "github.com/equinor/radix-api/api/test"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretsstorevclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	clusterName        = "AnyClusterName"
	anyAppName         = "any-app"
	anyComponentName   = "app"
	anyJobName         = "job"
	anyEnvironment     = "dev"
	anyEnvironmentName = "TEST_SECRET"
	egressIps          = "0.0.0.0"
	subscriptionId     = "12347718-c8f8-4995-bfbb-02655ff1f89c"
)

func setupTest() (*commontest.Utils, *controllertest.Utils, kubernetes.Interface, radixclient.Interface, prometheusclient.Interface, secretsstorevclient.Interface) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := fake.NewSimpleClientset()
	prometheusclient := prometheusfake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient, secretproviderclient)
	commonTestUtils.CreateClusterPrerequisites(clusterName, egressIps, subscriptionId)

	// secretControllerTestUtils is used for issuing HTTP request and processing responses
	secretControllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, secretproviderclient, NewSecretController())

	return &commonTestUtils, &secretControllerTestUtils, kubeclient, radixclient, prometheusclient, secretproviderclient
}

func executeUpdateComponentSecretTest(oldSecretValue, updateSecret, updateComponent, updateSecretName, updateSecretValue string) *httptest.ResponseRecorder {
	response := executeUpdateSecretTest(
		oldSecretValue,
		updateSecret,
		updateComponent,
		updateSecretName,
		updateSecretValue,
		configureApplicationComponentSecret)

	return response
}

func executeUpdateJobSecretTest(oldSecretValue, updateSecret, updateComponent, updateSecretName, updateSecretValue string) *httptest.ResponseRecorder {
	response := executeUpdateSecretTest(
		oldSecretValue,
		updateSecret,
		updateComponent,
		updateSecretName,
		updateSecretValue,
		configureApplicationJobSecret)

	return response
}

func configureApplicationComponentSecret(builder *operatorutils.ApplicationBuilder) {
	(*builder).WithComponents(
		operatorutils.AnApplicationComponent().
			WithName(anyComponentName).
			WithSecrets(anyEnvironmentName),
	)
}

func configureApplicationJobSecret(builder *operatorutils.ApplicationBuilder) {
	(*builder).WithJobComponents(
		operatorutils.AnApplicationJobComponent().
			WithName(anyJobName).
			WithSecrets(anyEnvironmentName),
	)
}

func executeUpdateSecretTest(oldSecretValue, updateSecret, updateComponent, updateSecretName, updateSecretValue string, appConfigurator func(builder *operatorutils.ApplicationBuilder)) *httptest.ResponseRecorder {

	// Setup
	parameters := secretModels.SecretParameters{
		SecretValue: updateSecretValue,
	}

	commonTestUtils, controllerTestUtils, kubeclient, _, _, _ := setupTest()
	appBuilder := operatorutils.
		ARadixApplication().
		WithAppName(anyAppName)
	appConfigurator(&appBuilder)

	commonTestUtils.ApplyApplication(appBuilder)
	ns := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}
	kubeclient.CoreV1().Namespaces().Create(context.Background(), &namespace, metav1.CreateOptions{})

	// Component secret
	secretObject := corev1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name: operatorutils.GetComponentSecretName(anyComponentName),
		},
		Data: map[string][]byte{anyEnvironmentName: []byte(oldSecretValue)},
	}
	kubeclient.CoreV1().Secrets(ns).Create(context.Background(), &secretObject, metav1.CreateOptions{})

	// Job secret
	secretObject = corev1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name: operatorutils.GetComponentSecretName(anyJobName),
		},
		Data: map[string][]byte{anyEnvironmentName: []byte(oldSecretValue)},
	}
	kubeclient.CoreV1().Secrets(ns).Create(context.Background(), &secretObject, metav1.CreateOptions{})

	// Test
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, updateSecret, updateComponent, updateSecretName), parameters)
	response := <-responseChannel
	return response
}

func TestUpdateSecret_OK(t *testing.T) {
	oldSecretValue := "oldvalue"
	updateSecretValue := "newvalue"

	response := executeUpdateComponentSecretTest(oldSecretValue, anyEnvironment, anyComponentName, anyEnvironmentName, updateSecretValue)
	assert.Equal(t, http.StatusOK, response.Code)

	response = executeUpdateJobSecretTest(oldSecretValue, anyEnvironment, anyJobName, anyEnvironmentName, updateSecretValue)
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestUpdateSecret_NonExistingEnvironment_Missing2(t *testing.T) {
	nonExistingSecretName := "TEST"
	oldSecretValue := "oldvalue"
	updateSecretValue := "newvalue"

	response := executeUpdateComponentSecretTest(oldSecretValue, anyEnvironment, anyComponentName, nonExistingSecretName, updateSecretValue)
	assert.Equal(t, http.StatusOK, response.Code)

	response = executeUpdateJobSecretTest(oldSecretValue, anyEnvironment, anyJobName, nonExistingSecretName, updateSecretValue)
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestUpdateSecret_EmptySecretValue_ValidationError(t *testing.T) {
	oldSecretValue := "oldvalue"
	updateSecretValue := ""

	response := executeUpdateComponentSecretTest(oldSecretValue, anyEnvironment, anyComponentName, anyEnvironmentName, updateSecretValue)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "New secret value is empty", errorResponse.Message)
	assert.Equal(t, "Secret failed validation", errorResponse.Err.Error())

	response = executeUpdateJobSecretTest(oldSecretValue, anyEnvironment, anyJobName, anyEnvironmentName, updateSecretValue)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "New secret value is empty", errorResponse.Message)
	assert.Equal(t, "Secret failed validation", errorResponse.Err.Error())
}

func TestUpdateSecret_NoUpdate_NoError(t *testing.T) {
	oldSecretValue := "oldvalue"
	updateSecretValue := "oldvalue"

	response := executeUpdateComponentSecretTest(oldSecretValue, anyEnvironment, anyComponentName, anyEnvironmentName, updateSecretValue)
	assert.Equal(t, http.StatusOK, response.Code)

	response = executeUpdateJobSecretTest(oldSecretValue, anyEnvironment, anyJobName, anyEnvironmentName, updateSecretValue)
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestUpdateSecret_NonExistingComponent_Missing(t *testing.T) {
	nonExistingComponent := "frontend"
	nonExistingSecretObjName := operatorutils.GetComponentSecretName(nonExistingComponent)
	oldSecretValue := "oldvalue"
	updateSecretValue := "newvalue"

	response := executeUpdateComponentSecretTest(oldSecretValue, anyEnvironment, nonExistingComponent, anyEnvironmentName, updateSecretValue)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, fmt.Sprintf("secrets \"%s\" not found", nonExistingSecretObjName), errorResponse.Err.Error())

	response = executeUpdateJobSecretTest(oldSecretValue, anyEnvironment, nonExistingComponent, anyEnvironmentName, updateSecretValue)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, fmt.Sprintf("secrets \"%s\" not found", nonExistingSecretObjName), errorResponse.Err.Error())
}

func TestUpdateSecret_NonExistingEnvironment_Missing(t *testing.T) {
	nonExistingSecret := "prod"
	oldSecretValue := "oldvalue"
	updateSecretValue := "newvalue"

	response := executeUpdateComponentSecretTest(oldSecretValue, nonExistingSecret, anyComponentName, anyEnvironmentName, updateSecretValue)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	secretObjName := operatorutils.GetComponentSecretName(anyComponentName)
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, fmt.Sprintf("secrets \"%s\" not found", secretObjName), errorResponse.Err.Error())

	response = executeUpdateJobSecretTest(oldSecretValue, nonExistingSecret, anyJobName, anyEnvironmentName, updateSecretValue)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	secretObjName = operatorutils.GetComponentSecretName(anyJobName)
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, fmt.Sprintf("secrets \"%s\" not found", secretObjName), errorResponse.Err.Error())
}
