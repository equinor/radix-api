package environments

import (
	"fmt"
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

	if secretObject.Data == nil {
		secretObject.Data = make(map[string][]byte)
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
	var appNamespace = k8sObjectUtils.GetAppNamespace(appName)
	var envNamespace = k8sObjectUtils.GetEnvironmentNamespace(appName, envName)
	ra, err := eh.radixclient.RadixV1().RadixApplications(appNamespace).Get(appName, metav1.GetOptions{})
	if err != nil {
		return []environmentModels.Secret{}, nil
	}

	// Secrets from config
	raComponentSecretsMap := make(map[string]map[string]bool)
	for _, raComponent := range ra.Spec.Components {
		if len(raComponent.Secrets) <= 0 {
			continue
		}

		raSecretNamesMap := make(map[string]bool)
		raComponentSecrets := raComponent.Secrets
		for _, raComponentSecretName := range raComponentSecrets {
			raSecretNamesMap[raComponentSecretName] = true
		}
		raComponentSecretsMap[raComponent.Name] = raSecretNamesMap
	}

	secretDTOsMap := make(map[string]environmentModels.Secret)
	for raComponentName, raSecretNamesMap := range raComponentSecretsMap {
		secret, err := eh.client.CoreV1().Secrets(envNamespace).Get(raComponentName, metav1.GetOptions{})
		if err != nil && errors.IsNotFound(err) {
			// Mark secrets as Pending (exist in config, does not exist in cluster) due to no secret object in the cluster
			for raSecretName := range raSecretNamesMap {
				raSecretNameAndComponentName := fmt.Sprintf("%s-%s", raSecretName, raComponentName)
				if _, exists := secretDTOsMap[raSecretNameAndComponentName]; !exists {
					secretDTO := environmentModels.Secret{Name: raSecretName, Component: raComponentName, Status: environmentModels.Pending.String()}
					secretDTOsMap[raSecretNameAndComponentName] = secretDTO
				}
			}
			continue
		}
		if err != nil {
			return nil, err
		}

		// Secrets from cluster
		clusterSecretEntriesMap := secret.Data

		// Handle Pending secrets (exist in config, does not exist in cluster) due to no secret object entry in the cluster
		for raSecretName := range raSecretNamesMap {
			raSecretNameAndComponentName := fmt.Sprintf("%s-%s", raSecretName, raComponentName)
			if _, exists := secretDTOsMap[raSecretNameAndComponentName]; exists {
				continue
			}
			status := environmentModels.Consistent.String()
			if _, exists := clusterSecretEntriesMap[raSecretName]; !exists {
				status = environmentModels.Pending.String()
			}
			secretDTO := environmentModels.Secret{Name: raSecretName, Component: raComponentName, Status: status}
			secretDTOsMap[raSecretNameAndComponentName] = secretDTO
		}

		// Handle Orphan secrets (exist in cluster, does not exist in config)
		for clusterSecretName := range clusterSecretEntriesMap {
			clusterSecretNameAndComponentName := fmt.Sprintf("%s-%s", clusterSecretName, raComponentName)
			if _, exists := secretDTOsMap[clusterSecretNameAndComponentName]; exists {
				continue
			}
			status := environmentModels.Consistent.String()
			if _, exists := raSecretNamesMap[clusterSecretName]; !exists {
				status = environmentModels.Orphan.String()
			}
			secretDTO := environmentModels.Secret{Name: clusterSecretName, Component: raComponentName, Status: status}
			secretDTOsMap[clusterSecretNameAndComponentName] = secretDTO
		}
	}

	secrets := make([]environmentModels.Secret, 0)
	for _, secretDTO := range secretDTOsMap {
		secrets = append(secrets, secretDTO)
	}

	return secrets, nil
}
