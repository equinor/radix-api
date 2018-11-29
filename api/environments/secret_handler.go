package environments

import (
	"strings"

	environmentModels "github.com/statoil/radix-api/api/environments/models"
	k8sObjectUtils "github.com/statoil/radix-operator/pkg/apis/utils"

	"github.com/statoil/radix-api/api/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ChangeEnvironmentComponentSecret handler for HandleChangeEnvironmentComponentSecret
func (eh EnvironmentHandler) ChangeEnvironmentComponentSecret(appName, envName, componentName, secretName string, componentSecret environmentModels.SecretParameters) (*environmentModels.SecretParameters, error) {
	newSecretValue := componentSecret.SecretValue
	if strings.TrimSpace(newSecretValue) == "" {
		return nil, utils.ValidationError("Secret", "New secret value is empty")
	}

	ns := k8sObjectUtils.GetEnvironmentNamespace(appName, envName)
	secretObject, err := eh.client.CoreV1().Secrets(ns).Get(componentName, metav1.GetOptions{})
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

	updatedSecret, err := eh.client.CoreV1().Secrets(ns).Update(secretObject)
	if err != nil {
		return nil, err
	}

	componentSecret.SecretValue = string(updatedSecret.Data[secretName])

	return &componentSecret, nil
}

// GetEnvironmentSecrets Lists environment secrets for application
func (eh EnvironmentHandler) GetEnvironmentSecrets(appName, envName string) ([]environmentModels.Secret, error) {
	return nil, nil
}
