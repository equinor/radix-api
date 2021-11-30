package secrets

import (
	"context"
	"fmt"
	"strings"

	"github.com/equinor/radix-api/api/deployments"
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	"github.com/equinor/radix-api/api/events"
	secretModels "github.com/equinor/radix-api/api/secrets/models"
	"github.com/equinor/radix-api/models"
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
func WithAccounts(accounts models.Accounts) SecretHandlerOptions {
	return func(eh *SecretHandler) {
		eh.client = accounts.UserAccount.Client
		eh.radixclient = accounts.UserAccount.RadixClient
		eh.inClusterClient = accounts.ServiceAccount.Client
		eh.deployHandler = deployments.Init(accounts)
		eh.eventHandler = events.Init(accounts.UserAccount.Client)
		eh.accounts = accounts
		kubeUtil, _ := kube.New(accounts.UserAccount.Client, accounts.UserAccount.RadixClient)
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
	accounts        models.Accounts
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
func (eh SecretHandler) ChangeComponentSecret(appName, envName, componentName, secretName string, componentSecret secretModels.SecretParameters) (*secretModels.SecretParameters, error) {
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
		// This is the account key part of the Csi Azure cred secret
		secretObjName = strings.TrimSuffix(secretName, defaults.CsiAzureCredsAccountKeyPartSuffix)
		partName = defaults.CsiAzureCredsAccountKeyPart

	} else if strings.HasSuffix(secretName, defaults.CsiAzureCredsAccountNamePartSuffix) {
		// This is the account name part of the Csi Azure cred secret
		secretObjName = strings.TrimSuffix(secretName, defaults.CsiAzureCredsAccountNamePartSuffix)
		partName = defaults.CsiAzureCredsAccountNamePart

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

// ChangeComponentSecretProperty handler for HandleChangeComponentSecret
func (eh SecretHandler) ChangeComponentSecretProperty(appName, envName, componentName, secretName, resource string, secretType secretModels.SecretType, componentSecret secretModels.SecretParameters) (*secretModels.SecretParameters, error) {
	//TODO change
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
		// This is the account key part of the Csi Azure cred secret
		secretObjName = strings.TrimSuffix(secretName, defaults.CsiAzureCredsAccountKeyPartSuffix)
		partName = defaults.CsiAzureCredsAccountKeyPart

	} else if strings.HasSuffix(secretName, defaults.CsiAzureCredsAccountNamePartSuffix) {
		// This is the account name part of the Csi Azure cred secret
		secretObjName = strings.TrimSuffix(secretName, defaults.CsiAzureCredsAccountNamePartSuffix)
		partName = defaults.CsiAzureCredsAccountNamePart

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

// GetSecrets Lists environment secrets for application
func (eh SecretHandler) GetSecrets(appName, envName string) ([]secretModels.Secret, error) {
	deployments, err := eh.deployHandler.GetDeploymentsForApplicationEnvironment(appName, envName, false)

	if err != nil {
		return nil, err
	}

	depl, err := eh.deployHandler.GetDeploymentWithName(appName, deployments[0].Name)
	if err != nil {
		return nil, err
	}

	return eh.GetSecretsForDeployment(appName, envName, depl)
}

// GetSecretsForDeployment Lists environment secrets for application
func (eh SecretHandler) GetSecretsForDeployment(appName, envName string, activeDeployment *deploymentModels.Deployment) ([]secretModels.Secret, error) {
	var appNamespace = k8sObjectUtils.GetAppNamespace(appName)
	var envNamespace = k8sObjectUtils.GetEnvironmentNamespace(appName, envName)
	ra, err := eh.radixclient.RadixV1().RadixApplications(appNamespace).Get(context.TODO(), appName, metav1.GetOptions{})
	if err != nil {
		return []secretModels.Secret{}, nil
	}

	rd, err := eh.radixclient.RadixV1().RadixDeployments(envNamespace).Get(context.TODO(), activeDeployment.Name, metav1.GetOptions{})
	if err != nil {
		return []secretModels.Secret{}, nil
	}

	secretsFromLatestDeployment, err := eh.getSecretsFromLatestDeployment(rd, envNamespace)
	if err != nil {
		return []secretModels.Secret{}, nil
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

	secrets := make([]secretModels.Secret, 0)
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

func (eh SecretHandler) getSecretsFromLatestDeployment(activeDeployment *v1.RadixDeployment, envNamespace string) (map[string]secretModels.Secret, error) {
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

	secretDTOsMap := make(map[string]secretModels.Secret)
	for componentName, secretNamesMap := range componentSecretsMap {
		secretObjectName := k8sObjectUtils.GetComponentSecretName(componentName)

		secret, err := eh.client.CoreV1().Secrets(envNamespace).Get(context.TODO(), secretObjectName, metav1.GetOptions{})
		if err != nil && errors.IsNotFound(err) {
			// Mark secrets as Pending (exist in config, does not exist in cluster) due to no secret object in the cluster
			for secretName := range secretNamesMap {
				secretNameAndComponentName := fmt.Sprintf("%s-%s", secretName, componentName)
				if _, exists := secretDTOsMap[secretNameAndComponentName]; !exists {
					secretDTO := secretModels.Secret{Name: secretName, DisplayName: secretName, Component: componentName, Status: environmentModels.Pending.String(), Type: secretModels.SecretTypeGeneric}
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
			secretDTO := secretModels.Secret{Name: secretName, DisplayName: secretName, Component: componentName, Status: status, Type: secretModels.SecretTypePending}
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
			secretDTO := secretModels.Secret{Name: clusterSecretName, DisplayName: clusterSecretName, Component: componentName, Status: status, Type: secretModels.SecretTypeOrphaned}
			secretDTOsMap[clusterSecretNameAndComponentName] = secretDTO
		}
	}

	return secretDTOsMap, nil
}

func (eh SecretHandler) getSecretsFromComponentObjects(component v1.RadixCommonDeployComponent, envNamespace string) ([]secretModels.Secret, error) {
	var secrets []secretModels.Secret
	secrets = append(secrets, eh.getCredentialSecretsForBlobVolumes(component, envNamespace)...)
	//secretsForSecretRefs, err := eh.getCredentialSecretsForSecretRefs(component, envNamespace)
	//if err != nil {
	//	return nil, err
	//}
	//secrets = append(secrets, secretsForSecretRefs...)
	return secrets, nil
}

func (eh SecretHandler) getCredentialSecretsForBlobVolumes(component v1.RadixCommonDeployComponent, envNamespace string) []secretModels.Secret {
	var secrets []secretModels.Secret
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

//func (eh SecretHandler) getCredentialSecretsForSecretRefs(component v1.RadixCommonDeployComponent, envNamespace string) ([]secretModels.Secret, error) {
//	var secrets []secretModels.Secret
//	for _, secretRef := range component.GetSecretRefs() {
//		if len(secretRef.AzureKeyVaults) > 0 {
//			for _, radixAzureKeyVault := range secretRef.AzureKeyVaults {
//				labelSelector := kube.GetLabelSelectorForSecretRefObject(component.GetName(), string(v1.RadixSecretRefAzureKeyVault), radixAzureKeyVault.Name)
//				credSecrets, err := eh.kubeUtil.ListSecretExistsForLabels(envNamespace, labelSelector)
//				if err != nil {
//					return nil, err
//				}
//
//				group := fmt.Sprintf("Credentials for Azure Key Vault %s", radixAzureKeyVault.Name)
//				for _, secret := range credSecrets {
//					clientIdStatus := environmentModels.Consistent.String()
//					clientSecretStatus := environmentModels.Consistent.String()
//
//					secretValue, err := eh.client.CoreV1().Secrets(envNamespace).Get(context.Background(), secret.Name, metav1.GetOptions{})
//					if err != nil {
//						log.Warnf("Error on retrieving secret '%s'. Message: %s", secretName, err.Error())
//						clientIdStatus = environmentModels.Pending.String()
//						clientSecretStatus = environmentModels.Pending.String()
//					} else {
//						clientIdValue := strings.TrimSpace(string(secretValue.Data[defaults.CsiAzureKeyVaultCredsClientIdPart]))
//						if strings.EqualFold(clientIdValue, secretDefaultData) {
//							clientIdStatus = environmentModels.Pending.String()
//						}
//						clientSecretValue := strings.TrimSpace(string(secretValue.Data[defaults.CsiAzureKeyVaultCredsClientSecretPart]))
//						if strings.EqualFold(clientSecretValue, secretDefaultData) {
//							clientSecretStatus = environmentModels.Pending.String()
//						}
//					}
//
//					secrets = append(secrets, secretModels.Secret{Name: secret.Name, DisplayName: fmt.Sprintf("Client ID"), Group: group, Component: component.GetName(), Status: clientIdStatus, Type: secretModels.SecretTypeCsiAzureKeyVault})
//					secrets = append(secrets, secretModels.Secret{Name: secret.Name, DisplayName: fmt.Sprintf("Client Secret"), Group: group, Component: component.GetName(), Status: clientSecretStatus, Type: secretModels.SecretTypeCsiAzureKeyVault})
//				}
//			}
//		}
//	}
//	return secrets, nil
//}

func (eh SecretHandler) getBlobFuseSecrets(component v1.RadixCommonDeployComponent, envNamespace string, volumeMount v1.RadixVolumeMount) (secretModels.Secret, secretModels.Secret) {
	return eh.getAzureVolumeMountSecrets(envNamespace, component,
		defaults.GetBlobFuseCredsSecretName(component.GetName(), volumeMount.Name),
		volumeMount.Name,
		defaults.BlobFuseCredsAccountNamePart,
		defaults.BlobFuseCredsAccountKeyPart,
		defaults.BlobFuseCredsAccountNamePartSuffix,
		defaults.BlobFuseCredsAccountKeyPartSuffix,
		secretModels.SecretTypeAzureBlobFuseVolume)
}

func (eh SecretHandler) getCsiAzureSecrets(component v1.RadixCommonDeployComponent, envNamespace string, volumeMount v1.RadixVolumeMount) (secretModels.Secret, secretModels.Secret) {
	return eh.getAzureVolumeMountSecrets(envNamespace, component,
		defaults.GetCsiAzureCredsSecretName(component.GetName(), volumeMount.Name),
		volumeMount.Name,
		defaults.CsiAzureCredsAccountNamePart,
		defaults.CsiAzureCredsAccountKeyPart,
		defaults.CsiAzureCredsAccountNamePartSuffix,
		defaults.CsiAzureCredsAccountKeyPartSuffix,
		secretModels.SecretTypeCsiAzureBlobVolume)
}

func (eh SecretHandler) getAzureVolumeMountSecrets(envNamespace string, component v1.RadixCommonDeployComponent, volumeMountName, secretName, accountNamePart, accountKeyPart, accountNamePartSuffix, accountKeyPartSuffix string, secretType secretModels.SecretType) (secretModels.Secret, secretModels.Secret) {
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
	//"accountkey"
	accountKeySecretDTO := secretModels.Secret{Name: secretName + accountKeyPartSuffix, DisplayName: fmt.Sprintf("Account Key"), Resource: volumeMountName, Component: component.GetName(), Status: accountkeyStatus, Type: secretType}
	//"accountname"
	accountNameSecretDTO := secretModels.Secret{Name: secretName + accountNamePartSuffix, DisplayName: fmt.Sprintf("Account Name"), Resource: volumeMountName, Component: component.GetName(), Status: accountnameStatus, Type: secretType}
	return accountKeySecretDTO, accountNameSecretDTO
}

//func (eh SecretHandler) getAzureKeyVaultSecret(component v1.RadixCommonDeployComponent, envNamespace, secretName, keyVaultNamePart string) (secretModels.Secret, secretModels.Secret) {
//	clientIdStatus := environmentModels.Consistent.String()
//	clientSecretStatus := environmentModels.Consistent.String()
//
//	secretValue, err := eh.client.CoreV1().Secrets(envNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
//	if err != nil {
//		log.Warnf("Error on retrieving secret '%s'. Message: %s", secretName, err.Error())
//		clientIdStatus = environmentModels.Pending.String()
//		clientSecretStatus = environmentModels.Pending.String()
//	} else {
//		accountkeyValue := strings.TrimSpace(string(secretValue.Data[accountKeyPart]))
//		if strings.EqualFold(accountkeyValue, secretDefaultData) {
//			clientIdStatus = environmentModels.Pending.String()
//		}
//		accountnameValue := strings.TrimSpace(string(secretValue.Data[accountNamePart]))
//		if strings.EqualFold(accountnameValue, secretDefaultData) {
//			clientSecretStatus = environmentModels.Pending.String()
//		}
//	}
//
//	accountKeySecretDTO := secretModels.Secret{Name: secretName + defaults.CsiAzureKeyVaultCredsClientSecretPart, Component: component.GetName(), Status: clientIdStatus}
//	accountNameSecretDTO := secretModels.Secret{Name: secretName + defaults.CsiAzureKeyVaultCredsClientIdPart, Component: component.GetName(), Status: clientSecretStatus}
//	return accountKeySecretDTO, accountNameSecretDTO
//}

func (eh SecretHandler) getSecretsFromVolumeMounts(activeDeployment *v1.RadixDeployment, envNamespace string) (map[string]secretModels.Secret, error) {
	secretDTOsMap := make(map[string]secretModels.Secret)

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

func (eh SecretHandler) getSecretsFromAuthenticationClientCertificate(activeDeployment *v1.RadixDeployment, envNamespace string) (map[string]secretModels.Secret, error) {
	secretDTOsMap := make(map[string]secretModels.Secret)

	for _, component := range activeDeployment.Spec.Components {
		secret := eh.getSecretsFromComponentAuthenticationClientCertificate(&component, envNamespace)
		if secret != nil {
			secretDTOsMap[secret.Name] = *secret
		}
	}

	return secretDTOsMap, nil
}

func (eh SecretHandler) getSecretsFromComponentAuthenticationClientCertificate(component v1.RadixCommonDeployComponent, envNamespace string) *secretModels.Secret {
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

		return &secretModels.Secret{
			Name:      secretName,
			Component: component.GetName(),
			Status:    secretStatus,
		}
	}

	return nil
}

func (eh SecretHandler) getSecretsFromTLSCertificates(ra *v1.RadixApplication, envName, envNamespace string) (map[string]secretModels.Secret, error) {
	secretDTOsMap := make(map[string]secretModels.Secret)

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

		secretDTO := secretModels.Secret{Name: externalAlias.Alias + certPartSuffix, Component: externalAlias.Component, Status: certStatus}
		secretDTOsMap[secretDTO.Name] = secretDTO

		secretDTO = secretModels.Secret{Name: externalAlias.Alias + keyPartSuffix, Component: externalAlias.Component, Status: keyStatus}
		secretDTOsMap[secretDTO.Name] = secretDTO
	}

	return secretDTOsMap, nil
}
