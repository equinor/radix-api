package environments

import (
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"

	environmentModels "github.com/statoil/radix-api/api/environments/models"
	"github.com/statoil/radix-api/api/utils"
	k8sObjectUtils "github.com/statoil/radix-operator/pkg/apis/utils"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// EnvironmentHandler Instance variables
type EnvironmentHandler struct {
	kubeClient  kubernetes.Interface
	radixClient radixclient.Interface
}

// Init Constructor
func Init(kubeClient kubernetes.Interface, radixClient radixclient.Interface) EnvironmentHandler {
	return EnvironmentHandler{
		kubeClient:  kubeClient,
		radixClient: radixClient,
	}
}

// HandleChangeEnvironmentComponentSecret handler for HandleChangeEnvironmentComponentSecret
func (eh EnvironmentHandler) HandleChangeEnvironmentComponentSecret(appName, envName, componentName, secretName string, componentSecret environmentModels.ComponentSecret) (*environmentModels.ComponentSecret, error) {
	newSecretValue := componentSecret.SecretValue
	if strings.TrimSpace(newSecretValue) == "" {
		return nil, utils.ValidationError("Secret", "New secret value is empty")
	}

	ns := k8sObjectUtils.GetEnvironmentNamespace(appName, envName)
	secretObject, err := eh.kubeClient.CoreV1().Secrets(ns).Get(componentName, metaV1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		return nil, utils.TypeMissingError("Secret object does not exist", err)
	}
	if err != nil {
		return nil, utils.UnexpectedError("Failed getting secret object", err)
	}

	oldSecretValue, exists := secretObject.Data[secretName]
	if !exists {
		return nil, utils.ValidationError("Secret", "Secret name does not exist")
	}

	if string(oldSecretValue) == newSecretValue {
		return nil, utils.ValidationError("Secret", "No change in secret value")
	}

	secretObject.Data[secretName] = []byte(newSecretValue)

	updatedSecret, err := eh.kubeClient.CoreV1().Secrets(ns).Update(secretObject)
	if err != nil {
		return nil, err
	}

	componentSecret.SecretValue = string(updatedSecret.Data[secretName])

	return &componentSecret, nil
}
