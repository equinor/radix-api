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
	if err != nil || envVarsConfigMap.Data == nil || envVarsMetadataMap == nil {
		return nil, err
	}
	envVars, err := deployment.GetEnvironmentVariables(&eh.kubeUtil, appName, rd, radixDeployComponent)
	if err != nil {
		return nil, err
	}
	var apiEnvVars []envvarsmodels.EnvVar
	for _, envVar := range envVars {
		apiEnvVar := envvarsmodels.EnvVar{Name: envVar.Name}
		if envVar.ValueFrom == nil {
			apiEnvVar.Value = envVar.Value
		} else if envVarValue, foundValue := envVarsConfigMap.Data[envVar.Name]; foundValue {
			apiEnvVar.Value = envVarValue
		}
		if envVarMetadata, foundMetadata := envVarsMetadataMap[envVar.Name]; foundMetadata {
			apiEnvVar.Metadata = &envvarsmodels.EnvVarMetadata{RadixConfigValue: envVarMetadata.RadixConfigValue}
		}
		apiEnvVars = append(apiEnvVars, apiEnvVar)
	}
	sort.Slice(apiEnvVars, func(i, j int) bool { return apiEnvVars[i].Name < apiEnvVars[j].Name })
	return apiEnvVars, nil
}

//ChangeEnvVar Change environment variables
func (eh EnvVarsHandler) ChangeEnvVar(appName, envName, componentName string, envVarsParams []envvarsmodels.EnvVarParameter) error {
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
			log.Infof("Not found changing variable '%s'", envVarParam.Name)
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
