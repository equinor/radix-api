package secrets

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "github.com/equinor/radix-api/api/events"
	"github.com/equinor/radix-api/api/secrets/models"
	controllertest "github.com/equinor/radix-api/api/test"
	apiModels "github.com/equinor/radix-api/models"
	radixmodels "github.com/equinor/radix-common/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	k8sObjectUtils "github.com/equinor/radix-operator/pkg/apis/utils"
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
	containerRegistry  = "any.container.registry"
	anyAppName         = "any-app"
	anyComponentName   = "app"
	anyJobName         = "job"
	anyEnvironment     = "dev"
	anyEnvironmentName = "TEST_SECRET"
	egressIps          = "0.0.0.0"
)

func setupTest() (*commontest.Utils, *controllertest.Utils, kubernetes.Interface, radixclient.Interface, prometheusclient.Interface, secretsstorevclient.Interface) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := fake.NewSimpleClientset()
	prometheusclient := prometheusfake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient, secretproviderclient)
	commonTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry, egressIps)

	// secretControllerTestUtils is used for issuing HTTP request and processing responses
	secretControllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, NewSecretController())

	return &commonTestUtils, &secretControllerTestUtils, kubeclient, radixclient, prometheusclient, secretproviderclient
}

func setupGetDeploymentsTest(commonTestUtils *commontest.Utils, appName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage string, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated time.Time, environment string) {
	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName(deploymentOneImage).
		WithAppName(appName).
		WithEnvironment(environment).
		WithImageTag(deploymentOneImage).
		WithCreated(deploymentOneCreated).
		WithCondition(v1.DeploymentInactive).
		WithActiveFrom(deploymentOneCreated).
		WithActiveTo(deploymentTwoCreated))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName(deploymentTwoImage).
		WithAppName(appName).
		WithEnvironment(environment).
		WithImageTag(deploymentTwoImage).
		WithCreated(deploymentTwoCreated).
		WithCondition(v1.DeploymentInactive).
		WithActiveFrom(deploymentTwoCreated).
		WithActiveTo(deploymentThreeCreated))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName(deploymentThreeImage).
		WithAppName(appName).
		WithEnvironment(environment).
		WithImageTag(deploymentThreeImage).
		WithCreated(deploymentThreeCreated).
		WithCondition(v1.DeploymentActive).
		WithActiveFrom(deploymentThreeCreated))
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

func configureApplicationComponentSecret(builder *k8sObjectUtils.ApplicationBuilder) {
	(*builder).WithComponents(
		builders.AnApplicationComponent().
			WithName(anyComponentName).
			WithSecrets(anyEnvironmentName),
	)
}

func configureApplicationJobSecret(builder *k8sObjectUtils.ApplicationBuilder) {
	(*builder).WithJobComponents(
		builders.AnApplicationJobComponent().
			WithName(anyJobName).
			WithSecrets(anyEnvironmentName),
	)
}

func executeUpdateSecretTest(oldSecretValue, updateSecret, updateComponent, updateSecretName, updateSecretValue string, appConfigurator func(builder *k8sObjectUtils.ApplicationBuilder)) *httptest.ResponseRecorder {

	// Setup
	parameters := models.SecretParameters{
		SecretValue: updateSecretValue,
	}

	commonTestUtils, controllerTestUtils, kubeclient, _, _, _ := setupTest()
	appBuilder := builders.
		ARadixApplication().
		WithAppName(anyAppName)
	appConfigurator(&appBuilder)

	commonTestUtils.ApplyApplication(appBuilder)
	ns := k8sObjectUtils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}
	kubeclient.CoreV1().Namespaces().Create(context.TODO(), &namespace, metav1.CreateOptions{})

	// Component secret
	secretObject := corev1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name: k8sObjectUtils.GetComponentSecretName(anyComponentName),
		},
		Data: map[string][]byte{anyEnvironmentName: []byte(oldSecretValue)},
	}
	kubeclient.CoreV1().Secrets(ns).Create(context.TODO(), &secretObject, metav1.CreateOptions{})

	// Job secret
	secretObject = corev1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name: k8sObjectUtils.GetComponentSecretName(anyJobName),
		},
		Data: map[string][]byte{anyEnvironmentName: []byte(oldSecretValue)},
	}
	kubeclient.CoreV1().Secrets(ns).Create(context.TODO(), &secretObject, metav1.CreateOptions{})

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
	nonExistingSecretObjName := k8sObjectUtils.GetComponentSecretName(nonExistingComponent)
	oldSecretValue := "oldvalue"
	updateSecretValue := "newvalue"

	response := executeUpdateComponentSecretTest(oldSecretValue, anyEnvironment, nonExistingComponent, anyEnvironmentName, updateSecretValue)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, "Secret object does not exist", errorResponse.Message)
	assert.Equal(t, fmt.Sprintf("secrets \"%s\" not found", nonExistingSecretObjName), errorResponse.Err.Error())

	response = executeUpdateJobSecretTest(oldSecretValue, anyEnvironment, nonExistingComponent, anyEnvironmentName, updateSecretValue)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, "Secret object does not exist", errorResponse.Message)
	assert.Equal(t, fmt.Sprintf("secrets \"%s\" not found", nonExistingSecretObjName), errorResponse.Err.Error())
}

func TestUpdateSecret_NonExistingEnvironment_Missing(t *testing.T) {
	nonExistingSecret := "prod"
	oldSecretValue := "oldvalue"
	updateSecretValue := "newvalue"

	response := executeUpdateComponentSecretTest(oldSecretValue, nonExistingSecret, anyComponentName, anyEnvironmentName, updateSecretValue)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	secretObjName := k8sObjectUtils.GetComponentSecretName(anyComponentName)
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, "Secret object does not exist", errorResponse.Message)
	assert.Equal(t, fmt.Sprintf("secrets \"%s\" not found", secretObjName), errorResponse.Err.Error())

	response = executeUpdateJobSecretTest(oldSecretValue, nonExistingSecret, anyJobName, anyEnvironmentName, updateSecretValue)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	secretObjName = k8sObjectUtils.GetComponentSecretName(anyJobName)
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, "Secret object does not exist", errorResponse.Message)
	assert.Equal(t, fmt.Sprintf("secrets \"%s\" not found", secretObjName), errorResponse.Err.Error())
}

func componentBuilderFromSecretMap(secretsMap map[string][]string) func(*k8sObjectUtils.DeploymentBuilder) {
	return func(deployBuilder *k8sObjectUtils.DeploymentBuilder) {
		componentBuilders := make([]builders.DeployComponentBuilder, 0, len(secretsMap))
		for componentName, componentSecrets := range secretsMap {
			component := builders.
				NewDeployComponentBuilder().
				WithName(componentName).
				WithSecrets(componentSecrets)
			componentBuilders = append(componentBuilders, component)
		}
		(*deployBuilder).WithComponents(componentBuilders...)
	}
}

func jobBuilderFromSecretMap(secretsMap map[string][]string) func(*k8sObjectUtils.DeploymentBuilder) {
	return func(deployBuilder *k8sObjectUtils.DeploymentBuilder) {
		jobBuilders := make([]builders.DeployJobComponentBuilder, 0, len(secretsMap))
		for jobName, jobSecret := range secretsMap {
			job := builders.
				NewDeployJobComponentBuilder().
				WithName(jobName).
				WithSecrets(jobSecret)
			jobBuilders = append(jobBuilders, job)
		}
		(*deployBuilder).WithJobComponents(jobBuilders...)
	}
}

func applyTestSecretComponentSecrets(commonTestUtils *commontest.Utils, kubeclient kubernetes.Interface, appName, environmentName, buildFrom string, componentSecretsMap map[string][]string, clusterComponentSecretsMap map[string]map[string][]byte) {
	configurator := componentBuilderFromSecretMap(componentSecretsMap)
	applyTestSecretSecrets(commonTestUtils, kubeclient, appName, environmentName, buildFrom, clusterComponentSecretsMap, configurator)
}

func applyTestSecretJobSecrets(commonTestUtils *commontest.Utils, kubeclient kubernetes.Interface, appName, environmentName, buildFrom string, componentSecretsMap map[string][]string, clusterComponentSecretsMap map[string]map[string][]byte) {
	configurator := jobBuilderFromSecretMap(componentSecretsMap)
	applyTestSecretSecrets(commonTestUtils, kubeclient, appName, environmentName, buildFrom, clusterComponentSecretsMap, configurator)
}

func applyTestSecretSecrets(commonTestUtils *commontest.Utils, kubeclient kubernetes.Interface, appName, environmentName, buildFrom string, clusterComponentSecretsMap map[string]map[string][]byte, deploymentConfigurator func(*k8sObjectUtils.DeploymentBuilder)) {
	ns := k8sObjectUtils.GetEnvironmentNamespace(appName, environmentName)

	deployBuilder := builders.
		NewDeploymentBuilder().
		WithRadixApplication(builders.ARadixApplication()).
		WithAppName(anyAppName).
		WithEnvironment(environmentName)

	deploymentConfigurator(&deployBuilder)

	commonTestUtils.ApplyDeployment(deployBuilder)

	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}
	kubeclient.CoreV1().Namespaces().Create(context.TODO(), &namespace, metav1.CreateOptions{})

	for componentName, clusterComponentSecrets := range clusterComponentSecretsMap {
		secretObject := corev1.Secret{
			Type: "Opaque",
			ObjectMeta: metav1.ObjectMeta{
				Name: k8sObjectUtils.GetComponentSecretName(componentName),
			},
			Data: clusterComponentSecrets,
		}
		kubeclient.CoreV1().Secrets(ns).Create(context.TODO(), &secretObject, metav1.CreateOptions{})
	}
}

func assertSecretObject(t *testing.T, secretObject models.Secret, name, component, status, testname string) {
	assert.Equal(t, name, secretObject.Name, testname, fmt.Sprintf("%s: incorrect secret name", testname))
	assert.Equal(t, component, secretObject.Component, fmt.Sprintf("%s: incorrect component name", testname))
	assert.Equal(t, status, secretObject.Status, fmt.Sprintf("%s: incorrect secret status", testname))
}

type secretTestFunc func(commonTestUtils *commontest.Utils, kubeclient kubernetes.Interface, appName, environmentName, buildFrom string, componentSecretsMap map[string][]string, clusterComponentSecretsMap map[string]map[string][]byte)

type secretTestDefinition struct {
	name   string
	tester secretTestFunc
}

var secretTestFunctions []secretTestDefinition = []secretTestDefinition{
	{name: "component secrets", tester: applyTestSecretComponentSecrets},
	{name: "job secrets", tester: applyTestSecretJobSecrets},
}

func TestGetSecrets_OneComponent_AllConsistent(t *testing.T) {
	for _, test := range secretTestFunctions {
		commonTestUtils, _, kubeclient, radixclient, _, secretproviderclient := setupTest()
		handler := initHandler(kubeclient, radixclient, secretproviderclient)

		appName := "any-app"
		componentOneName := "backend"
		environmentOne := "dev"
		buildFrom := "master"
		secretA := "a"
		secretB := "b"
		secretC := "c"

		componentSecrets := []string{secretA, secretB, secretC}
		componentSecretsMap := map[string][]string{
			componentOneName: componentSecrets,
		}

		clusterComponentSecrets := map[string][]byte{
			secretA: []byte(secretA),
			secretB: []byte(secretB),
			secretC: []byte(secretC),
		}
		clusterComponentSecretsMap := map[string]map[string][]byte{
			componentOneName: clusterComponentSecrets,
		}

		test.tester(commonTestUtils, kubeclient, appName, environmentOne, buildFrom, componentSecretsMap, clusterComponentSecretsMap)

		secrets, _ := handler.GetSecrets(appName, environmentOne)

		assert.Equal(t, 3, len(secrets), fmt.Sprintf("%s: incorrect secret count", test.name))
		for _, aSecret := range secrets {
			if aSecret.Name == secretA {
				assertSecretObject(t, aSecret, secretA, componentOneName, "Consistent", test.name)
			}
			if aSecret.Name == secretB {
				assertSecretObject(t, aSecret, secretB, componentOneName, "Consistent", test.name)
			}
			if aSecret.Name == secretC {
				assertSecretObject(t, aSecret, secretC, componentOneName, "Consistent", test.name)
			}
		}
	}
}

func TestGetSecrets_OneComponent_PartiallyConsistent(t *testing.T) {
	for _, test := range secretTestFunctions {
		commonTestUtils, _, kubeclient, radixclient, _, secretproviderclient := setupTest()
		handler := initHandler(kubeclient, radixclient, secretproviderclient)

		appName := "any-app"
		componentOneName := "backend"
		environmentOne := "dev"
		buildFrom := "master"
		secretA := "a"
		secretB := "b"
		secretC := "c"
		secretD := "d"

		componentSecrets := []string{secretA, secretB, secretC}
		componentSecretsMap := map[string][]string{
			componentOneName: componentSecrets,
		}

		clusterComponentSecrets := map[string][]byte{
			secretB: []byte(secretB),
			secretC: []byte(secretC),
			secretD: []byte(secretD),
		}
		clusterComponentSecretsMap := map[string]map[string][]byte{
			componentOneName: clusterComponentSecrets,
		}

		test.tester(commonTestUtils, kubeclient, appName, environmentOne, buildFrom, componentSecretsMap, clusterComponentSecretsMap)

		secrets, _ := handler.GetSecrets(appName, environmentOne)

		assert.Equal(t, 4, len(secrets), fmt.Sprintf("%s: incorrect secret count", test.name))
		for _, aSecret := range secrets {
			if aSecret.Name == secretA {
				assertSecretObject(t, aSecret, secretA, componentOneName, "Pending", test.name)
			}
			if aSecret.Name == secretB {
				assertSecretObject(t, aSecret, secretB, componentOneName, "Consistent", test.name)
			}
			if aSecret.Name == secretC {
				assertSecretObject(t, aSecret, secretC, componentOneName, "Consistent", test.name)
			}
			if aSecret.Name == secretD {
				assertSecretObject(t, aSecret, secretD, componentOneName, "Orphan", test.name)
			}
		}
	}
}

func TestGetSecrets_OneComponent_NoConsistent(t *testing.T) {
	for _, test := range secretTestFunctions {
		commonTestUtils, _, kubeclient, radixclient, _, secretproviderclient := setupTest()
		handler := initHandler(kubeclient, radixclient, secretproviderclient)

		appName := "any-app"
		componentOneName := "backend"
		environmentOne := "dev"
		buildFrom := "master"
		secretA := "a"
		secretB := "b"
		secretC := "c"
		secretD := "d"
		secretE := "e"
		secretF := "f"

		componentSecrets := []string{secretA, secretB, secretC}
		componentSecretsMap := map[string][]string{
			componentOneName: componentSecrets,
		}

		clusterComponentSecrets := map[string][]byte{
			secretD: []byte(secretD),
			secretE: []byte(secretE),
			secretF: []byte(secretF),
		}
		clusterComponentSecretsMap := map[string]map[string][]byte{
			componentOneName: clusterComponentSecrets,
		}

		test.tester(commonTestUtils, kubeclient, appName, environmentOne, buildFrom, componentSecretsMap, clusterComponentSecretsMap)

		secrets, _ := handler.GetSecrets(appName, environmentOne)

		assert.Equal(t, 6, len(secrets), fmt.Sprintf("%s: incorrect secret count", test.name))
		for _, aSecret := range secrets {
			if aSecret.Name == secretA {
				assertSecretObject(t, aSecret, secretA, componentOneName, "Pending", test.name)
			}
			if aSecret.Name == secretB {
				assertSecretObject(t, aSecret, secretB, componentOneName, "Pending", test.name)
			}
			if aSecret.Name == secretC {
				assertSecretObject(t, aSecret, secretC, componentOneName, "Pending", test.name)
			}
			if aSecret.Name == secretD {
				assertSecretObject(t, aSecret, secretD, componentOneName, "Orphan", test.name)
			}
			if aSecret.Name == secretE {
				assertSecretObject(t, aSecret, secretE, componentOneName, "Orphan", test.name)
			}
			if aSecret.Name == secretF {
				assertSecretObject(t, aSecret, secretF, componentOneName, "Orphan", test.name)
			}
		}
	}
}

func TestGetSecrets_TwoComponents_AllConsistent(t *testing.T) {
	for _, test := range secretTestFunctions {
		commonTestUtils, _, kubeclient, radixclient, _, secretproviderclient := setupTest()
		handler := initHandler(kubeclient, radixclient, secretproviderclient)

		appName := "any-app"
		componentOneName := "backend"
		componentTwoName := "frontend"
		environmentOne := "dev"
		buildFrom := "master"
		secretA := "a"
		secretB := "b"
		secretC := "c"

		componentOneSecrets := []string{secretA, secretB, secretC}
		componentTwoSecrets := []string{secretA, secretB, secretC}
		componentSecretsMap := map[string][]string{
			componentOneName: componentOneSecrets,
			componentTwoName: componentTwoSecrets,
		}

		clusterComponentOneSecrets := map[string][]byte{
			secretA: []byte(secretA),
			secretB: []byte(secretB),
			secretC: []byte(secretC),
		}
		clusterComponentTwoSecrets := map[string][]byte{
			secretA: []byte(secretA),
			secretB: []byte(secretB),
			secretC: []byte(secretC),
		}
		clusterComponentSecretsMap := map[string]map[string][]byte{
			componentOneName: clusterComponentOneSecrets,
			componentTwoName: clusterComponentTwoSecrets,
		}

		test.tester(commonTestUtils, kubeclient, appName, environmentOne, buildFrom, componentSecretsMap, clusterComponentSecretsMap)

		secrets, _ := handler.GetSecrets(appName, environmentOne)

		assert.Equal(t, 6, len(secrets), fmt.Sprintf("%s: incorrect secret count", test.name))
		for _, aSecret := range secrets {
			if aSecret.Component == componentOneName && aSecret.Name == secretA {
				assertSecretObject(t, aSecret, secretA, componentOneName, "Consistent", test.name)
			}
			if aSecret.Component == componentOneName && aSecret.Name == secretB {
				assertSecretObject(t, aSecret, secretB, componentOneName, "Consistent", test.name)
			}
			if aSecret.Component == componentOneName && aSecret.Name == secretC {
				assertSecretObject(t, aSecret, secretC, componentOneName, "Consistent", test.name)
			}
			if aSecret.Component == componentTwoName && aSecret.Name == secretA {
				assertSecretObject(t, aSecret, secretA, componentTwoName, "Consistent", test.name)
			}
			if aSecret.Component == componentTwoName && aSecret.Name == secretB {
				assertSecretObject(t, aSecret, secretB, componentTwoName, "Consistent", test.name)
			}
			if aSecret.Component == componentTwoName && aSecret.Name == secretC {
				assertSecretObject(t, aSecret, secretC, componentTwoName, "Consistent", test.name)
			}
		}
	}
}

func TestGetSecrets_TwoComponents_PartiallyConsistent(t *testing.T) {
	for _, test := range secretTestFunctions {
		commonTestUtils, _, kubeclient, radixclient, _, secretproviderclient := setupTest()
		handler := initHandler(kubeclient, radixclient, secretproviderclient)

		appName := "any-app"
		componentOneName := "backend"
		componentTwoName := "frontend"
		environmentOne := "dev"
		buildFrom := "master"
		secretA := "a"
		secretB := "b"
		secretC := "c"
		secretD := "d"

		componentOneSecrets := []string{secretA, secretB, secretC}
		componentTwoSecrets := []string{secretA, secretB, secretC}
		componentSecretsMap := map[string][]string{
			componentOneName: componentOneSecrets,
			componentTwoName: componentTwoSecrets,
		}

		clusterComponentOneSecrets := map[string][]byte{
			secretB: []byte(secretB),
			secretC: []byte(secretC),
			secretD: []byte(secretD),
		}
		clusterComponentTwoSecrets := map[string][]byte{
			secretB: []byte(secretB),
			secretC: []byte(secretC),
			secretD: []byte(secretD),
		}
		clusterComponentSecretsMap := map[string]map[string][]byte{
			componentOneName: clusterComponentOneSecrets,
			componentTwoName: clusterComponentTwoSecrets,
		}

		test.tester(commonTestUtils, kubeclient, appName, environmentOne, buildFrom, componentSecretsMap, clusterComponentSecretsMap)

		secrets, _ := handler.GetSecrets(appName, environmentOne)

		assert.Equal(t, 8, len(secrets), fmt.Sprintf("%s: incorrect secret count", test.name))
		for _, aSecret := range secrets {
			if aSecret.Component == componentOneName && aSecret.Name == secretA {
				assertSecretObject(t, aSecret, secretA, componentOneName, "Pending", test.name)
			}
			if aSecret.Component == componentOneName && aSecret.Name == secretB {
				assertSecretObject(t, aSecret, secretB, componentOneName, "Consistent", test.name)
			}
			if aSecret.Component == componentOneName && aSecret.Name == secretC {
				assertSecretObject(t, aSecret, secretC, componentOneName, "Consistent", test.name)
			}
			if aSecret.Component == componentOneName && aSecret.Name == secretD {
				assertSecretObject(t, aSecret, secretD, componentOneName, "Orphan", test.name)
			}

			if aSecret.Component == componentTwoName && aSecret.Name == secretA {
				assertSecretObject(t, aSecret, secretA, componentTwoName, "Pending", test.name)
			}
			if aSecret.Component == componentTwoName && aSecret.Name == secretB {
				assertSecretObject(t, aSecret, secretB, componentTwoName, "Consistent", test.name)
			}
			if aSecret.Component == componentTwoName && aSecret.Name == secretC {
				assertSecretObject(t, aSecret, secretC, componentTwoName, "Consistent", test.name)
			}
			if aSecret.Component == componentTwoName && aSecret.Name == secretD {
				assertSecretObject(t, aSecret, secretD, componentTwoName, "Orphan", test.name)
			}
		}
	}
}

func TestGetSecrets_TwoComponents_NoConsistent(t *testing.T) {
	for _, test := range secretTestFunctions {
		commonTestUtils, _, kubeclient, radixclient, _, secretproviderclient := setupTest()
		handler := initHandler(kubeclient, radixclient, secretproviderclient)

		appName := "any-app"
		componentOneName := "backend"
		componentTwoName := "frontend"
		environmentOne := "dev"
		buildFrom := "master"
		secretA := "a"
		secretB := "b"
		secretC := "c"
		secretD := "d"
		secretE := "e"
		secretF := "f"

		componentOneSecrets := []string{secretA, secretB, secretC}
		componentTwoSecrets := []string{secretA, secretB, secretC}
		componentSecretsMap := map[string][]string{
			componentOneName: componentOneSecrets,
			componentTwoName: componentTwoSecrets,
		}

		clusterComponentOneSecrets := map[string][]byte{
			secretD: []byte(secretD),
			secretE: []byte(secretE),
			secretF: []byte(secretF),
		}
		clusterComponentTwoSecrets := map[string][]byte{
			secretD: []byte(secretD),
			secretE: []byte(secretE),
			secretF: []byte(secretF),
		}
		clusterComponentSecretsMap := map[string]map[string][]byte{
			componentOneName: clusterComponentOneSecrets,
			componentTwoName: clusterComponentTwoSecrets,
		}

		test.tester(commonTestUtils, kubeclient, appName, environmentOne, buildFrom, componentSecretsMap, clusterComponentSecretsMap)

		secrets, _ := handler.GetSecrets(appName, environmentOne)

		assert.Equal(t, 12, len(secrets), fmt.Sprintf("%s: incorrect secret count", test.name))
		for _, aSecret := range secrets {
			if aSecret.Component == componentOneName && aSecret.Name == secretA {
				assertSecretObject(t, aSecret, secretA, componentOneName, "Pending", test.name)
			}
			if aSecret.Component == componentOneName && aSecret.Name == secretB {
				assertSecretObject(t, aSecret, secretB, componentOneName, "Pending", test.name)
			}
			if aSecret.Component == componentOneName && aSecret.Name == secretC {
				assertSecretObject(t, aSecret, secretC, componentOneName, "Pending", test.name)
			}
			if aSecret.Component == componentOneName && aSecret.Name == secretD {
				assertSecretObject(t, aSecret, secretD, componentOneName, "Orphan", test.name)
			}
			if aSecret.Component == componentOneName && aSecret.Name == secretE {
				assertSecretObject(t, aSecret, secretE, componentOneName, "Orphan", test.name)
			}
			if aSecret.Component == componentOneName && aSecret.Name == secretF {
				assertSecretObject(t, aSecret, secretF, componentOneName, "Orphan", test.name)
			}

			if aSecret.Component == componentTwoName && aSecret.Name == secretA {
				assertSecretObject(t, aSecret, secretA, componentTwoName, "Pending", test.name)
			}
			if aSecret.Component == componentTwoName && aSecret.Name == secretB {
				assertSecretObject(t, aSecret, secretB, componentTwoName, "Pending", test.name)
			}
			if aSecret.Component == componentTwoName && aSecret.Name == secretC {
				assertSecretObject(t, aSecret, secretC, componentTwoName, "Pending", test.name)
			}
			if aSecret.Component == componentTwoName && aSecret.Name == secretD {
				assertSecretObject(t, aSecret, secretD, componentTwoName, "Orphan", test.name)
			}
			if aSecret.Component == componentTwoName && aSecret.Name == secretE {
				assertSecretObject(t, aSecret, secretE, componentTwoName, "Orphan", test.name)
			}
			if aSecret.Component == componentTwoName && aSecret.Name == secretF {
				assertSecretObject(t, aSecret, secretF, componentTwoName, "Orphan", test.name)
			}
		}
	}
}

func TestGetSecretsForDeploymentForExternalAlias(t *testing.T) {
	commonTestUtils, _, kubeclient, radixclient, _, secretproviderclient := setupTest()
	handler := initHandler(kubeclient, radixclient, secretproviderclient)

	appName := "any-app"
	componentName := "backend"
	environmentName := "dev"
	buildFrom := "master"
	alias := "cdn.myalias.com"

	deployment, err := commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(environmentName).
		WithImageTag(buildFrom))

	commonTestUtils.ApplyApplication(builders.
		ARadixApplication().
		WithAppName(appName).
		WithEnvironment(environmentName, buildFrom).
		WithComponent(builders.
			AnApplicationComponent().
			WithName(componentName)).
		WithDNSExternalAlias(alias, environmentName, componentName))

	secrets, err := handler.GetSecretsForDeployment(appName, environmentName, deployment.Name)

	assert.NoError(t, err)
	assert.Len(t, secrets, 2)
	for _, s := range secrets {
		if s.Name == alias+"-key" {
			assert.Equal(t, "Pending", s.Status)
		} else if s.Name == alias+"-cert" {
			assert.Equal(t, "Pending", s.Status)
		}
	}

	kubeclient.CoreV1().Secrets(appName+"-"+environmentName).Create(context.TODO(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: alias,
		},
	}, metav1.CreateOptions{})

	secrets, err = handler.GetSecretsForDeployment(appName, environmentName, deployment.Name)

	assert.NoError(t, err)
	assert.Len(t, secrets, 2)
	for _, s := range secrets {
		if s.Name == alias+"-key" {
			assert.Equal(t, "Consistent", s.Status)
		} else if s.Name == alias+"-cert" {
			assert.Equal(t, "Consistent", s.Status)
		}
	}
}

func initHandler(client kubernetes.Interface,
	radixclient radixclient.Interface,
	secretproviderclient secretsstorevclient.Interface,
	handlerConfig ...SecretHandlerOptions) SecretHandler {
	accounts := apiModels.NewAccounts(client, radixclient, secretproviderclient, client, radixclient, secretproviderclient, "", radixmodels.Impersonation{})
	options := []SecretHandlerOptions{WithAccounts(accounts)}
	options = append(options, handlerConfig...)
	return Init(options...)
}

func createComponentPod(kubeclient kubernetes.Interface, namespace, componentName string) {
	podSpec := getPodSpec(componentName)
	kubeclient.CoreV1().Pods(namespace).Create(context.TODO(), podSpec, metav1.CreateOptions{})
}

func deleteComponentPod(kubeclient kubernetes.Interface, namespace, componentName string) {
	kubeclient.CoreV1().Pods(namespace).Delete(context.TODO(), getComponentPodName(componentName), metav1.DeleteOptions{})
}

func getPodSpec(componentName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: getComponentPodName(componentName),
			Labels: map[string]string{
				kube.RadixComponentLabel: componentName,
			},
		},
	}
}

func getComponentPodName(componentName string) string {
	return fmt.Sprintf("%s-pod", componentName)
}
