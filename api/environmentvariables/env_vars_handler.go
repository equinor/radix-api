package environmentvariables

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/rs/zerolog/log"

	envvarsmodels "github.com/equinor/radix-api/api/environmentvariables/models"
	"github.com/equinor/radix-api/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"k8s.io/client-go/kubernetes"
)

type EnvVarsHandler interface {
	GetComponentEnvVars(ctx context.Context, appName string, envName string, componentName string) ([]envvarsmodels.EnvVar, error)
	ChangeEnvVar(ctx context.Context, appName, envName, componentName string, envVarsParams []envvarsmodels.EnvVarParameter) error
}

// EnvVarsHandlerOptions defines a configuration function
type EnvVarsHandlerOptions func(*envVarsHandler)

// WithAccounts configures all EnvVarsHandler fields
func WithAccounts(accounts models.Accounts) EnvVarsHandlerOptions {
	return func(eh *envVarsHandler) {
		kubeUtil, _ := kube.New(accounts.UserAccount.Client, accounts.UserAccount.RadixClient, accounts.UserAccount.SecretProviderClient)
		eh.kubeUtil = kubeUtil
		eh.inClusterClient = accounts.ServiceAccount.Client
		eh.accounts = accounts
	}
}

// EnvVarsHandler Instance variables
type envVarsHandler struct {
	kubeUtil        *kube.Kube
	inClusterClient kubernetes.Interface
	accounts        models.Accounts
}

// Init Constructor.
// Use the WithAccounts configuration function to configure a 'ready to use' EnvVarsHandler.
// EnvVarsHandlerOptions are processed in the sequence they are passed to this function.
func Init(opts ...EnvVarsHandlerOptions) EnvVarsHandler {
	eh := envVarsHandler{}
	for _, opt := range opts {
		opt(&eh)
	}
	return &eh
}

// GetComponentEnvVars Get environment variables with metadata for the component
func (eh *envVarsHandler) GetComponentEnvVars(ctx context.Context, appName string, envName string, componentName string) ([]envvarsmodels.EnvVar, error) {
	namespace := crdUtils.GetEnvironmentNamespace(appName, envName)
	rd, err := eh.kubeUtil.GetActiveDeployment(namespace)
	if err != nil {
		return nil, err
	}
	radixDeployComponent := getComponent(rd, componentName)
	if radixDeployComponent == nil {
		return nil, fmt.Errorf("RadixDeployComponent not found by name")
	}
	envVarsConfigMap, _, envVarsMetadataMap, err := eh.kubeUtil.GetEnvVarsConfigMapAndMetadataMap(namespace, componentName)
	if err != nil {
		return nil, err
	}
	envVars, err := deployment.GetEnvironmentVariables(eh.kubeUtil, appName, rd, radixDeployComponent)
	if err != nil {
		return nil, err
	}
	secretNamesMap := make(map[string]interface{})
	secretNamesMap = appendKeysToMap(secretNamesMap, radixDeployComponent.GetSecrets())
	secretNamesMap = appendSecretRefsKeysToMap(secretNamesMap, radixDeployComponent.GetSecretRefs())
	var apiEnvVars []envvarsmodels.EnvVar
	for _, envVar := range envVars {
		apiEnvVar := envvarsmodels.EnvVar{Name: envVar.Name}
		if envVar.ValueFrom == nil {
			apiEnvVar.Value = envVar.Value
			apiEnvVars = append(apiEnvVars, apiEnvVar)
			continue
		}
		if _, ok := secretNamesMap[envVar.Name]; ok {
			continue //skip secrets
		}
		if envVarsConfigMap.Data == nil {
			continue
		}
		if envVarValue, foundValue := envVarsConfigMap.Data[envVar.Name]; foundValue {
			apiEnvVar.Value = envVarValue
		}
		if envVarsMetadataMap != nil {
			if envVarMetadata, foundMetadata := envVarsMetadataMap[envVar.Name]; foundMetadata {
				apiEnvVar.Metadata = &envvarsmodels.EnvVarMetadata{RadixConfigValue: envVarMetadata.RadixConfigValue}
			}
		}
		apiEnvVars = append(apiEnvVars, apiEnvVar)
	}
	sort.Slice(apiEnvVars, func(i, j int) bool { return apiEnvVars[i].Name < apiEnvVars[j].Name })
	return apiEnvVars, nil
}

func appendKeysToMap(namesMap map[string]interface{}, values []string) map[string]interface{} {
	for _, value := range values {
		namesMap[value] = true
	}
	return namesMap
}

func appendSecretRefsKeysToMap(namesMap map[string]interface{}, secretRefs v1.RadixSecretRefs) map[string]interface{} {
	for _, azureKeyVault := range secretRefs.AzureKeyVaults {
		for _, keyVaultItem := range azureKeyVault.Items {
			namesMap[keyVaultItem.EnvVar] = true
		}
	}
	return namesMap
}

// ChangeEnvVar Change environment variables
func (eh *envVarsHandler) ChangeEnvVar(ctx context.Context, appName, envName, componentName string, envVarsParams []envvarsmodels.EnvVarParameter) error {
	namespace := crdUtils.GetEnvironmentNamespace(appName, envName)
	currentEnvVarsConfigMap, envVarsMetadataConfigMap, envVarsMetadataMap, err := eh.kubeUtil.GetEnvVarsConfigMapAndMetadataMap(namespace, componentName)
	desiredEnvVarsConfigMap := currentEnvVarsConfigMap.DeepCopy()
	if err != nil {
		return err
	}
	hasChanges := false
	for _, envVarParam := range envVarsParams {
		if crdUtils.IsRadixEnvVar(envVarParam.Name) {
			continue
		}
		currentEnvVarValue, foundEnvVar := desiredEnvVarsConfigMap.Data[envVarParam.Name]
		if !foundEnvVar {

			log.Ctx(ctx).Info().Msgf("Not found changing variable %s", envVarParam.Name)
			hasChanges = true
			delete(envVarsMetadataMap, envVarParam.Name)
			continue
		}
		newEnvVarValue := strings.TrimSpace(envVarParam.Value)
		desiredEnvVarsConfigMap.Data[envVarParam.Name] = newEnvVarValue
		hasChanges = true
		metadata, foundMetadata := envVarsMetadataMap[envVarParam.Name]
		if foundMetadata {
			if strings.EqualFold(metadata.RadixConfigValue, newEnvVarValue) {
				delete(envVarsMetadataMap, envVarParam.Name) //delete metadata for equal value in radixconfig
			}
			continue
		}
		if !strings.EqualFold(currentEnvVarValue, newEnvVarValue) { //create metadata for changed env-var
			envVarsMetadataMap[envVarParam.Name] = kube.EnvVarMetadata{RadixConfigValue: currentEnvVarValue}
		}
	}
	if !hasChanges {
		return nil
	}
	err = eh.kubeUtil.ApplyConfigMap(namespace, currentEnvVarsConfigMap, desiredEnvVarsConfigMap)
	if err != nil {
		return err
	}
	return eh.kubeUtil.ApplyEnvVarsMetadataConfigMap(namespace, envVarsMetadataConfigMap, envVarsMetadataMap)
}

func getComponent(rd *v1.RadixDeployment, componentName string) v1.RadixCommonDeployComponent {
	for _, component := range rd.Spec.Components {
		if strings.EqualFold(component.Name, componentName) {
			return &component
		}
	}
	for _, jobComponent := range rd.Spec.Jobs {
		if strings.EqualFold(jobComponent.Name, componentName) {
			return &jobComponent
		}
	}
	return nil
}
