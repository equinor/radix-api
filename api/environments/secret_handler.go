package environments

import (
	"context"
	"fmt"
	"strings"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	k8sObjectUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"

	"github.com/equinor/radix-api/api/utils"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	secretDefaultData = "xx"
	certPartSuffix    = "-cert"
	tlsCertPart       = "tls.crt"
	keyPartSuffix     = "-key"
	tlsKeyPart        = "tls.key"
	clientCertSuffix  = "-clientcertca"
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

	} else if strings.HasSuffix(secretName, defaults.BlobFuseCredsAccountKeyPartSuffix) {
		// This is the account key part of the blobfuse cred secret
		secretObjName = strings.TrimSuffix(secretName, defaults.BlobFuseCredsAccountKeyPartSuffix)
		partName = defaults.BlobFuseCredsAccountKeyPart

	} else if strings.HasSuffix(secretName, defaults.BlobFuseCredsAccountNamePartSuffix) {
		// This is the account name part of the blobfuse cred secret
		secretObjName = strings.TrimSuffix(secretName, defaults.BlobFuseCredsAccountNamePartSuffix)
		partName = defaults.BlobFuseCredsAccountNamePart

	} else if strings.HasSuffix(secretName, clientCertSuffix) {
		// This is the account name part of the client certificate secret
		secretObjName = secretName
		partName = "ca.crt"

	} else {
		// This is a regular secret
		secretObjName = k8sObjectUtils.GetComponentSecretName(componentName)
		partName = secretName

	}

	secretObject, err := eh.client.CoreV1().Secrets(ns).Get(context.TODO(), secretObjName, metav1.GetOptions{})
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

	updatedSecret, err := eh.client.CoreV1().Secrets(ns).Update(context.TODO(), secretObject, metav1.UpdateOptions{})
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

	depl, err := eh.deployHandler.GetDeploymentWithName(appName, deployments[0].Name)
	if err != nil {
		return nil, err
	}

	return eh.GetEnvironmentSecretsForDeployment(appName, envName, depl)
}

// GetEnvironmentSecretsForDeployment Lists environment secrets for application
func (eh EnvironmentHandler) GetEnvironmentSecretsForDeployment(appName, envName string, activeDeployment *deploymentModels.Deployment) ([]environmentModels.Secret, error) {
	var appNamespace = k8sObjectUtils.GetAppNamespace(appName)
	var envNamespace = k8sObjectUtils.GetEnvironmentNamespace(appName, envName)
	ra, err := eh.radixclient.RadixV1().RadixApplications(appNamespace).Get(context.TODO(), appName, metav1.GetOptions{})
	if err != nil {
		return []environmentModels.Secret{}, nil
	}

	rd, err := eh.radixclient.RadixV1().RadixDeployments(envNamespace).Get(context.TODO(), activeDeployment.Name, metav1.GetOptions{})
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

	secretsFromVolumeMounts, err := eh.getSecretsFromVolumeMounts(rd, envNamespace)
	if err != nil {
		return nil, err
	}

	secretsFromAuthenticationClientCertificate, err := eh.getSecretsFromAuthenticationClientCertificate(rd, envNamespace)
	if err != nil {
		return nil, err
	}

	secrets := make([]environmentModels.Secret, 0)
	for _, secretFromVolumeMounts := range secretsFromVolumeMounts {
		secrets = append(secrets, secretFromVolumeMounts)
	}

	for _, secretFromTLSCertificate := range secretsFromTLSCertificates {
		secrets = append(secrets, secretFromTLSCertificate)
	}

	for _, secretFromLatestDeployment := range secretsFromLatestDeployment {
		secrets = append(secrets, secretFromLatestDeployment)
	}

	for _, secretFromAuthenticationClientCertificate := range secretsFromAuthenticationClientCertificate {
		secrets = append(secrets, secretFromAuthenticationClientCertificate)
	}

	return secrets, nil
}

func (eh EnvironmentHandler) getSecretsForComponent(component v1.RadixCommonDeployComponent) map[string]bool {
	if len(component.GetSecrets()) <= 0 {
		return nil
	}

	secretNamesMap := make(map[string]bool)
	componentSecrets := component.GetSecrets()
	for _, componentSecretName := range componentSecrets {
		secretNamesMap[componentSecretName] = true
	}

	return secretNamesMap
}

func (eh EnvironmentHandler) getSecretsFromLatestDeployment(activeDeployment *v1.RadixDeployment, envNamespace string) (map[string]environmentModels.Secret, error) {
	componentSecretsMap := make(map[string]map[string]bool)
	for _, component := range activeDeployment.Spec.Components {
		secrets := eh.getSecretsForComponent(&component)
		if len(secrets) <= 0 {
			continue
		}
		componentSecretsMap[component.Name] = secrets
	}
	for _, job := range activeDeployment.Spec.Jobs {
		secrets := eh.getSecretsForComponent(&job)
		if len(secrets) <= 0 {
			continue
		}
		componentSecretsMap[job.Name] = secrets
	}

	secretDTOsMap := make(map[string]environmentModels.Secret)
	for componentName, secretNamesMap := range componentSecretsMap {
		secretObjectName := k8sObjectUtils.GetComponentSecretName(componentName)

		secret, err := eh.client.CoreV1().Secrets(envNamespace).Get(context.TODO(), secretObjectName, metav1.GetOptions{})
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

func (eh EnvironmentHandler) getSecretsFromComponentVolumeMounts(component v1.RadixCommonDeployComponent, envNamespace string) []environmentModels.Secret {
	var secrets []environmentModels.Secret
	for _, volumeMount := range component.GetVolumeMounts() {
		switch volumeMount.Type {
		case v1.MountTypeBlob:
			accountKeySecret, accountNameSecret := eh.getBlobFuseSecrets(component, envNamespace, volumeMount)
			secrets = append(secrets, accountKeySecret)
			secrets = append(secrets, accountNameSecret)
		case v1.MountTypeBlobCsiAzure, v1.MountTypeFileCsiAzure:
			accountKeySecret, accountNameSecret := eh.getCsiAzureSecrets(component, envNamespace, volumeMount)
			secrets = append(secrets, accountKeySecret)
			secrets = append(secrets, accountNameSecret)
		}
	}
	return secrets
}

func (eh EnvironmentHandler) getBlobFuseSecrets(component v1.RadixCommonDeployComponent, envNamespace string, volumeMount v1.RadixVolumeMount) (environmentModels.Secret, environmentModels.Secret) {
	return eh.getAzureVolumeMountSecrets(component, envNamespace, defaults.GetBlobFuseCredsSecretName(component.GetName(), volumeMount.Name), defaults.BlobFuseCredsAccountNamePart, defaults.BlobFuseCredsAccountKeyPart, defaults.BlobFuseCredsAccountNamePartSuffix, defaults.BlobFuseCredsAccountKeyPartSuffix)
}

func (eh EnvironmentHandler) getCsiAzureSecrets(component v1.RadixCommonDeployComponent, envNamespace string, volumeMount v1.RadixVolumeMount) (environmentModels.Secret, environmentModels.Secret) {
	return eh.getAzureVolumeMountSecrets(component, envNamespace, defaults.GetCsiAzureCredsSecretName(component.GetName(), volumeMount.Name), defaults.CsiAzureCredsAccountNamePart, defaults.CsiAzureCredsAccountKeyPart, defaults.CsiAzureCredsAccountNamePartSuffix, defaults.CsiAzureCredsAccountKeyPartSuffix)
}

func (eh EnvironmentHandler) getAzureVolumeMountSecrets(component v1.RadixCommonDeployComponent, envNamespace, secretName, accountNamePart, accountKeyPart, accountNamePartSuffix, accountKeyPartSuffix string) (environmentModels.Secret, environmentModels.Secret) {
	accountkeyStatus := environmentModels.Consistent.String()
	accountnameStatus := environmentModels.Consistent.String()

	secretValue, err := eh.client.CoreV1().Secrets(envNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		log.Warnf("Error on retrieving secret '%s'. Message: %s", secretName, err.Error())
		accountkeyStatus = environmentModels.Pending.String()
		accountnameStatus = environmentModels.Pending.String()
	} else {
		accountkeyValue := strings.TrimSpace(string(secretValue.Data[accountKeyPart]))
		if strings.EqualFold(accountkeyValue, secretDefaultData) {
			accountkeyStatus = environmentModels.Pending.String()
		}
		accountnameValue := strings.TrimSpace(string(secretValue.Data[accountNamePart]))
		if strings.EqualFold(accountnameValue, secretDefaultData) {
			accountnameStatus = environmentModels.Pending.String()
		}
	}

	accountKeySecretDTO := environmentModels.Secret{Name: secretName + accountKeyPartSuffix, Component: component.GetName(), Status: accountkeyStatus}
	accountNameSecretDTO := environmentModels.Secret{Name: secretName + accountNamePartSuffix, Component: component.GetName(), Status: accountnameStatus}
	return accountKeySecretDTO, accountNameSecretDTO
}

func (eh EnvironmentHandler) getSecretsFromVolumeMounts(activeDeployment *v1.RadixDeployment, envNamespace string) (map[string]environmentModels.Secret, error) {
	secretDTOsMap := make(map[string]environmentModels.Secret)

	for _, component := range activeDeployment.Spec.Components {
		secrets := eh.getSecretsFromComponentVolumeMounts(&component, envNamespace)
		for _, secret := range secrets {
			secretDTOsMap[secret.Name] = secret
		}
	}

	for _, job := range activeDeployment.Spec.Jobs {
		secrets := eh.getSecretsFromComponentVolumeMounts(&job, envNamespace)
		for _, secret := range secrets {
			secretDTOsMap[secret.Name] = secret
		}
	}

	return secretDTOsMap, nil
}

func (eh EnvironmentHandler) getSecretsFromAuthenticationClientCertificate(activeDeployment *v1.RadixDeployment, envNamespace string) (map[string]environmentModels.Secret, error) {
	secretDTOsMap := make(map[string]environmentModels.Secret)

	for _, component := range activeDeployment.Spec.Components {
		secret := eh.getSecretsFromComponentAuthenticationClientCertificate(&component, envNamespace)
		if secret != nil {
			secretDTOsMap[secret.Name] = *secret
		}
	}

	return secretDTOsMap, nil
}

func (eh EnvironmentHandler) getSecretsFromComponentAuthenticationClientCertificate(component v1.RadixCommonDeployComponent, envNamespace string) *environmentModels.Secret {
	if auth := component.GetAuthentication(); auth != nil && component.GetPublicPort() != "" && deployment.IsSecretRequiredForClientCertificate(auth.ClientCertificate) {
		secretName := k8sObjectUtils.GetComponentClientCertificateSecretName(component.GetName())
		secretStatus := environmentModels.Consistent.String()

		secret, err := eh.client.CoreV1().Secrets(envNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
		if err != nil {
			secretStatus = environmentModels.Pending.String()
		} else {
			secretValue := strings.TrimSpace(string(secret.Data["ca.crt"]))
			if strings.EqualFold(secretValue, secretDefaultData) {
				secretStatus = environmentModels.Pending.String()
			}
		}

		return &environmentModels.Secret{
			Name:      secretName,
			Component: component.GetName(),
			Status:    secretStatus,
		}
	}

	return nil
}

func (eh EnvironmentHandler) getSecretsFromTLSCertificates(ra *v1.RadixApplication, envName, envNamespace string) (map[string]environmentModels.Secret, error) {
	secretDTOsMap := make(map[string]environmentModels.Secret)

	for _, externalAlias := range ra.Spec.DNSExternalAlias {
		if externalAlias.Environment != envName {
			continue
		}

		certStatus := environmentModels.Consistent.String()
		keyStatus := environmentModels.Consistent.String()

		secretValue, err := eh.client.CoreV1().Secrets(envNamespace).Get(context.TODO(), externalAlias.Alias, metav1.GetOptions{})
		if err != nil {
			log.Warnf("Error on retrieving secret '%s'. Message: %s", externalAlias.Alias, err.Error())
			certStatus = environmentModels.Pending.String()
			keyStatus = environmentModels.Pending.String()
		} else {
			certValue := strings.TrimSpace(string(secretValue.Data[tlsCertPart]))
			if strings.EqualFold(certValue, secretDefaultData) {
				certStatus = environmentModels.Pending.String()
			}

			keyValue := strings.TrimSpace(string(secretValue.Data[tlsKeyPart]))
			if strings.EqualFold(keyValue, secretDefaultData) {
				keyStatus = environmentModels.Pending.String()
			}
		}

		secretDTO := environmentModels.Secret{Name: externalAlias.Alias + certPartSuffix, Component: externalAlias.Component, Status: certStatus}
		secretDTOsMap[secretDTO.Name] = secretDTO

		secretDTO = environmentModels.Secret{Name: externalAlias.Alias + keyPartSuffix, Component: externalAlias.Component, Status: keyStatus}
		secretDTOsMap[secretDTO.Name] = secretDTO
	}

	return secretDTOsMap, nil
}
