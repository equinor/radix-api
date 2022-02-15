package secrets

import (
	"context"
	"fmt"
	"strings"

	"github.com/equinor/radix-api/api/deployments"
	"github.com/equinor/radix-api/api/secrets/models"
	"github.com/equinor/radix-api/api/secrets/suffix"
	apiModels "github.com/equinor/radix-api/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	secretDefaultData = "xx"
	tlsCertPart       = "tls.crt"
	tlsKeyPart        = "tls.key"
)

// SecretHandlerOptions defines a configuration function
type SecretHandlerOptions func(*SecretHandler)

// WithAccounts configures all SecretHandler fields
func WithAccounts(accounts apiModels.Accounts) SecretHandlerOptions {
	return func(eh *SecretHandler) {
		eh.client = accounts.UserAccount.Client
		eh.radixclient = accounts.UserAccount.RadixClient
		eh.deployHandler = deployments.Init(accounts)
	}
}

// SecretHandler Instance variables
type SecretHandler struct {
	client        kubernetes.Interface
	radixclient   radixclient.Interface
	deployHandler deployments.DeployHandler
}

// Init Constructor.
// Use the WithAccounts configuration function to configure a 'ready to use' SecretHandler.
// SecretHandlerOptions are processed in the seqeunce they are passed to this function.
func Init(opts ...SecretHandlerOptions) SecretHandler {
	eh := SecretHandler{}

	for _, opt := range opts {
		opt(&eh)
	}

	return eh
}

// ChangeComponentSecret handler for HandleChangeComponentSecret
func (eh SecretHandler) ChangeComponentSecret(appName, envName, componentName, secretName string, componentSecret models.SecretParameters) error {
	newSecretValue := componentSecret.SecretValue
	if strings.TrimSpace(newSecretValue) == "" {
		return radixhttp.ValidationError("Secret", "New secret value is empty")
	}

	ns := operatorutils.GetEnvironmentNamespace(appName, envName)

	var secretObjName, partName string

	if strings.HasSuffix(secretName, suffix.ExternalDNSCert) {
		// This is the cert part of the TLS secret
		secretObjName = strings.TrimSuffix(secretName, suffix.ExternalDNSCert)
		partName = tlsCertPart

	} else if strings.HasSuffix(secretName, suffix.ExternalDNSKeyPart) {
		// This is the key part of the TLS secret
		secretObjName = strings.TrimSuffix(secretName, suffix.ExternalDNSKeyPart)
		partName = tlsKeyPart

	} else if strings.HasSuffix(secretName, defaults.BlobFuseCredsAccountKeyPartSuffix) {
		// This is the account key part of the blobfuse cred secret
		secretObjName = strings.TrimSuffix(secretName, defaults.BlobFuseCredsAccountKeyPartSuffix)
		partName = defaults.BlobFuseCredsAccountKeyPart

	} else if strings.HasSuffix(secretName, defaults.BlobFuseCredsAccountNamePartSuffix) {
		// This is the account name part of the blobfuse cred secret
		secretObjName = strings.TrimSuffix(secretName, defaults.BlobFuseCredsAccountNamePartSuffix)
		partName = defaults.BlobFuseCredsAccountNamePart

	} else if strings.HasSuffix(secretName, defaults.CsiAzureCredsAccountKeyPartSuffix) {
		// This is the account key part of the Csi Azure volume cred secret
		secretObjName = strings.TrimSuffix(secretName, defaults.CsiAzureCredsAccountKeyPartSuffix)
		partName = defaults.CsiAzureCredsAccountKeyPart

	} else if strings.HasSuffix(secretName, defaults.CsiAzureCredsAccountNamePartSuffix) {
		// This is the account name part of the Csi Azure volume cred secret
		secretObjName = strings.TrimSuffix(secretName, defaults.CsiAzureCredsAccountNamePartSuffix)
		partName = defaults.CsiAzureCredsAccountNamePart

	} else if strings.HasSuffix(secretName, defaults.CsiAzureKeyVaultCredsClientIdSuffix) {
		// This is the client-id part of the Csi Azure KeyVault cred secret
		secretObjName = strings.TrimSuffix(secretName, defaults.CsiAzureKeyVaultCredsClientIdSuffix)
		partName = defaults.CsiAzureKeyVaultCredsClientIdPart

	} else if strings.HasSuffix(secretName, defaults.CsiAzureKeyVaultCredsClientSecretSuffix) {
		// This is the client secret part of the Csi Azure KeyVault cred secret
		secretObjName = strings.TrimSuffix(secretName, defaults.CsiAzureKeyVaultCredsClientSecretSuffix)
		partName = defaults.CsiAzureKeyVaultCredsClientSecretPart

	} else if strings.HasSuffix(secretName, suffix.ClientCertificate) {
		// This is the account name part of the client certificate secret
		secretObjName = secretName
		partName = "ca.crt"

	} else if strings.HasSuffix(secretName, suffix.OAuth2ClientSecret) {
		secretObjName = operatorutils.GetAuxiliaryComponentSecretName(componentName, defaults.OAuthProxyAuxiliaryComponentSuffix)
		partName = defaults.OAuthClientSecretKeyName
	} else if strings.HasSuffix(secretName, suffix.OAuth2CookieSecret) {
		secretObjName = operatorutils.GetAuxiliaryComponentSecretName(componentName, defaults.OAuthProxyAuxiliaryComponentSuffix)
		partName = defaults.OAuthCookieSecretKeyName
	} else if strings.HasSuffix(secretName, suffix.OAuth2RedisPassword) {
		secretObjName = operatorutils.GetAuxiliaryComponentSecretName(componentName, defaults.OAuthProxyAuxiliaryComponentSuffix)
		partName = defaults.OAuthRedisPasswordKeyName
	} else {
		// This is a regular secret
		secretObjName = operatorutils.GetComponentSecretName(componentName)
		partName = secretName

	}

	secretObject, err := eh.client.CoreV1().Secrets(ns).Get(context.TODO(), secretObjName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		return radixhttp.TypeMissingError("Secret object does not exist", err)
	}
	if err != nil {
		return radixhttp.UnexpectedError("Failed getting secret object", err)
	}

	if secretObject.Data == nil {
		secretObject.Data = make(map[string][]byte)
	}

	secretObject.Data[partName] = []byte(newSecretValue)

	_, err = eh.client.CoreV1().Secrets(ns).Update(context.TODO(), secretObject, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

// GetSecretsForDeployment Lists environment secrets for application
func (eh SecretHandler) GetSecretsForDeployment(appName, envName, deploymentName string) ([]models.Secret, error) {
	var appNamespace = operatorutils.GetAppNamespace(appName)
	var envNamespace = operatorutils.GetEnvironmentNamespace(appName, envName)
	ra, err := eh.radixclient.RadixV1().RadixApplications(appNamespace).Get(context.TODO(), appName, metav1.GetOptions{})
	if err != nil {
		return []models.Secret{}, nil
	}

	rd, err := eh.radixclient.RadixV1().RadixDeployments(envNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return []models.Secret{}, nil
	}

	secretsFromLatestDeployment, err := eh.getSecretsFromLatestDeployment(rd, envNamespace)
	if err != nil {
		return []models.Secret{}, nil
	}

	secretsFromTLSCertificates, err := eh.getSecretsFromTLSCertificates(ra, envName, envNamespace)
	if err != nil {
		return nil, err
	}

	secretsFromVolumeMounts, err := eh.getSecretsFromVolumeMounts(rd, envNamespace)
	if err != nil {
		return nil, err
	}

	secretsFromAuthentication, err := eh.getSecretsFromAuthentication(rd, envNamespace)
	if err != nil {
		return nil, err
	}

	secretRefsSecrets, err := eh.getSecretRefsSecrets(rd, envNamespace)
	if err != nil {
		return nil, err
	}

	secrets := make([]models.Secret, 0)
	for _, secretFromVolumeMounts := range secretsFromVolumeMounts {
		secrets = append(secrets, secretFromVolumeMounts)
	}

	for _, secretFromTLSCertificate := range secretsFromTLSCertificates {
		secrets = append(secrets, secretFromTLSCertificate)
	}

	for _, secretFromLatestDeployment := range secretsFromLatestDeployment {
		secrets = append(secrets, secretFromLatestDeployment)
	}

	secrets = append(secrets, secretsFromAuthentication...)

	secrets = append(secrets, secretRefsSecrets...)

	return secrets, nil
}

func (eh SecretHandler) getSecretsForComponent(component v1.RadixCommonDeployComponent) map[string]bool {
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

func (eh SecretHandler) getSecretsFromLatestDeployment(activeDeployment *v1.RadixDeployment, envNamespace string) (map[string]models.Secret, error) {
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

	secretDTOsMap := make(map[string]models.Secret)
	for componentName, secretNamesMap := range componentSecretsMap {
		secretObjectName := operatorutils.GetComponentSecretName(componentName)

		secret, err := eh.client.CoreV1().Secrets(envNamespace).Get(context.TODO(), secretObjectName, metav1.GetOptions{})
		if err != nil && errors.IsNotFound(err) {
			// Mark secrets as Pending (exist in config, does not exist in cluster) due to no secret object in the cluster
			for secretName := range secretNamesMap {
				secretNameAndComponentName := fmt.Sprintf("%s-%s", secretName, componentName)
				if _, exists := secretDTOsMap[secretNameAndComponentName]; !exists {
					secretDTO := models.Secret{Name: secretName, DisplayName: secretName, Component: componentName, Status: models.Pending.String(), Type: models.SecretTypeGeneric}
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
			status := models.Consistent.String()
			if _, exists := clusterSecretEntriesMap[secretName]; !exists {
				status = models.Pending.String()
			}
			secretDTO := models.Secret{Name: secretName, DisplayName: secretName, Component: componentName, Status: status, Type: models.SecretTypeGeneric}
			secretDTOsMap[secretNameAndComponentName] = secretDTO
		}

		// Handle Orphan secrets (exist in cluster, does not exist in config)
		for clusterSecretName := range clusterSecretEntriesMap {
			clusterSecretNameAndComponentName := fmt.Sprintf("%s-%s", clusterSecretName, componentName)
			if _, exists := secretDTOsMap[clusterSecretNameAndComponentName]; exists {
				continue
			}
			status := models.Consistent.String()
			if _, exists := secretNamesMap[clusterSecretName]; !exists {
				status = models.Orphan.String()
			}
			secretDTO := models.Secret{Name: clusterSecretName, DisplayName: clusterSecretName, Component: componentName, Status: status, Type: models.SecretTypeOrphaned}
			secretDTOsMap[clusterSecretNameAndComponentName] = secretDTO
		}
	}

	return secretDTOsMap, nil
}

func (eh SecretHandler) getCredentialSecretsForBlobVolumes(component v1.RadixCommonDeployComponent, envNamespace string) []models.Secret {
	var secrets []models.Secret
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

func (eh SecretHandler) getCredentialSecretsForSecretRefs(component v1.RadixCommonDeployComponent, envNamespace string) ([]models.Secret, error) {
	secretRefs := component.GetSecretRefs()
	if len(secretRefs.AzureKeyVaults) == 0 {
		return nil, nil
	}
	var secrets []models.Secret
	for _, azureKeyVault := range secretRefs.AzureKeyVaults {
		secretName := defaults.GetCsiAzureKeyVaultCredsSecretName(component.GetName(), azureKeyVault.Name)
		clientIdStatus := models.Consistent.String()
		clientSecretStatus := models.Consistent.String()

		secretValue, err := eh.client.CoreV1().Secrets(envNamespace).Get(context.Background(), secretName, metav1.GetOptions{})
		if err != nil {
			log.Warnf("Error on retrieving secret '%s'. Message: %s", secretName, err.Error())
			clientIdStatus = models.Pending.String()
			clientSecretStatus = models.Pending.String()
		} else {
			clientIdValue := strings.TrimSpace(string(secretValue.Data[defaults.CsiAzureKeyVaultCredsClientIdPart]))
			if strings.EqualFold(clientIdValue, secretDefaultData) {
				clientIdStatus = models.Pending.String()
			}
			clientSecretValue := strings.TrimSpace(string(secretValue.Data[defaults.CsiAzureKeyVaultCredsClientSecretPart]))
			if strings.EqualFold(clientSecretValue, secretDefaultData) {
				clientSecretStatus = models.Pending.String()
			}
		}

		secrets = append(secrets, models.Secret{Name: secretName + defaults.CsiAzureKeyVaultCredsClientIdSuffix,
			DisplayName: "Client ID",
			Resource:    azureKeyVault.Name,
			Component:   component.GetName(),
			Status:      clientIdStatus,
			Type:        models.SecretTypeCsiAzureKeyVaultCreds},
		)
		secrets = append(secrets, models.Secret{Name: secretName + defaults.CsiAzureKeyVaultCredsClientSecretSuffix,
			DisplayName: "Client Secret",
			Resource:    azureKeyVault.Name,
			Component:   component.GetName(),
			Status:      clientSecretStatus,
			Type:        models.SecretTypeCsiAzureKeyVaultCreds},
		)
	}
	return secrets, nil
}

func (eh SecretHandler) getBlobFuseSecrets(component v1.RadixCommonDeployComponent, envNamespace string, volumeMount v1.RadixVolumeMount) (models.Secret, models.Secret) {
	return eh.getAzureVolumeMountSecrets(envNamespace, component,
		defaults.GetBlobFuseCredsSecretName(component.GetName(), volumeMount.Name),
		volumeMount.Name,
		defaults.BlobFuseCredsAccountNamePart,
		defaults.BlobFuseCredsAccountKeyPart,
		defaults.BlobFuseCredsAccountNamePartSuffix,
		defaults.BlobFuseCredsAccountKeyPartSuffix,
		models.SecretTypeAzureBlobFuseVolume)
}

func (eh SecretHandler) getCsiAzureSecrets(component v1.RadixCommonDeployComponent, envNamespace string, volumeMount v1.RadixVolumeMount) (models.Secret, models.Secret) {
	return eh.getAzureVolumeMountSecrets(envNamespace, component,
		defaults.GetCsiAzureCredsSecretName(component.GetName(), volumeMount.Name),
		volumeMount.Name,
		defaults.CsiAzureCredsAccountNamePart,
		defaults.CsiAzureCredsAccountKeyPart,
		defaults.CsiAzureCredsAccountNamePartSuffix,
		defaults.CsiAzureCredsAccountKeyPartSuffix,
		models.SecretTypeCsiAzureBlobVolume)
}

func (eh SecretHandler) getAzureVolumeMountSecrets(envNamespace string, component v1.RadixCommonDeployComponent, secretName, volumeMountName, accountNamePart, accountKeyPart, accountNamePartSuffix, accountKeyPartSuffix string, secretType models.SecretType) (models.Secret, models.Secret) {
	accountkeyStatus := models.Consistent.String()
	accountnameStatus := models.Consistent.String()

	secretValue, err := eh.client.CoreV1().Secrets(envNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		log.Warnf("Error on retrieving secret '%s'. Message: %s", secretName, err.Error())
		accountkeyStatus = models.Pending.String()
		accountnameStatus = models.Pending.String()
	} else {
		accountkeyValue := strings.TrimSpace(string(secretValue.Data[accountKeyPart]))
		if strings.EqualFold(accountkeyValue, secretDefaultData) {
			accountkeyStatus = models.Pending.String()
		}
		accountnameValue := strings.TrimSpace(string(secretValue.Data[accountNamePart]))
		if strings.EqualFold(accountnameValue, secretDefaultData) {
			accountnameStatus = models.Pending.String()
		}
	}
	//"accountkey"
	accountKeySecretDTO := models.Secret{Name: secretName + accountKeyPartSuffix, DisplayName: "Account Key", Resource: volumeMountName, Component: component.GetName(), Status: accountkeyStatus, Type: secretType}
	//"accountname"
	accountNameSecretDTO := models.Secret{Name: secretName + accountNamePartSuffix, DisplayName: "Account Name", Resource: volumeMountName, Component: component.GetName(), Status: accountnameStatus, Type: secretType}
	return accountKeySecretDTO, accountNameSecretDTO
}

func (eh SecretHandler) getSecretsFromVolumeMounts(activeDeployment *v1.RadixDeployment, envNamespace string) ([]models.Secret, error) {
	var secrets []models.Secret
	for _, component := range activeDeployment.Spec.Components {
		secrets = append(secrets, eh.getCredentialSecretsForBlobVolumes(&component, envNamespace)...)
	}
	for _, job := range activeDeployment.Spec.Jobs {
		secrets = append(secrets, eh.getCredentialSecretsForBlobVolumes(&job, envNamespace)...)
	}
	return secrets, nil
}

func (eh SecretHandler) getSecretsFromAuthentication(activeDeployment *v1.RadixDeployment, envNamespace string) ([]models.Secret, error) {
	var secrets []models.Secret

	for _, component := range activeDeployment.Spec.Components {
		authSecrets, err := eh.getSecretsFromComponentAuthentication(&component, envNamespace)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, authSecrets...)
	}

	return secrets, nil
}

func (eh SecretHandler) getSecretsFromComponentAuthentication(component v1.RadixCommonDeployComponent, envNamespace string) ([]models.Secret, error) {
	var secrets []models.Secret
	secrets = append(secrets, eh.getSecretsFromComponentAuthenticationClientCertificate(component, envNamespace)...)

	oauthSecrets, err := eh.getSecretsFromComponentAuthenticationOAuth2(component, envNamespace)
	if err != nil {
		return nil, err
	}
	secrets = append(secrets, oauthSecrets...)

	return secrets, nil
}

func (eh SecretHandler) getSecretRefsSecrets(radixDeployment *v1.RadixDeployment, envNamespace string) ([]models.Secret, error) {
	var secrets []models.Secret
	for _, component := range radixDeployment.Spec.Components {
		componentSecrets, err := eh.getRadixCommonComponentSecretRefs(&component, envNamespace)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, componentSecrets...)
	}
	for _, jobComponent := range radixDeployment.Spec.Jobs {
		componentSecrets, err := eh.getRadixCommonComponentSecretRefs(&jobComponent, envNamespace)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, componentSecrets...)
	}
	return secrets, nil
}

func (eh SecretHandler) getRadixCommonComponentSecretRefs(component v1.RadixCommonDeployComponent, envNamespace string) ([]models.Secret, error) {
	secrets, err := eh.getCredentialSecretsForSecretRefs(component, envNamespace)
	if err != nil {
		return nil, err
	}

	secretRefs := component.GetSecretRefs()
	for _, azureKeyVault := range secretRefs.AzureKeyVaults {
		for _, item := range azureKeyVault.Items {
			itemType := string(v1.RadixAzureKeyVaultObjectTypeSecret)
			if item.Type != nil {
				itemType = string(*item.Type)
			}
			secrets = append(secrets, models.Secret{
				Name:        item.EnvVar,
				DisplayName: fmt.Sprintf("%s '%s'", itemType, item.Name),
				Type:        models.SecretTypeCsiAzureKeyVaultItem,
				Resource:    azureKeyVault.Name,
				Component:   component.GetName(),
				Status:      models.External.String(),
			})
		}
	}

	return secrets, nil
}

func (eh SecretHandler) getSecretsFromComponentAuthenticationClientCertificate(component v1.RadixCommonDeployComponent, envNamespace string) []models.Secret {
	var secrets []models.Secret
	if auth := component.GetAuthentication(); auth != nil && component.IsPublic() && deployment.IsSecretRequiredForClientCertificate(auth.ClientCertificate) {
		secretName := operatorutils.GetComponentClientCertificateSecretName(component.GetName())
		secretStatus := models.Consistent.String()

		secret, err := eh.client.CoreV1().Secrets(envNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
		if err != nil {
			secretStatus = models.Pending.String()
		} else {
			secretValue := strings.TrimSpace(string(secret.Data["ca.crt"]))
			if strings.EqualFold(secretValue, secretDefaultData) {
				secretStatus = models.Pending.String()
			}
		}

		secrets = append(secrets, models.Secret{Name: secretName, DisplayName: "Client certificate", Type: models.SecretTypeClientCertificateAuth, Component: component.GetName(), Status: secretStatus})
	}

	return secrets
}

func (eh SecretHandler) getSecretsFromComponentAuthenticationOAuth2(component v1.RadixCommonDeployComponent, envNamespace string) ([]models.Secret, error) {
	var secrets []models.Secret
	if auth := component.GetAuthentication(); auth != nil && component.IsPublic() && auth.OAuth2 != nil {
		oauth2, err := defaults.NewOAuth2Config(defaults.WithOAuth2Defaults()).MergeWith(auth.OAuth2)
		if err != nil {
			return nil, err
		}

		clientSecretStatus := models.Consistent.String()
		cookieSecretStatus := models.Consistent.String()
		redisPasswordStatus := models.Consistent.String()

		secretName := operatorutils.GetAuxiliaryComponentSecretName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix)
		secret, err := eh.client.CoreV1().Secrets(envNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
		if err != nil {
			clientSecretStatus = models.Pending.String()
			cookieSecretStatus = models.Pending.String()
			redisPasswordStatus = models.Pending.String()
		} else {
			if secretValue, found := secret.Data[defaults.OAuthClientSecretKeyName]; !found || len(strings.TrimSpace(string(secretValue))) == 0 {
				clientSecretStatus = models.Pending.String()
			}
			if secretValue, found := secret.Data[defaults.OAuthCookieSecretKeyName]; !found || len(strings.TrimSpace(string(secretValue))) == 0 {
				cookieSecretStatus = models.Pending.String()
			}
			if secretValue, found := secret.Data[defaults.OAuthRedisPasswordKeyName]; !found || len(strings.TrimSpace(string(secretValue))) == 0 {
				redisPasswordStatus = models.Pending.String()
			}
		}

		secrets = append(secrets, models.Secret{Name: component.GetName() + suffix.OAuth2ClientSecret, DisplayName: "Client Secret", Type: models.SecretTypeOAuth2Proxy, Component: component.GetName(), Status: clientSecretStatus})
		secrets = append(secrets, models.Secret{Name: component.GetName() + suffix.OAuth2CookieSecret, DisplayName: "Cookie Secret", Type: models.SecretTypeOAuth2Proxy, Component: component.GetName(), Status: cookieSecretStatus})

		if oauth2.SessionStoreType == v1.SessionStoreRedis {
			secrets = append(secrets, models.Secret{Name: component.GetName() + suffix.OAuth2RedisPassword, DisplayName: "Redis Password", Type: models.SecretTypeOAuth2Proxy, Component: component.GetName(), Status: redisPasswordStatus})
		}
	}

	return secrets, nil
}

func (eh SecretHandler) getSecretsFromTLSCertificates(ra *v1.RadixApplication, envName, envNamespace string) (map[string]models.Secret, error) {
	secretDTOsMap := make(map[string]models.Secret)

	for _, externalAlias := range ra.Spec.DNSExternalAlias {
		if externalAlias.Environment != envName {
			continue
		}

		certStatus := models.Consistent.String()
		keyStatus := models.Consistent.String()

		secretValue, err := eh.client.CoreV1().Secrets(envNamespace).Get(context.TODO(), externalAlias.Alias, metav1.GetOptions{})
		if err != nil {
			log.Warnf("Error on retrieving secret '%s'. Message: %s", externalAlias.Alias, err.Error())
			certStatus = models.Pending.String()
			keyStatus = models.Pending.String()
		} else {
			certValue := strings.TrimSpace(string(secretValue.Data[tlsCertPart]))
			if strings.EqualFold(certValue, secretDefaultData) {
				certStatus = models.Pending.String()
			}

			keyValue := strings.TrimSpace(string(secretValue.Data[tlsKeyPart]))
			if strings.EqualFold(keyValue, secretDefaultData) {
				keyStatus = models.Pending.String()
			}
		}

		secretDTO := models.Secret{
			Name:        externalAlias.Alias + suffix.ExternalDNSCert,
			DisplayName: "Certificate",
			Resource:    externalAlias.Alias,
			Type:        models.SecretTypeClientCert,
			Component:   externalAlias.Component,
			Status:      certStatus,
		}
		secretDTOsMap[secretDTO.Name] = secretDTO

		secretDTO = models.Secret{
			Name:        externalAlias.Alias + suffix.ExternalDNSKeyPart,
			DisplayName: "Key",
			Resource:    externalAlias.Alias,
			Type:        models.SecretTypeClientCert,
			Component:   externalAlias.Component,
			Status:      keyStatus,
		}
		secretDTOsMap[secretDTO.Name] = secretDTO
	}

	return secretDTOsMap, nil
}
