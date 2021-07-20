package environmentvariables

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/equinor/radix-api/api/deployments"
	envvarsmodels "github.com/equinor/radix-api/api/environmentvariables/models"
	"github.com/equinor/radix-api/api/events"
	"github.com/equinor/radix-api/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	coreListers "k8s.io/client-go/listers/core/v1"
)

// EnvVarsHandlerOptions defines a configuration function
type EnvVarsHandlerOptions func(*EnvVarsHandler)

// WithAccounts configures all EnvVarsHandler fields
func WithAccounts(accounts models.Accounts) EnvVarsHandlerOptions {
	return func(eh *EnvVarsHandler) {
		eh.client = accounts.UserAccount.Client
		eh.radixclient = accounts.UserAccount.RadixClient
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
	client          kubernetes.Interface
	radixclient     radixclient.Interface
	inClusterClient kubernetes.Interface
	deployHandler   deployments.DeployHandler
	eventHandler    events.EventHandler
	accounts        models.Accounts
}

type Kube struct {
}

func (k EnvVarsHandler) GetKubeClient() kubernetes.Interface {
	return k.client
}

func (k EnvVarsHandler) GetRadixClient() radixclient.Interface {
	return k.radixclient
}

func (k EnvVarsHandler) GetConfigMapLister() coreListers.ConfigMapLister {
	return nil
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
	rd, err := eh.getActiveDeployment(namespace)
	if err != nil {
		return nil, err
	}
	component := getComponent(rd, componentName)
	if component == nil {
		return nil, fmt.Errorf("component not found by name")
	}
	envVarsConfigMap, _, envVarsMetadataMap, err := eh.getEnvVarsConfigMapAndMetadataMap(namespace, componentName)
	if err != nil {
		return nil, err
	}
	var apiEnvVars []envvarsmodels.EnvVar
	envVarsMap := component.GetEnvironmentVariables()
	for envVarName, envVar := range envVarsMap {
		apiEnvVar := envvarsmodels.EnvVar{Name: envVarName, Value: envVar}
		if cmEnvVar, foundCmEnvVar := envVarsConfigMap.Data[envVarName]; foundCmEnvVar {
			apiEnvVar.Value = cmEnvVar
			if envVarMetadata, foundMetadata := envVarsMetadataMap[envVarName]; foundMetadata {
				apiEnvVar.Metadata = &envvarsmodels.EnvVarMetadata{RadixConfigValue: envVarMetadata.RadixConfigValue}
			}
		}
		apiEnvVars = append(apiEnvVars, apiEnvVar)
	}
	sort.Slice(apiEnvVars, func(i, j int) bool { return apiEnvVars[i].Name < apiEnvVars[j].Name })
	return apiEnvVars, nil
}

func (eh EnvVarsHandler) getEnvVarsConfigMapAndMetadataMap(namespace string, componentName string) (*corev1.ConfigMap, *corev1.ConfigMap, map[string]v1.EnvVarMetadata, error) {
	envVarsConfigMap, err := eh.getConfigMap(namespace, kube.GetEnvVarsConfigMapName(componentName))
	if err != nil {
		return nil, nil, nil, err
	}
	envVarsMetadataConfigMap, err := eh.getConfigMap(namespace, kube.GetEnvVarsMetadataConfigMapName(componentName))
	if err != nil {
		return nil, nil, nil, err
	}
	envVarsMetadataMap, err := kube.GetEnvVarsMetadataFromConfigMap(envVarsMetadataConfigMap)
	return envVarsConfigMap, envVarsMetadataConfigMap, envVarsMetadataMap, nil
}

//ChangeEnvVar Change environment variables
func (eh EnvVarsHandler) ChangeEnvVar(appName, envName, componentName string, envVars []envvarsmodels.EnvVarParameter) error {
	namespace := crdUtils.GetEnvironmentNamespace(appName, envName)
	currentEnvVarsConfigMap, currentEnvVarsMetadataConfigMap, envVarsMetadataMap, err := eh.getEnvVarsConfigMapAndMetadataMap(namespace, componentName)
	if err != nil {
		return err
	}
	desiredEnvVarsConfigMap := currentEnvVarsConfigMap.DeepCopy()
	hasChanges := false
	for _, envVarParam := range envVars {
		_, foundEnvVar := currentEnvVarsConfigMap.Data[envVarParam.Name]
		if !foundEnvVar {
			log.Infof("Not found variable '%s'", envVarParam.Name)
			continue
		}
		newEnvVarValue := strings.TrimSpace(envVarParam.Value)
		desiredEnvVarsConfigMap.Data[envVarParam.Name] = newEnvVarValue
		hasChanges = true
		metadata, foundMetadata := envVarsMetadataMap[envVarParam.Name]
		if foundMetadata && strings.EqualFold(metadata.RadixConfigValue, newEnvVarValue) {
			delete(envVarsMetadataMap, envVarParam.Name)
		}
	}
	if !hasChanges {
		return nil
	}
	err = kube.ApplyConfigMap(eh, namespace, currentEnvVarsConfigMap, desiredEnvVarsConfigMap)
	if err != nil {
		return err
	}
	return kube.ApplyEnvVarsMetadataConfigMap(eh, namespace, currentEnvVarsMetadataConfigMap, envVarsMetadataMap)
}

func (eh EnvVarsHandler) getConfigMap(namespace string, envVarsConfigMapName string) (*corev1.ConfigMap, error) {
	return eh.client.CoreV1().ConfigMaps(namespace).Get(context.TODO(), envVarsConfigMapName, metav1.GetOptions{})
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

func (eh EnvVarsHandler) getActiveDeployment(namespace string) (*v1.RadixDeployment, error) {
	radixDeployments, err := eh.radixclient.RadixV1().RadixDeployments(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, rd := range radixDeployments.Items {
		if rd.Status.ActiveTo.IsZero() {
			return &rd, err
		}
	}
	return nil, nil
}
