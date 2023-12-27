package environmentvariables

import (
	"testing"

	envvarsmodels "github.com/equinor/radix-api/api/environmentvariables/models"
	"github.com/equinor/radix-api/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetEnvVars(t *testing.T) {
	namespace := operatorutils.GetEnvironmentNamespace(appName, environmentName)
	t.Run("Get existing env vars", func(t *testing.T) {
		t.Parallel()
		_, _, _, commonTestUtils, kubeUtil, _ := setupTest(t)

		envVarsMap := map[string]string{
			"VAR1": "val1",
			"VAR2": "val2",
		}
		_, err := setupDeployment(&commonTestUtils, appName, environmentName, componentName, func(builder operatorutils.DeployComponentBuilder) {
			builder.WithEnvironmentVariables(envVarsMap).
				WithSecrets([]string{"SECRET1", "SECRET2"})
		})
		require.NoError(t, err)
		handler := envVarsHandler{
			kubeUtil:        commonTestUtils.GetKubeUtil(),
			inClusterClient: nil,
			accounts:        models.Accounts{},
		}

		_, err = kubeUtil.GetConfigMap(namespace, kube.GetEnvVarsConfigMapName(componentName))
		require.NoError(t, err)

		_, err = kubeUtil.GetConfigMap(namespace, kube.GetEnvVarsMetadataConfigMapName(componentName))
		require.NoError(t, err)

		envVars, err := handler.GetComponentEnvVars(appName, environmentName, componentName)

		assert.NoError(t, err)
		assert.NotEmpty(t, envVars)
		assert.Len(t, envVars, 2)
		assert.Equal(t, "VAR1", envVars[0].Name)
		assert.Equal(t, "val1", envVars[0].Value)
		assert.Equal(t, "VAR2", envVars[1].Name)
		assert.Equal(t, "val2", envVars[1].Value)
	})
}

func Test_ChangeGetEnvVars(t *testing.T) {
	namespace := operatorutils.GetEnvironmentNamespace(appName, environmentName)
	t.Run("Change existing env var", func(t *testing.T) {
		t.Parallel()
		_, _, _, commonTestUtils, kubeUtil, _ := setupTest(t)

		envVarsMap := map[string]string{
			"VAR1": "val1",
			"VAR2": "val2",
			"VAR3": "val3",
		}
		_, err := setupDeployment(&commonTestUtils, appName, environmentName, componentName, func(builder operatorutils.DeployComponentBuilder) {
			builder.WithEnvironmentVariables(envVarsMap)
		})
		require.NoError(t, err)
		handler := envVarsHandler{
			kubeUtil:        commonTestUtils.GetKubeUtil(),
			inClusterClient: nil,
			accounts:        models.Accounts{},
		}

		_, err = kubeUtil.GetConfigMap(namespace, kube.GetEnvVarsConfigMapName(componentName))
		require.NoError(t, err)

		_, err = kubeUtil.GetConfigMap(namespace, kube.GetEnvVarsMetadataConfigMapName(componentName))
		require.NoError(t, err)

		params := []envvarsmodels.EnvVarParameter{
			{
				Name:  "VAR2",
				Value: "new-val2",
			},
			{
				Name:  "VAR3",
				Value: "new-val3",
			},
		}
		err = handler.ChangeEnvVar(appName, environmentName, componentName, params)

		assert.NoError(t, err)

		envVars, err := handler.GetComponentEnvVars(appName, environmentName, componentName)
		assert.NoError(t, err)
		assert.NotEmpty(t, envVars)
		assert.Len(t, envVars, 3)
		assert.Equal(t, "VAR1", envVars[0].Name)
		assert.Equal(t, "val1", envVars[0].Value)
		assert.Equal(t, "VAR2", envVars[1].Name)
		assert.Equal(t, "new-val2", envVars[1].Value)
		assert.Equal(t, "VAR3", envVars[2].Name)
		assert.Equal(t, "new-val3", envVars[2].Value)
	})
	t.Run("Skipped changing not-existing env vars", func(t *testing.T) {
		t.Parallel()
		_, _, _, commonTestUtils, kubeUtil, _ := setupTest(t)

		envVarsMap := map[string]string{
			"VAR1": "val1",
			"VAR2": "val2",
		}
		_, err := setupDeployment(&commonTestUtils, appName, environmentName, componentName, func(builder operatorutils.DeployComponentBuilder) {
			builder.WithEnvironmentVariables(envVarsMap)
		})
		require.NoError(t, err)
		handler := envVarsHandler{
			kubeUtil:        commonTestUtils.GetKubeUtil(),
			inClusterClient: nil,
			accounts:        models.Accounts{},
		}

		_, err = kubeUtil.GetConfigMap(namespace, kube.GetEnvVarsConfigMapName(componentName))
		require.NoError(t, err)

		_, err = kubeUtil.GetConfigMap(namespace, kube.GetEnvVarsMetadataConfigMapName(componentName))
		require.NoError(t, err)

		params := []envvarsmodels.EnvVarParameter{
			{
				Name:  "SOME_NOT_EXISTING_VAR",
				Value: "new-val",
			},
			{
				Name:  "VAR2",
				Value: "new-val2",
			},
		}
		err = handler.ChangeEnvVar(appName, environmentName, componentName, params)

		require.NoError(t, err)

		envVars, err := handler.GetComponentEnvVars(appName, environmentName, componentName)
		assert.NoError(t, err)
		assert.NotEmpty(t, envVars)
		assert.Len(t, envVars, 2)
		assert.Equal(t, "VAR1", envVars[0].Name)
		assert.Equal(t, "val1", envVars[0].Value)
		assert.Equal(t, "VAR2", envVars[1].Name)
		assert.Equal(t, "new-val2", envVars[1].Value)
	})
}
