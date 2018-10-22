package deployments

import (
	"testing"
	"time"

	"github.com/statoil/radix-api/api/utils"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/statoil/radix-operator/pkg/apis/utils"
	radix "github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

func TestGetDeployments_Filter_FilterIsApplied(t *testing.T) {
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	// Setup
	apply(kubeclient, radixclient, crdUtils.NewDeploymentBuilder().WithAppName("any-app-1").WithEnvironment("prod").WithImageTag("abcdef"))

	// Ensure the second image is considered the latest version
	apply(kubeclient, radixclient, crdUtils.NewDeploymentBuilder().
		WithAppName("any-app-2").
		WithEnvironment("dev").
		WithImageTag("ghijklm").
		WithCreated(time.Now()))

	apply(kubeclient, radixclient, crdUtils.NewDeploymentBuilder().
		WithAppName("any-app-2").
		WithEnvironment("dev").
		WithImageTag("nopqrst").
		WithCreated(time.Now().AddDate(0, 0, 1)))

	apply(kubeclient, radixclient, crdUtils.NewDeploymentBuilder().
		WithAppName("any-app-2").
		WithEnvironment("prod").
		WithImageTag("uvwxyza"))

	var testScenarios = []struct {
		name                   string
		appName                string
		environment            string
		latestOnly             bool
		numDeploymentsExpected int
	}{
		{"no filter should list all", "", "", false, 4},
		{"list all accross all environments", "any-app-2", "", false, 3},
		{"list all for environment", "any-app-2", "dev", false, 2},
		{"only list latest in environment", "any-app-2", "dev", true, 1},
		{"only list latest for all apps in all environments", "", "", true, 3},
		{"non existing app should lead to empty list", "any-app-3", "", false, 0},
		{"non existing environment should lead to empty list", "any-app-2", "qa", false, 0},
	}

	for _, scenario := range testScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			deployments, _ := HandleGetDeployments(radixclient, scenario.appName, scenario.environment, scenario.latestOnly)
			assert.Equal(t, scenario.numDeploymentsExpected, len(deployments))
		})
	}
}

func TestPromote_ErrorScenarios_ErrorIsReturned(t *testing.T) {
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	// Setup
	apply(kubeclient, radixclient, crdUtils.NewDeploymentBuilder().
		WithAppName("any-app-").
		WithEnvironment("prod").
		WithImageTag("abcdef"))

	apply(kubeclient, radixclient, crdUtils.NewDeploymentBuilder().
		WithAppName("any-app-1").
		WithEnvironment("dev").
		WithImageTag("abcdef"))

	apply(kubeclient, radixclient, crdUtils.NewDeploymentBuilder().
		WithAppName("any-app-2").
		WithEnvironment("dev").
		WithImageTag("abcdef"))

	apply(kubeclient, radixclient, crdUtils.NewDeploymentBuilder().
		WithAppName("any-app-2").
		WithEnvironment("prod").
		WithImageTag("ghijklm"))

	apply(kubeclient, radixclient, crdUtils.NewDeploymentBuilder().
		WithAppName("any-app-3").
		WithEnvironment("dev").
		WithImageTag("abcdef"))

	apply(kubeclient, radixclient, crdUtils.NewDeploymentBuilder().
		WithAppName("any-app-3").
		WithEnvironment("prod").
		WithImageTag("abcdef"))

	createEnvNamespace(kubeclient, "any-app-4", "dev")
	createEnvNamespace(kubeclient, "any-app-4", "prod")

	var testScenarios = []struct {
		name                 string
		appName              string
		fromEnvironment      string
		imageTag             string
		toEnvironment        string
		expectedErrorMessage string
	}{
		{"promote empty app", "", "dev", "abcdef", "prod", "App name is required"},
		{"promote non-existing app", "noapp", "dev", "abcdef", "prod", ""},
		{"promote from non-existing environment", "any-app-", "dev", "abcdef", "prod", "Non existing from environment"},
		{"promote to non-existing environment", "any-app-1", "dev", "abcdef", "prod", "Non existing to environment"},
		{"promote non-existing image", "any-app-2", "dev", "nopqrst", "prod", "Non existing deployment"},
		{"promote an image into environment having already that image", "any-app-3", "dev", "abcdef", "prod", ""},
	}

	for _, scenario := range testScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			parameters := PromotionParameters{FromEnvironment: scenario.fromEnvironment, ToEnvironment: scenario.toEnvironment}

			_, err := HandlePromoteToEnvironment(kubeclient, radixclient, scenario.appName, crdUtils.GetDeploymentName(scenario.appName, scenario.imageTag), parameters)
			assert.Error(t, err)

			if scenario.expectedErrorMessage != "" {
				assert.Equal(t, scenario.expectedErrorMessage, (err.(*utils.Error)).Message)
			}
		})
	}
}

func TestPromote_HappyPathScenarios_NewStateIsExpected(t *testing.T) {
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	// Setup
	apply(kubeclient, radixclient, crdUtils.NewDeploymentBuilder().
		WithAppName("any-app-").
		WithEnvironment("dev").
		WithImageTag("abcdef"))

	// Create prod environment without any deployments
	createEnvNamespace(kubeclient, "any-app-", "prod")

	// Ensure the second image is considered the latest version
	apply(kubeclient, radixclient, crdUtils.NewDeploymentBuilder().
		WithAppName("any-app-1").
		WithEnvironment("dev").
		WithImageTag("ghijklm").
		WithCreated(time.Now()))

	apply(kubeclient, radixclient, crdUtils.NewDeploymentBuilder().
		WithAppName("any-app-1").
		WithEnvironment("dev").
		WithImageTag("nopqrst").
		WithCreated(time.Now().AddDate(0, 0, 1)))

	createEnvNamespace(kubeclient, "any-app-1", "prod")

	var testScenarios = []struct {
		name            string
		appName         string
		fromEnvironment string
		imageTag        string
		toEnvironment   string
		imageExpected   string
	}{
		{"promote single image", "any-app-", "dev", "abcdef", "prod", ""},
	}

	for _, scenario := range testScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			parameters := PromotionParameters{FromEnvironment: scenario.fromEnvironment, ToEnvironment: scenario.toEnvironment}

			_, err := HandlePromoteToEnvironment(kubeclient, radixclient, scenario.appName, crdUtils.GetDeploymentName(scenario.appName, scenario.imageTag), parameters)
			assert.NoError(t, err)

			if scenario.imageExpected != "" {
				deployments, _ := HandleGetDeployments(radixclient, scenario.appName, scenario.toEnvironment, false)
				assert.Equal(t, 1, len(deployments))
				assert.Equal(t, crdUtils.GetDeploymentName(scenario.appName, scenario.imageExpected), deployments[0].Name)
			}
		})
	}
}

func TestPromote_WithEnvironmentVariables_NewStateIsExpected(t *testing.T) {
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	// Setup
	// When we have enviroment specific config the deployment should contain the environment variables defined in the config
	devVariable := make(map[string]string)
	prodVariable := make(map[string]string)
	devVariable["DB_HOST"] = "useless-dev"
	prodVariable["DB_HOST"] = "useless-prod"

	environmentVariables := []v1.EnvVars{
		v1.EnvVars{Environment: "dev", Variables: devVariable},
		v1.EnvVars{Environment: "prod", Variables: prodVariable}}

	appComponent := crdUtils.NewApplicationComponentBuilder().
		WithName("app").
		WithEnvironmentVariablesMap(environmentVariables)

	customRadixConfig := crdUtils.NewRadixApplicationBuilder().
		WithAppName("any-app-2").
		WithComponent(appComponent).
		WithEnvironments([]string{"dev", "prod"})

	applyWithConfig(kubeclient, radixclient, customRadixConfig, crdUtils.NewDeploymentBuilder().
		WithAppName("any-app-2").
		WithEnvironment("dev").
		WithImageTag("abcdef").
		WithComponent(
			crdUtils.NewDeployComponentBuilder().
				WithName("app")))

	// Create prod environment without any deployments
	createEnvNamespace(kubeclient, "any-app-2", "prod")

	// Scenario
	_, err := HandlePromoteToEnvironment(kubeclient, radixclient, "any-app-2", crdUtils.GetDeploymentName("any-app-2", "abcdef"), PromotionParameters{FromEnvironment: "dev", ToEnvironment: "prod"})
	assert.NoError(t, err, "HandlePromoteToEnvironment - Unexpected error")

	deployments, _ := HandleGetDeployments(radixclient, "any-app-2", "prod", false)
	assert.Equal(t, 1, len(deployments), "HandlePromoteToEnvironment - Was not promoted as expected")

	// Get the RD to see if it has merged ok with the RA
	radixDeployment, _ := radixclient.RadixV1().RadixDeployments(crdUtils.GetEnvironmentNamespace(deployments[0].AppName, deployments[0].Environment)).Get(deployments[0].Name, metav1.GetOptions{})
	assert.Equal(t, 1, len(radixDeployment.Spec.Components[0].EnvironmentVariables), "HandlePromoteToEnvironment - Was not promoted as expected")
	assert.Equal(t, "useless-prod", radixDeployment.Spec.Components[0].EnvironmentVariables["DB_HOST"], "HandlePromoteToEnvironment - Was not promoted as expected")

}

func apply(kubeclient *kubernetes.Clientset, radixclient *radix.Clientset, builder crdUtils.DeploymentBuilder) {
	defaultRadixConfig := crdUtils.NewRadixApplicationBuilder().WithAppName(builder.BuildRD().Spec.AppName).WithComponent(crdUtils.NewApplicationComponentBuilder().WithName("app")).WithEnvironments([]string{"dev", "prod"})
	applyWithConfig(kubeclient, radixclient, defaultRadixConfig, builder)
}

func applyWithConfig(kubeclient *kubernetes.Clientset, radixclient *radix.Clientset, configBuilder crdUtils.ApplicationBuilder, builder crdUtils.DeploymentBuilder) {
	rd := builder.BuildRD()

	// Create a app namespace and RA
	ns := createAppNamespace(kubeclient, rd.Spec.AppName)
	radixclient.RadixV1().RadixApplications(ns).Create(configBuilder.BuildRA())

	ns = createEnvNamespace(kubeclient, rd.Spec.AppName, rd.Spec.Environment)
	radixclient.RadixV1().RadixDeployments(ns).Create(rd)
}

func createAppNamespace(kubeclient *kubernetes.Clientset, appName string) string {
	ns := crdUtils.GetAppNamespace(appName)
	createNamespace(kubeclient, ns)
	return ns
}

func createEnvNamespace(kubeclient *kubernetes.Clientset, appName, environment string) string {
	ns := crdUtils.GetEnvironmentNamespace(appName, environment)
	createNamespace(kubeclient, ns)
	return ns
}

func createNamespace(kubeclient *kubernetes.Clientset, ns string) {
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}

	kubeclient.CoreV1().Namespaces().Create(&namespace)
}
