package secrets

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/equinor/radix-api/api/events"
	"github.com/equinor/radix-api/api/secrets/models"
	"github.com/equinor/radix-api/api/secrets/suffix"
	controllertest "github.com/equinor/radix-api/api/test"
	apiModels "github.com/equinor/radix-api/models"
	radixmodels "github.com/equinor/radix-common/models"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
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
	containerRegistry  = "any.container.registry"
	anyAppName         = "any-app"
	anyComponentName   = "app"
	anyJobName         = "job"
	anyEnvironment     = "dev"
	anyEnvironmentName = "TEST_SECRET"
	egressIps          = "0.0.0.0"
)

type componentProps struct {
	componentName string
	secrets       []string
}

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
	parameters := models.SecretParameters{
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
	kubeclient.CoreV1().Namespaces().Create(context.TODO(), &namespace, metav1.CreateOptions{})

	// Component secret
	secretObject := corev1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name: operatorutils.GetComponentSecretName(anyComponentName),
		},
		Data: map[string][]byte{anyEnvironmentName: []byte(oldSecretValue)},
	}
	kubeclient.CoreV1().Secrets(ns).Create(context.TODO(), &secretObject, metav1.CreateOptions{})

	// Job secret
	secretObject = corev1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name: operatorutils.GetComponentSecretName(anyJobName),
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
	nonExistingSecretObjName := operatorutils.GetComponentSecretName(nonExistingComponent)
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
	secretObjName := operatorutils.GetComponentSecretName(anyComponentName)
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, "Secret object does not exist", errorResponse.Message)
	assert.Equal(t, fmt.Sprintf("secrets \"%s\" not found", secretObjName), errorResponse.Err.Error())

	response = executeUpdateJobSecretTest(oldSecretValue, nonExistingSecret, anyJobName, anyEnvironmentName, updateSecretValue)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	secretObjName = operatorutils.GetComponentSecretName(anyJobName)
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, "Secret object does not exist", errorResponse.Message)
	assert.Equal(t, fmt.Sprintf("secrets \"%s\" not found", secretObjName), errorResponse.Err.Error())
}

func componentBuilderFromSecretMap(secretsMap map[string][]string) func(*operatorutils.DeploymentBuilder) {
	return func(deployBuilder *operatorutils.DeploymentBuilder) {
		componentBuilders := make([]operatorutils.DeployComponentBuilder, 0, len(secretsMap))
		for componentName, componentSecrets := range secretsMap {
			component := operatorutils.
				NewDeployComponentBuilder().
				WithName(componentName).
				WithSecrets(componentSecrets)
			componentBuilders = append(componentBuilders, component)
		}
		(*deployBuilder).WithComponents(componentBuilders...)
	}
}

func jobBuilderFromSecretMap(secretsMap map[string][]string) func(*operatorutils.DeploymentBuilder) {
	return func(deployBuilder *operatorutils.DeploymentBuilder) {
		jobBuilders := make([]operatorutils.DeployJobComponentBuilder, 0, len(secretsMap))
		for jobName, jobSecret := range secretsMap {
			job := operatorutils.
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

func applyTestSecretSecrets(commonTestUtils *commontest.Utils, kubeclient kubernetes.Interface, appName, environmentName, buildFrom string, clusterComponentSecretsMap map[string]map[string][]byte, deploymentConfigurator func(*operatorutils.DeploymentBuilder)) {
	ns := operatorutils.GetEnvironmentNamespace(appName, environmentName)

	deployBuilder := operatorutils.
		NewDeploymentBuilder().
		WithRadixApplication(operatorutils.ARadixApplication()).
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
				Name: operatorutils.GetComponentSecretName(componentName),
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
		deploymentName := "deployment1"
		createDeployment(radixclient, appName, environmentOne, deploymentName, componentProps{
			componentName: componentOneName, secrets: []string{secretA, secretB, secretC},
		})

		secrets, _ := handler.GetSecretsForDeployment(appName, environmentOne, deploymentName)

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

func createDeployment(radixclient radixclient.Interface, appName, environmentOne, deploymentName string, componentSecrets ...componentProps) {
	radixDeployment := v1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: deploymentName},
		Spec: v1.RadixDeploymentSpec{
			Environment: environmentOne,
		},
	}
	for _, componentSecret := range componentSecrets {
		radixDeployment.Spec.Components = append(radixDeployment.Spec.Components, v1.RadixDeployComponent{
			Name:    componentSecret.componentName,
			Secrets: componentSecret.secrets,
		},
		)
	}
	appEnvNamespace := operatorutils.GetEnvironmentNamespace(appName, environmentOne)
	_, _ = radixclient.RadixV1().RadixDeployments(appEnvNamespace).Create(context.Background(), &radixDeployment, metav1.CreateOptions{})
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
		deploymentName := "deployment1"
		createDeployment(radixclient, appName, environmentOne, deploymentName, componentProps{
			componentName: componentOneName, secrets: []string{secretA, secretB, secretC},
		})

		secrets, _ := handler.GetSecretsForDeployment(appName, environmentOne, deploymentName)

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
		deploymentName := "deployment1"
		createDeployment(radixclient, appName, environmentOne, deploymentName, componentProps{
			componentName: componentOneName, secrets: []string{secretA, secretB, secretC},
		})

		secrets, _ := handler.GetSecretsForDeployment(appName, environmentOne, deploymentName)

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

		deploymentName := "deployment1"
		createDeployment(radixclient, appName, environmentOne, deploymentName,
			componentProps{
				componentName: componentOneName, secrets: []string{secretA, secretB, secretC},
			},
			componentProps{
				componentName: componentTwoName, secrets: []string{secretA, secretB, secretC},
			})

		secrets, _ := handler.GetSecretsForDeployment(appName, environmentOne, deploymentName)

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

		deploymentName := "deployment1"
		createDeployment(radixclient, appName, environmentOne, deploymentName,
			componentProps{
				componentName: componentOneName, secrets: []string{secretA, secretB, secretC},
			},
			componentProps{
				componentName: componentTwoName, secrets: []string{secretA, secretB, secretC},
			})

		secrets, _ := handler.GetSecretsForDeployment(appName, environmentOne, deploymentName)

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

		deploymentName := "deployment1"
		createDeployment(radixclient, appName, environmentOne, deploymentName,
			componentProps{
				componentName: componentOneName, secrets: []string{secretA, secretB, secretC},
			},
			componentProps{
				componentName: componentTwoName, secrets: []string{secretA, secretB, secretC},
			})

		secrets, _ := handler.GetSecretsForDeployment(appName, environmentOne, deploymentName)

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

	deployment, _ := commonTestUtils.ApplyDeployment(operatorutils.
		ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(environmentName).
		WithComponents(operatorutils.NewDeployComponentBuilder().WithName(componentName).WithDNSExternalAlias(alias)).
		WithImageTag(buildFrom))

	commonTestUtils.ApplyApplication(operatorutils.
		ARadixApplication().
		WithAppName(appName).
		WithEnvironment(environmentName, buildFrom).
		WithComponents(operatorutils.
			AnApplicationComponent().
			WithName(componentName)))

	secrets, err := handler.GetSecretsForDeployment(appName, environmentName, deployment.Name)

	assert.NoError(t, err)
	expectedSecrets := []models.Secret{
		{Name: alias + "-key", DisplayName: "Key", Status: models.Pending.String(), Resource: alias, Type: models.SecretTypeClientCert, Component: componentName},
		{Name: alias + "-cert", DisplayName: "Certificate", Status: models.Pending.String(), Resource: alias, Type: models.SecretTypeClientCert, Component: componentName},
	}
	assert.ElementsMatch(t, expectedSecrets, secrets)
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
	expectedSecrets[0].Status = models.Consistent.String()
	expectedSecrets[1].Status = models.Consistent.String()
	assert.ElementsMatch(t, expectedSecrets, secrets)
}

func Test_GetSecretsForDeployment_OAuth2(t *testing.T) {
	commonTestUtils, _, kubeclient, radixclient, _, secretproviderclient := setupTest()
	handler := initHandler(kubeclient, radixclient, secretproviderclient)

	appName := "appname"
	component1Name := "c1"
	component2Name := "c2"
	component3Name := "c3"
	component4Name := "c4"
	environmentName := "dev"
	envNs := operatorutils.GetEnvironmentNamespace(appName, environmentName)

	deployment, _ := commonTestUtils.ApplyDeployment(operatorutils.
		NewDeploymentBuilder().
		WithAppName(appName).
		WithComponents(
			operatorutils.NewDeployComponentBuilder().WithName(component1Name).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}}),
			operatorutils.NewDeployComponentBuilder().WithName(component2Name).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{SessionStoreType: v1.SessionStoreRedis}}),
			operatorutils.NewDeployComponentBuilder().WithName(component3Name).WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}}),
			operatorutils.NewDeployComponentBuilder().WithName(component4Name).WithPublicPort("http"),
		).
		WithEnvironment(environmentName))

	commonTestUtils.ApplyApplication(operatorutils.
		ARadixApplication().
		WithAppName(appName).
		WithEnvironment(environmentName, "branch1").
		WithComponents(
			operatorutils.NewApplicationComponentBuilder().WithName(component1Name),
			operatorutils.NewApplicationComponentBuilder().WithName(component2Name),
			operatorutils.NewApplicationComponentBuilder().WithName(component3Name),
			operatorutils.NewApplicationComponentBuilder().WithName(component4Name),
		))

	// No secret objects exist
	secretDtos, err := handler.GetSecretsForDeployment(appName, environmentName, deployment.Name)
	assert.NoError(t, err)
	expected := []models.Secret{
		{Name: component1Name + suffix.OAuth2ClientSecret, DisplayName: "Client Secret", Type: models.SecretTypeOAuth2Proxy, Component: component1Name, Status: models.Pending.String()},
		{Name: component1Name + suffix.OAuth2CookieSecret, DisplayName: "Cookie Secret", Type: models.SecretTypeOAuth2Proxy, Component: component1Name, Status: models.Pending.String()},
		{Name: component2Name + suffix.OAuth2ClientSecret, DisplayName: "Client Secret", Type: models.SecretTypeOAuth2Proxy, Component: component2Name, Status: models.Pending.String()},
		{Name: component2Name + suffix.OAuth2CookieSecret, DisplayName: "Cookie Secret", Type: models.SecretTypeOAuth2Proxy, Component: component2Name, Status: models.Pending.String()},
		{Name: component2Name + suffix.OAuth2RedisPassword, DisplayName: "Redis Password", Type: models.SecretTypeOAuth2Proxy, Component: component2Name, Status: models.Pending.String()},
	}
	assert.ElementsMatch(t, expected, secretDtos)

	// k8s secrets with clientsecret set for component1 and cookiesecret set for component2
	kubeclient.CoreV1().Secrets(envNs).Create(
		context.Background(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: operatorutils.GetAuxiliaryComponentSecretName(component1Name, defaults.OAuthProxyAuxiliaryComponentSuffix)},
			Data:       map[string][]byte{defaults.OAuthClientSecretKeyName: []byte("client secret")},
		},
		metav1.CreateOptions{},
	)
	comp2Secret, _ := kubeclient.CoreV1().Secrets(envNs).Create(
		context.Background(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: operatorutils.GetAuxiliaryComponentSecretName(component2Name, defaults.OAuthProxyAuxiliaryComponentSuffix)},
			Data:       map[string][]byte{defaults.OAuthCookieSecretKeyName: []byte("cookie secret")},
		},
		metav1.CreateOptions{},
	)
	secretDtos, err = handler.GetSecretsForDeployment(appName, environmentName, deployment.Name)
	assert.NoError(t, err)
	expected = []models.Secret{
		{Name: component1Name + suffix.OAuth2ClientSecret, DisplayName: "Client Secret", Type: models.SecretTypeOAuth2Proxy, Component: component1Name, Status: models.Consistent.String()},
		{Name: component1Name + suffix.OAuth2CookieSecret, DisplayName: "Cookie Secret", Type: models.SecretTypeOAuth2Proxy, Component: component1Name, Status: models.Pending.String()},
		{Name: component2Name + suffix.OAuth2ClientSecret, DisplayName: "Client Secret", Type: models.SecretTypeOAuth2Proxy, Component: component2Name, Status: models.Pending.String()},
		{Name: component2Name + suffix.OAuth2CookieSecret, DisplayName: "Cookie Secret", Type: models.SecretTypeOAuth2Proxy, Component: component2Name, Status: models.Consistent.String()},
		{Name: component2Name + suffix.OAuth2RedisPassword, DisplayName: "Redis Password", Type: models.SecretTypeOAuth2Proxy, Component: component2Name, Status: models.Pending.String()},
	}
	assert.ElementsMatch(t, expected, secretDtos)

	// RedisPassword should have status Consistent
	comp2Secret.Data[defaults.OAuthRedisPasswordKeyName] = []byte("redis pwd")
	kubeclient.CoreV1().Secrets(envNs).Update(context.Background(), comp2Secret, metav1.UpdateOptions{})
	secretDtos, err = handler.GetSecretsForDeployment(appName, environmentName, deployment.Name)
	assert.NoError(t, err)
	expected = []models.Secret{
		{Name: component1Name + suffix.OAuth2ClientSecret, DisplayName: "Client Secret", Type: models.SecretTypeOAuth2Proxy, Component: component1Name, Status: models.Consistent.String()},
		{Name: component1Name + suffix.OAuth2CookieSecret, DisplayName: "Cookie Secret", Type: models.SecretTypeOAuth2Proxy, Component: component1Name, Status: models.Pending.String()},
		{Name: component2Name + suffix.OAuth2ClientSecret, DisplayName: "Client Secret", Type: models.SecretTypeOAuth2Proxy, Component: component2Name, Status: models.Pending.String()},
		{Name: component2Name + suffix.OAuth2CookieSecret, DisplayName: "Cookie Secret", Type: models.SecretTypeOAuth2Proxy, Component: component2Name, Status: models.Consistent.String()},
		{Name: component2Name + suffix.OAuth2RedisPassword, DisplayName: "Redis Password", Type: models.SecretTypeOAuth2Proxy, Component: component2Name, Status: models.Consistent.String()},
	}
	assert.ElementsMatch(t, expected, secretDtos)

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
