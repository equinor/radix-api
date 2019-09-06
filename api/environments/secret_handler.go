package environments

import (
	"fmt"
	"strings"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
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
	deployments, err := eh.deployHandler.GetDeploymentsForApplicationEnvironment(appName, envName, false)

	if err != nil {
		return nil, err
	}

	deployment, err := eh.deployHandler.GetDeploymentWithName(appName, deployments[0].Name)
	if err != nil {
		return nil, err
	}

	return eh.GetEnvironmentSecretsForDeployment(appName, envName, deployment)
}

// GetEnvironmentSecretsForDeployment Lists environment secrets for application
func (eh EnvironmentHandler) GetEnvironmentSecretsForDeployment(appName, envName string, activeDeployment *deploymentModels.Deployment) ([]environmentModels.Secret, error) {
	var appNamespace = k8sObjectUtils.GetAppNamespace(appName)
	var envNamespace = k8sObjectUtils.GetEnvironmentNamespace(appName, envName)
	ra, err := eh.radixclient.RadixV1().RadixApplications(appNamespace).Get(appName, metav1.GetOptions{})
	if err != nil {
		return []environmentModels.Secret{}, nil
	}

	rd, err := eh.radixclient.RadixV1().RadixDeployments(envNamespace).Get(activeDeployment.Name, metav1.GetOptions{})
	if err != nil {
		return []environmentModels.Secret{}, nil
	}

	secretsFromLatestDeployment, err := eh.getSecretsFromLatestDeployment(rd, envNamespace)
	if err != nil {
		return []environmentModels.Secret{}, nil
	}

	secretsFromTLSCertificates, err := eh.getSecretsFromTLSCertificates(ra, envName, envNamespace)
	if err != nil {
		return nil, err
	}

	secrets := make([]environmentModels.Secret, 0)
	for _, secretFromTLSCertificate := range secretsFromTLSCertificates {
		secrets = append(secrets, secretFromTLSCertificate)
	}

	for _, secretFromLatestDeployment := range secretsFromLatestDeployment {
		secrets = append(secrets, secretFromLatestDeployment)
	}

	return secrets, nil
}

func (eh EnvironmentHandler) getSecretsFromLatestDeployment(activeDeployment *v1.RadixDeployment, envNamespace string) (map[string]environmentModels.Secret, error) {
	componentSecretsMap := make(map[string]map[string]bool)
	for _, component := range activeDeployment.Spec.Components {
		if len(component.Secrets) <= 0 {
			continue
		}

		secretNamesMap := make(map[string]bool)
		componentSecrets := component.Secrets
		for _, componentSecretName := range componentSecrets {
			secretNamesMap[componentSecretName] = true
		}
		componentSecretsMap[component.Name] = secretNamesMap
	}

	secretDTOsMap := make(map[string]environmentModels.Secret)
	for componentName, secretNamesMap := range componentSecretsMap {
		secretObjectName := k8sObjectUtils.GetComponentSecretName(componentName)

		secret, err := eh.client.CoreV1().Secrets(envNamespace).Get(secretObjectName, metav1.GetOptions{})
		if err != nil && errors.IsNotFound(err) {
			// Mark secrets as Pending (exist in config, does not exist in cluster) due to no secret object in the cluster
			for secretName := range secretNamesMap {
				secretNameAndComponentName := fmt.Sprintf("%s-%s", secretName, componentName)
				if _, exists := secretDTOsMap[secretNameAndComponentName]; !exists {
					secretDTO := environmentModels.Secret{Name: secretName, Component: componentName, Status: environmentModels.Pending.String()}
					secretDTOsMap[secretNameAndComponentName] = secretDTO
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
		for secretName := range secretNamesMap {
			secretNameAndComponentName := fmt.Sprintf("%s-%s", secretName, componentName)
			if _, exists := secretDTOsMap[secretNameAndComponentName]; exists {
				continue
			}
			status := environmentModels.Consistent.String()
			if _, exists := clusterSecretEntriesMap[secretName]; !exists {
				status = environmentModels.Pending.String()
			}
			secretDTO := environmentModels.Secret{Name: secretName, Component: componentName, Status: status}
			secretDTOsMap[secretNameAndComponentName] = secretDTO
		}

		// Handle Orphan secrets (exist in cluster, does not exist in config)
		for clusterSecretName := range clusterSecretEntriesMap {
			clusterSecretNameAndComponentName := fmt.Sprintf("%s-%s", clusterSecretName, componentName)
			if _, exists := secretDTOsMap[clusterSecretNameAndComponentName]; exists {
				continue
			}
			status := environmentModels.Consistent.String()
			if _, exists := secretNamesMap[clusterSecretName]; !exists {
				status = environmentModels.Orphan.String()
			}
			secretDTO := environmentModels.Secret{Name: clusterSecretName, Component: componentName, Status: status}
			secretDTOsMap[clusterSecretNameAndComponentName] = secretDTO
		}
	}

	return secretDTOsMap, nil
}

func (eh EnvironmentHandler) getSecretsFromTLSCertificates(ra *v1.RadixApplication, envName, envNamespace string) (map[string]environmentModels.Secret, error) {
	secretDTOsMap := make(map[string]environmentModels.Secret)

	for _, externalAlias := range ra.Spec.DNSExternalAlias {
		if externalAlias.Environment != envName {
			continue
		}

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

func secretContained(secrets []environmentModels.Secret, theSecret environmentModels.Secret) bool {
	for _, aSecret := range secrets {
		if aSecret.Name == theSecret.Name &&
			aSecret.Component == theSecret.Component {
			return true
		}
	}
	return false
}
