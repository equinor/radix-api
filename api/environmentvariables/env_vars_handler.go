package environmentvariables

import (
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"sort"
	"strings"

	"github.com/equinor/radix-api/api/deployments"
	envvarsmodels "github.com/equinor/radix-api/api/environmentvariables/models"
	"github.com/equinor/radix-api/api/events"
	"github.com/equinor/radix-api/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// EnvVarsHandlerOptions defines a configuration function
type EnvVarsHandlerOptions func(*EnvVarsHandler)

// WithAccounts configures all EnvVarsHandler fields
func WithAccounts(accounts models.Accounts) EnvVarsHandlerOptions {
	return func(eh *EnvVarsHandler) {
		kubeUtil, _ := kube.New(accounts.UserAccount.Client, accounts.UserAccount.RadixClient)
		eh.kubeUtil = *kubeUtil
		eh.inClusterClient = accounts.ServiceAccount.Client
		eh.deployHandler = deployments.Init(accounts)
		eh.eventHandler = events.Init(accounts.UserAccount.Client)
		eh.accounts = accounts
	}
}

// WithEventHandler configures the eventHandler used by EnvVarHandler
func WithEventHandler(eventHandler events.EventHandler) EnvVarsHandlerOptions {
	return func(eh *EnvVarsHandler) {
		eh.eventHandler = eventHandler
	}
}

// EnvVarsHandler Instance variables
type EnvVarsHandler struct {
	kubeUtil        kube.Kube
	inClusterClient kubernetes.Interface
	deployHandler   deployments.DeployHandler
	eventHandler    events.EventHandler
	accounts        models.Accounts
}

// Init Constructor.
// Use the WithAccounts configuration function to configure a 'ready to use' EnvVarsHandler.
// EnvVarsHandlerOptions are processed in the seqeunce they are passed to this function.
func Init(opts ...EnvVarsHandlerOptions) EnvVarsHandler {
	eh := EnvVarsHandler{}

	for _, opt := range opts {
		opt(&eh)
	}

	return eh
}

//GetComponentEnvVars Get environment variables with metadata for the component
func (eh EnvVarsHandler) GetComponentEnvVars(appName string, envName string, componentName string) ([]envvarsmodels.EnvVar, error) {
	apiEnvVars, _, _, _, err := eh.getComponentEnvVars(appName, envName, componentName)
	if err != nil {
		return nil, err
	}
	sort.Slice(apiEnvVars, func(i, j int) bool { return apiEnvVars[i].Name < apiEnvVars[j].Name })
	return apiEnvVars, nil
}

//ChangeEnvVar Change environment variables
func (eh EnvVarsHandler) ChangeEnvVar(appName, envName, componentName string, envVars []envvarsmodels.EnvVarParameter) error {
	apiEnvVars, currentEnvVarsConfigMap, currentEnvVarsMetadataConfigMap, envVarsMetadataMap, err := eh.getComponentEnvVars(appName, envName, componentName)
	apiEnvVarsMap := getEnvVarMapFromList(apiEnvVars)
	if err != nil {
		return err
	}
	desiredEnvVarsConfigMap := currentEnvVarsConfigMap.DeepCopy()
	hasChanges := false
	for _, envVarParam := range envVars {
		currentEnvVarValue, foundEnvVar := currentEnvVarsConfigMap.Data[envVarParam.Name]
		if !foundEnvVar {
			_, foundApiEnvVar := apiEnvVarsMap[envVarParam.Name]
			if !foundApiEnvVar {
				log.Infof("Not found variable '%s'", envVarParam.Name)
				continue
			}
			currentEnvVarsConfigMap.Data[envVarParam.Name] = envVarParam.Value //add env-var, not existing in config-map, but existing in list of radix-deploy-component env-vars
			hasChanges = true
			if _, foundMetadata := envVarsMetadataMap[envVarParam.Name]; foundMetadata { //in case outdated metadata exists from past
				delete(envVarsMetadataMap, envVarParam.Name)
			}
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
			envVarsMetadataMap[envVarParam.Name] = v1.EnvVarMetadata{RadixConfigValue: currentEnvVarValue}
		}
	}
	if !hasChanges {
		return nil
	}
	namespace := crdUtils.GetEnvironmentNamespace(appName, envName)
	err = eh.kubeUtil.ApplyConfigMap(namespace, currentEnvVarsConfigMap, desiredEnvVarsConfigMap)
	if err != nil {
		return err
	}
	return eh.kubeUtil.ApplyEnvVarsMetadataConfigMap(namespace, currentEnvVarsMetadataConfigMap, envVarsMetadataMap)
}

func getEnvVarMapFromList(envVars []envvarsmodels.EnvVar) map[string]envvarsmodels.EnvVar {
	envVarsMap := make(map[string]envvarsmodels.EnvVar, len(envVars))
	for _, envVar := range envVars {
		envVar := envVar
		envVarsMap[envVar.Name] = envVar
	}
	return envVarsMap

}

func (eh EnvVarsHandler) getComponentEnvVars(appName string, envName string, componentName string) ([]envvarsmodels.EnvVar, *corev1.ConfigMap, *corev1.ConfigMap, map[string]v1.EnvVarMetadata, error) {
	namespace := crdUtils.GetEnvironmentNamespace(appName, envName)
	rd, err := eh.kubeUtil.GetActiveDeployment(namespace)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	radixDeployComponent := getComponent(rd, componentName)
	if radixDeployComponent == nil {
		return nil, nil, nil, nil, fmt.Errorf("radixDeployComponent not found by name")
	}
	envVarsConfigMap, envVarsMetadataConfigMap, envVarsMetadataMap, err := eh.getEnvVarsConfigMapAndMetadataMap(namespace, componentName)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	var apiEnvVars []envvarsmodels.EnvVar
	envVars := deployment.GetEnvironmentVariablesFrom(appName, rd, radixDeployComponent, envVarsConfigMap, envVarsMetadataMap)
	for _, envVar := range envVars {
		apiEnvVar := envvarsmodels.EnvVar{Name: envVar.Name, Value: envVar.Value}
		if cmEnvVar, foundCmEnvVar := envVarsConfigMap.Data[envVar.Name]; foundCmEnvVar {
			apiEnvVar.Value = cmEnvVar
			if envVarMetadata, foundMetadata := envVarsMetadataMap[envVar.Name]; foundMetadata {
				apiEnvVar.Metadata = &envvarsmodels.EnvVarMetadata{RadixConfigValue: envVarMetadata.RadixConfigValue}
			}
		}
		apiEnvVars = append(apiEnvVars, apiEnvVar)
	}
	return apiEnvVars, envVarsConfigMap, envVarsMetadataConfigMap, envVarsMetadataMap, nil
}

func (eh EnvVarsHandler) getEnvVarsConfigMapAndMetadataMap(namespace string, componentName string) (*corev1.ConfigMap, *corev1.ConfigMap, map[string]v1.EnvVarMetadata, error) {
	envVarsConfigMap, err := eh.kubeUtil.GetConfigMap(namespace, kube.GetEnvVarsConfigMapName(componentName))
	if err != nil {
		return nil, nil, nil, err
	}
	envVarsMetadataConfigMap, err := eh.kubeUtil.GetConfigMap(namespace, kube.GetEnvVarsMetadataConfigMapName(componentName))
	if err != nil {
		return nil, nil, nil, err
	}
	envVarsMetadataMap, err := kube.GetEnvVarsMetadataFromConfigMap(envVarsMetadataConfigMap)
	if err != nil {
		return nil, nil, nil, err
	}
	return envVarsConfigMap, envVarsMetadataConfigMap, envVarsMetadataMap, nil
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
