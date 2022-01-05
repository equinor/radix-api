package secrets

import (
	"context"
	"fmt"
	"strings"

	"github.com/equinor/radix-api/api/deployments"
	"github.com/equinor/radix-api/api/events"
	"github.com/equinor/radix-api/api/secrets/models"
	apiModels "github.com/equinor/radix-api/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	k8sObjectUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	secretDefaultData = "xx"
	certPartSuffix    = "-cert"
	tlsCertPart       = "tls.crt"
	keyPartSuffix     = "-key"
	tlsKeyPart        = "tls.key"
	clientCertSuffix  = "-clientcertca"
)

// SecretHandlerOptions defines a configuration function
type SecretHandlerOptions func(*SecretHandler)

// WithAccounts configures all SecretHandler fields
func WithAccounts(accounts apiModels.Accounts) SecretHandlerOptions {
	return func(eh *SecretHandler) {
		eh.client = accounts.UserAccount.Client
		eh.radixclient = accounts.UserAccount.RadixClient
		eh.inClusterClient = accounts.ServiceAccount.Client
		eh.deployHandler = deployments.Init(accounts)
		eh.eventHandler = events.Init(accounts.UserAccount.Client)
		eh.accounts = accounts
		kubeUtil, _ := kube.New(accounts.UserAccount.Client, accounts.UserAccount.RadixClient)
		kubeUtil.WithSecretsProvider(accounts.UserAccount.SecretProviderClient)
		eh.kubeUtil = kubeUtil
	}
}

// WithEventHandler configures the eventHandler used by SecretHandler
func WithEventHandler(eventHandler events.EventHandler) SecretHandlerOptions {
	return func(eh *SecretHandler) {
		eh.eventHandler = eventHandler
	}
}

// SecretHandler Instance variables
type SecretHandler struct {
	client          kubernetes.Interface
	radixclient     radixclient.Interface
	inClusterClient kubernetes.Interface
	deployHandler   deployments.DeployHandler
	eventHandler    events.EventHandler
	accounts        apiModels.Accounts
	kubeUtil        *kube.Kube
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
func (eh SecretHandler) ChangeComponentSecret(appName, envName, componentName, secretName string, componentSecret models.SecretParameters) (*models.SecretParameters, error) {
	newSecretValue := componentSecret.SecretValue
	if strings.TrimSpace(newSecretValue) == "" {
		return nil, radixhttp.ValidationError("Secret", "New secret value is empty")
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
		return nil, radixhttp.TypeMissingError("Secret object does not exist", err)
	}
	if err != nil {
		return nil, radixhttp.UnexpectedError("Failed getting secret object", err)
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

//// ChangeComponentSecretProperty handler for HandleChangeComponentSecret
//func (eh SecretHandler) ChangeComponentSecretProperty(appName, envName, componentName, secretName, resource string, secretType models.SecretType, componentSecret models.SecretParameters) (*models.SecretParameters, error) {
//	//TODO change
//}

// GetSecrets Lists environment secrets for application
func (eh SecretHandler) GetSecrets(appName, envName string) ([]models.Secret, error) {
	deployments, err := eh.deployHandler.GetDeploymentsForApplicationEnvironment(appName, envName, false)

	if err != nil {
		return nil, err
	}

	depl, err := eh.deployHandler.GetDeploymentWithName(appName, deployments[0].Name)
	if err != nil {
		return nil, err
	}

	return eh.GetSecretsForDeployment(appName, envName, depl.Name)
}

// GetSecretsForDeployment Lists environment secrets for application
func (eh SecretHandler) GetSecretsForDeployment(appName, envName, deploymentName string) ([]models.Secret, error) {
	var appNamespace = k8sObjectUtils.GetAppNamespace(appName)
	var envNamespace = k8sObjectUtils.GetEnvironmentNamespace(appName, envName)
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

	secretsFromAuthenticationClientCertificate, err := eh.getSecretsFromAuthenticationClientCertificate(rd, envNamespace)
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

	for _, secretFromAuthenticationClientCertificate := range secretsFromAuthenticationClientCertificate {
		secrets = append(secrets, secretFromAuthenticationClientCertificate)
	}

	for _, secretRefsSecret := range secretRefsSecrets {
		secrets = append(secrets, secretRefsSecret)
	}

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
		secretObjectName := k8sObjectUtils.GetComponentSecretName(componentName)

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
			secretDTO := models.Secret{Name: secretName, DisplayName: secretName, Component: componentName, Status: status, Type: models.SecretTypePending}
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

func (eh SecretHandler) getSecretsFromComponentObjects(component v1.RadixCommonDeployComponent, envNamespace string) ([]models.Secret, error) {
	var secrets []models.Secret
	secrets = append(secrets, eh.getCredentialSecretsForBlobVolumes(component, envNamespace)...)
	secretsForSecretRefs, err := eh.getCredentialSecretsForSecretRefs(component, envNamespace)
	if err != nil {
		return nil, err
	}
	secrets = append(secrets, secretsForSecretRefs...)
	return secrets, nil
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
			DisplayName: fmt.Sprintf("Client ID"),
			Resource:    azureKeyVault.Name,
			Component:   component.GetName(),
			Status:      clientIdStatus,
			Type:        models.SecretTypeCsiAzureKeyVaultCreds},
		)
		secrets = append(secrets, models.Secret{Name: secretName + defaults.CsiAzureKeyVaultCredsClientSecretSuffix,
			DisplayName: fmt.Sprintf("Client Secret"),
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
	accountKeySecretDTO := models.Secret{Name: secretName + accountKeyPartSuffix, DisplayName: fmt.Sprintf("Account Key"), Resource: volumeMountName, Component: component.GetName(), Status: accountkeyStatus, Type: secretType}
	//"accountname"
	accountNameSecretDTO := models.Secret{Name: secretName + accountNamePartSuffix, DisplayName: fmt.Sprintf("Account Name"), Resource: volumeMountName, Component: component.GetName(), Status: accountnameStatus, Type: secretType}
	return accountKeySecretDTO, accountNameSecretDTO
}

//func (eh SecretHandler) getAzureKeyVaultSecret(component v1.RadixCommonDeployComponent, envNamespace, secretName, keyVaultNamePart string) (models.Secret, models.Secret) {
//	clientIdStatus := models.Consistent.String()
//	clientSecretStatus := models.Consistent.String()
//
//	secretValue, err := eh.client.CoreV1().Secrets(envNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
//	if err != nil {
//		log.Warnf("Error on retrieving secret '%s'. Message: %s", secretName, err.Error())
//		clientIdStatus = models.Pending.String()
//		clientSecretStatus = models.Pending.String()
//	} else {
//		accountkeyValue := strings.TrimSpace(string(secretValue.Data[accountKeyPart]))
//		if strings.EqualFold(accountkeyValue, secretDefaultData) {
//			clientIdStatus = models.Pending.String()
//		}
//		accountnameValue := strings.TrimSpace(string(secretValue.Data[accountNamePart]))
//		if strings.EqualFold(accountnameValue, secretDefaultData) {
//			clientSecretStatus = models.Pending.String()
//		}
//	}
//
//	accountKeySecretDTO := models.Secret{Name: secretName + defaults.CsiAzureKeyVaultCredsClientSecretPart, Component: component.GetName(), Status: clientIdStatus}
//	accountNameSecretDTO := models.Secret{Name: secretName + defaults.CsiAzureKeyVaultCredsClientIdPart, Component: component.GetName(), Status: clientSecretStatus}
//	return accountKeySecretDTO, accountNameSecretDTO
//}

func (eh SecretHandler) getSecretsFromVolumeMounts(activeDeployment *v1.RadixDeployment, envNamespace string) (map[string]models.Secret, error) {
	secretDTOsMap := make(map[string]models.Secret)

	for _, component := range activeDeployment.Spec.Components {
		secrets, err := eh.getSecretsFromComponentObjects(&component, envNamespace)
		if err != nil {
			return nil, err
		}
		for _, secret := range secrets {
			secretDTOsMap[secret.Name] = secret
		}
	}

	for _, job := range activeDeployment.Spec.Jobs {
		secrets, err := eh.getSecretsFromComponentObjects(&job, envNamespace)
		if err != nil {
			return nil, err
		}
		for _, secret := range secrets {
			secretDTOsMap[secret.Name] = secret
		}
	}

	return secretDTOsMap, nil
}

func (eh SecretHandler) getSecretsFromAuthenticationClientCertificate(activeDeployment *v1.RadixDeployment, envNamespace string) (map[string]models.Secret, error) {
	secretDTOsMap := make(map[string]models.Secret)

	for _, component := range activeDeployment.Spec.Components {
		secret := eh.getSecretsFromComponentAuthenticationClientCertificate(&component, envNamespace)
		if secret != nil {
			secretDTOsMap[secret.Name] = *secret
		}
	}

	return secretDTOsMap, nil
}

func (eh SecretHandler) getSecretRefsSecrets(radixDeployment *v1.RadixDeployment, envNamespace string) (map[string]models.Secret, error) {
	secretsMap := make(map[string]models.Secret)
	for _, component := range radixDeployment.Spec.Components {
		secretRefs := component.GetSecretRefs()
		for _, azureKeyVault := range secretRefs.AzureKeyVaults {
			for _, item := range azureKeyVault.Items {
				itemType := string(v1.RadixAzureKeyVaultObjectTypeSecret)
				if item.Type != nil {
					itemType = string(*item.Type)
				}
				secret := &models.Secret{
					Name:        item.EnvVar,
					DisplayName: fmt.Sprintf("%s '%s'", itemType, item.Name),
					Type:        models.SecretTypeCsiAzureKeyVaultItem,
					Resource:    azureKeyVault.Name,
					Component:   component.GetName(),
					Status:      models.External.String(),
				}
				secretsMap[secret.Name] = *secret
			}
		}
	}
	return secretsMap, nil
}

func (eh SecretHandler) getSecretsFromComponentAuthenticationClientCertificate(component v1.RadixCommonDeployComponent, envNamespace string) *models.Secret {
	if auth := component.GetAuthentication(); auth != nil && component.GetPublicPort() != "" && deployment.IsSecretRequiredForClientCertificate(auth.ClientCertificate) {
		secretName := k8sObjectUtils.GetComponentClientCertificateSecretName(component.GetName())
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

		return &models.Secret{
			Name:      secretName,
			Component: component.GetName(),
			Status:    secretStatus,
		}
	}

	return nil
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

		secretDTO := models.Secret{Name: externalAlias.Alias + certPartSuffix, Component: externalAlias.Component, Status: certStatus}
		secretDTOsMap[secretDTO.Name] = secretDTO

		secretDTO = models.Secret{Name: externalAlias.Alias + keyPartSuffix, Component: externalAlias.Component, Status: keyStatus}
		secretDTOsMap[secretDTO.Name] = secretDTO
	}

	return secretDTOsMap, nil
}
