package environmentvariables

import (
	"github.com/equinor/radix-api/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func Test_GetEnvVars(t *testing.T) {
	namespace := utils.GetEnvironmentNamespace(appName, environmentName)
	t.Run("Get existing env vars", func(t *testing.T) {
		t.Parallel()
		_, _, _, commonTestUtils, kubeUtil := setupTest()

		envVarsMap := map[string]string{
			"VAR1": "val1",
			"VAR2": "val2",
		}
		setupDeployment(&commonTestUtils, appName, environmentName, componentName, func(builder builders.DeployComponentBuilder) {
			builder.WithEnvironmentVariables(envVarsMap)
		})
		handler := envVarsHandler{
			kubeUtil:        commonTestUtils.GetKubeUtil(),
			inClusterClient: nil,
			accounts:        models.Accounts{},
		}

		kubeUtil.CreateConfigMap(namespace, &corev1.ConfigMap{
			ObjectMeta: meta.ObjectMeta{Name: kube.GetEnvVarsConfigMapName(componentName)},
			Data:       envVarsMap})
		kubeUtil.CreateConfigMap(namespace, &corev1.ConfigMap{
			ObjectMeta: meta.ObjectMeta{Name: kube.GetEnvVarsMetadataConfigMapName(componentName)},
			Data: map[string]string{
				"metadata": `{
                            "VAR1": {"RadixConfigValue": "orig-val1"}
                        }`,
			}})

		envVars, err := handler.GetComponentEnvVars(appName, environmentName, componentName)

		assert.NoError(t, err)
		assert.NotEmpty(t, envVars)
		assert.Equal(t, "VAR1", envVars[0].Name)
		assert.Equal(t, "val1", envVars[0].Value)
		assert.Equal(t, "VAR2", envVars[1].Name)
		assert.Equal(t, "val2", envVars[1].Value)
	})
}
