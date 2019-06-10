package environments

import (
	"fmt"
	"strings"

	environmentModels "github.com/equinor/radix-api/api/environments/models"
	k8sObjectUtils "github.com/equinor/radix-operator/pkg/apis/utils"

	"github.com/equinor/radix-api/api/utils"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	tlsSecretDefaultData = "xx"
	certPartSuffix       = "-cert"
	tlsCertPart          = "tls.crt"
	keyPartSuffix        = "-key"
	tlsKeyPart           = "tls.key"
)

// ChangeEnvironmentComponentSecret handler for HandleChangeEnvironmentComponentSecret
func (eh EnvironmentHandler) ChangeEnvironmentComponentSecret(appName, envName, componentName, secretName string, componentSecret environmentModels.SecretParameters) (*environmentModels.SecretParameters, error) {
	newSecretValue := componentSecret.SecretValue
	if strings.TrimSpace(newSecretValue) == "" {
		return nil, utils.ValidationError("Secret", "New secret value is empty")
	}

	ns := k8sObjectUtils.GetEnvironmentNamespace(appName, envName)

	var secretObjName, partName string

	if strings.HasSuffix(secretName, certPartSuffix) {
		// This is the cert part of the TLS secret
		secretObjName = strings.TrimSuffix(secretName, certPartSuffix)
		partName = tlsCertPart

	} else if strings.HasSuffix(secretName, keyPartSuffix) {
		// This is the key part of the TLS secret
		secretObjName = strings.TrimSuffix(secretName, keyPartSuffix)
		partName = tlsKeyPart

	} else {
		// This is a regular secret
		secretObjName = k8sObjectUtils.GetComponentSecretName(componentName)
		partName = secretName

	}

	secretObject, err := eh.client.CoreV1().Secrets(ns).Get(secretObjName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		return nil, utils.TypeMissingError("Secret object does not exist", err)
	}
	if err != nil {
		return nil, utils.UnexpectedError("Failed getting secret object", err)
	}

	if secretObject.Data == nil {
		secretObject.Data = make(map[string][]byte)
	}

	secretObject.Data[partName] = []byte(newSecretValue)

	updatedSecret, err := eh.client.CoreV1().Secrets(ns).Update(secretObject)
	if err != nil {
		return nil, err
	}

	componentSecret.SecretValue = string(updatedSecret.Data[partName])
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

	secretsFromConfig, err := eh.getSecretsFromConfig(ra, envNamespace)
	if err != nil {
		return nil, err
	}

	secretsFromTLSCertificates, err := eh.getSecretsFromTLSCertificates(ra, envNamespace)
	if err != nil {
		return nil, err
	}

	secrets := make([]environmentModels.Secret, 0)
	for _, secretFromConfig := range secretsFromConfig {
		secrets = append(secrets, secretFromConfig)
	}

	for _, secretFromTLSCertificate := range secretsFromTLSCertificates {
		secrets = append(secrets, secretFromTLSCertificate)
	}

	return secrets, nil
}

func (eh EnvironmentHandler) getSecretsFromConfig(ra *v1.RadixApplication, envNamespace string) (map[string]environmentModels.Secret, error) {
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
		secretObjectName := k8sObjectUtils.GetComponentSecretName(raComponentName)

		secret, err := eh.client.CoreV1().Secrets(envNamespace).Get(secretObjectName, metav1.GetOptions{})
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

	return secretDTOsMap, nil
}

func (eh EnvironmentHandler) getSecretsFromTLSCertificates(ra *v1.RadixApplication, envNamespace string) (map[string]environmentModels.Secret, error) {
	secretDTOsMap := make(map[string]environmentModels.Secret)

	for _, externalAlias := range ra.Spec.DNSExternalAlias {
		certStatus := environmentModels.Consistent.String()
		keyStatus := environmentModels.Consistent.String()

		secretValue, err := eh.client.CoreV1().Secrets(envNamespace).Get(externalAlias.Alias, metav1.GetOptions{})
		if err != nil && errors.IsNotFound(err) {
			certStatus = environmentModels.Pending.String()
		}

		if err != nil {
			return nil, err
		}

		certValue := strings.TrimSpace(string(secretValue.Data[tlsCertPart]))
		if strings.EqualFold(certValue, tlsSecretDefaultData) {
			certStatus = environmentModels.Pending.String()
		}

		keyValue := strings.TrimSpace(string(secretValue.Data[tlsKeyPart]))
		if strings.EqualFold(keyValue, tlsSecretDefaultData) {
			keyStatus = environmentModels.Pending.String()
		}

		secretDTO := environmentModels.Secret{Name: externalAlias.Alias + certPartSuffix, Component: externalAlias.Component, Status: certStatus}
		secretDTOsMap[secretDTO.Name] = secretDTO

		secretDTO = environmentModels.Secret{Name: externalAlias.Alias + keyPartSuffix, Component: externalAlias.Component, Status: keyStatus}
		secretDTOsMap[secretDTO.Name] = secretDTO
	}

	return secretDTOsMap, nil
}
