package secrets

import (
	"context"
	"fmt"
	"strings"

	"github.com/equinor/radix-api/api/deployments"
	"github.com/equinor/radix-api/api/secrets/models"
	"github.com/equinor/radix-api/api/secrets/suffix"
	"github.com/equinor/radix-api/api/utils/labelselector"
	"github.com/equinor/radix-api/api/utils/secret"
	sortUtils "github.com/equinor/radix-api/api/utils/sort"
	"github.com/equinor/radix-api/api/utils/tlsvalidator"
	apiModels "github.com/equinor/radix-api/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

const (
	secretDefaultData          = "xx"
	secretStoreCsiManagedLabel = "secrets-store.csi.k8s.io/managed"
	k8sJobNameLabel            = "job-name" // A label that k8s automatically adds to a Pod created by a Job
)

// var (
// 	defaultTlsSecretValidator = tlsSecretValidator{}
// )

type podNameToSecretVersionMap map[string]string
type secretIdToPodNameToSecretVersionMap map[string]podNameToSecretVersionMap

// SecretHandlerOptions defines a configuration function
type SecretHandlerOptions func(*SecretHandler)

// WithAccounts configures all SecretHandler fields
func WithAccounts(accounts apiModels.Accounts) SecretHandlerOptions {
	return func(eh *SecretHandler) {
		eh.userAccount = accounts.UserAccount
		eh.serviceAccount = accounts.ServiceAccount
		eh.deployHandler = deployments.Init(accounts)
	}
}

// SecretHandler Instance variables
type SecretHandler struct {
	userAccount        apiModels.Account
	serviceAccount     apiModels.Account
	deployHandler      deployments.DeployHandler
	tlsSecretValidator tlsvalidator.Interface
}

// Init Constructor.
// Use the WithAccounts configuration function to configure a 'ready to use' SecretHandler.
// SecretHandlerOptions are processed in the sequence they are passed to this function.
func Init(opts ...SecretHandlerOptions) SecretHandler {
	eh := SecretHandler{}

	for _, opt := range opts {
		opt(&eh)
	}

	return eh
}

// ChangeComponentSecret handler for HandleChangeComponentSecret
func (eh SecretHandler) ChangeComponentSecret(ctx context.Context, appName, envName, componentName, secretName string, componentSecret models.SecretParameters) error {
	newSecretValue := componentSecret.SecretValue
	if strings.TrimSpace(newSecretValue) == "" {
		return radixhttp.ValidationError("Secret", "New secret value is empty")
	}

	ns := operatorutils.GetEnvironmentNamespace(appName, envName)

	var secretObjName, partName string

	if strings.HasSuffix(secretName, suffix.ExternalDNSTLSCert) {
		// This is the cert part of the TLS secret
		secretObjName = strings.TrimSuffix(secretName, suffix.ExternalDNSTLSCert)
		partName = corev1.TLSCertKey

	} else if strings.HasSuffix(secretName, suffix.ExternalDNSTLSKey) {
		// This is the key part of the TLS secret
		secretObjName = strings.TrimSuffix(secretName, suffix.ExternalDNSTLSKey)
		partName = corev1.TLSPrivateKeyKey

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

	secretObject, err := eh.userAccount.Client.CoreV1().Secrets(ns).Get(ctx, secretObjName, metav1.GetOptions{})
	if err != nil && k8sErrors.IsNotFound(err) {
		return radixhttp.TypeMissingError("Secret object does not exist", err)
	}
	if err != nil {
		return radixhttp.UnexpectedError("Failed getting secret object", err)
	}

	if secretObject.Data == nil {
		secretObject.Data = make(map[string][]byte)
	}

	secretObject.Data[partName] = []byte(newSecretValue)

	_, err = eh.userAccount.Client.CoreV1().Secrets(ns).Update(ctx, secretObject, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

// GetSecretsForDeployment Lists environment secrets for application
func (eh SecretHandler) GetSecretsForDeployment(ctx context.Context, appName, envName, deploymentName string) ([]models.Secret, error) {
	var envNamespace = operatorutils.GetEnvironmentNamespace(appName, envName)
	var secrets []models.Secret

	rd, err := eh.userAccount.RadixClient.RadixV1().RadixDeployments(envNamespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	secretsFromLatestDeployment, err := eh.getSecretsFromLatestDeployment(ctx, rd, envNamespace)
	if err != nil {
		return nil, err
	}
	for _, secretFromLatestDeployment := range secretsFromLatestDeployment {
		secrets = append(secrets, secretFromLatestDeployment)
	}

	secrets = append(secrets, eh.getSecretsFromTLSCertificates(ctx, rd, envNamespace)...)

	secretsFromVolumeMounts, err := eh.getSecretsFromVolumeMounts(ctx, rd, envNamespace)
	if err != nil {
		return nil, err
	}
	secrets = append(secrets, secretsFromVolumeMounts...)

	secretsFromAuthentication, err := eh.getSecretsFromAuthentication(ctx, rd, envNamespace)
	if err != nil {
		return nil, err
	}
	secrets = append(secrets, secretsFromAuthentication...)

	secretRefsSecrets, err := eh.getSecretRefsSecrets(appName, rd, envNamespace)
	if err != nil {
		return nil, err
	}
	secrets = append(secrets, secretRefsSecrets...)

	return secrets, nil
}

func (eh SecretHandler) getSecretsForComponent(component radixv1.RadixCommonDeployComponent) map[string]bool {
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

func (eh SecretHandler) getSecretsFromLatestDeployment(ctx context.Context, activeDeployment *radixv1.RadixDeployment, envNamespace string) (map[string]models.Secret, error) {
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

		secret, err := eh.userAccount.Client.CoreV1().Secrets(envNamespace).Get(ctx, secretObjectName, metav1.GetOptions{})
		if err != nil && k8sErrors.IsNotFound(err) {
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
	}

	return secretDTOsMap, nil
}

func (eh SecretHandler) getCredentialSecretsForBlobVolumes(ctx context.Context, component radixv1.RadixCommonDeployComponent, envNamespace string) []models.Secret {
	var secrets []models.Secret
	for _, volumeMount := range component.GetVolumeMounts() {
		switch volumeMount.Type {
		case radixv1.MountTypeBlob:
			accountKeySecret, accountNameSecret := eh.getBlobFuseSecrets(ctx, component, envNamespace, volumeMount)
			secrets = append(secrets, accountKeySecret)
			secrets = append(secrets, accountNameSecret)
		case radixv1.MountTypeBlobCsiAzure, radixv1.MountTypeBlob2CsiAzure, radixv1.MountTypeNfsCsiAzure, radixv1.MountTypeFileCsiAzure:
			accountKeySecret, accountNameSecret := eh.getCsiAzureSecrets(ctx, component, envNamespace, volumeMount)
			secrets = append(secrets, accountKeySecret)
			secrets = append(secrets, accountNameSecret)
		}
	}
	return secrets
}

func (eh SecretHandler) getCredentialSecretsForSecretRefsAzureKeyVault(envNamespace, componentName, azureKeyVaultName string) ([]models.Secret, error) {
	var secrets []models.Secret
	secretName := defaults.GetCsiAzureKeyVaultCredsSecretName(componentName, azureKeyVaultName)
	clientIdStatus := models.Consistent.String()
	clientSecretStatus := models.Consistent.String()

	secretValue, err := eh.userAccount.Client.CoreV1().Secrets(envNamespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		log.Warnf("Error on retrieving secret %s. Message: %s", secretName, err.Error())
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
	secrets = append(secrets, models.Secret{
		Name:        secretName + defaults.CsiAzureKeyVaultCredsClientIdSuffix,
		DisplayName: "Client ID",
		Resource:    azureKeyVaultName,
		Component:   componentName,
		Status:      clientIdStatus,
		Type:        models.SecretTypeCsiAzureKeyVaultCreds,
		ID:          models.SecretIdClientId},
	)
	secrets = append(secrets, models.Secret{
		Name:        secretName + defaults.CsiAzureKeyVaultCredsClientSecretSuffix,
		DisplayName: "Client Secret",
		Resource:    azureKeyVaultName,
		Component:   componentName,
		Status:      clientSecretStatus,
		Type:        models.SecretTypeCsiAzureKeyVaultCreds,
		ID:          models.SecretIdClientSecret},
	)
	return secrets, nil
}

func (eh SecretHandler) getBlobFuseSecrets(ctx context.Context, component radixv1.RadixCommonDeployComponent, envNamespace string, volumeMount radixv1.RadixVolumeMount) (models.Secret, models.Secret) {
	return eh.getAzureVolumeMountSecrets(ctx, envNamespace, component,
		defaults.GetBlobFuseCredsSecretName(component.GetName(), volumeMount.Name),
		volumeMount.Name,
		defaults.BlobFuseCredsAccountNamePart,
		defaults.BlobFuseCredsAccountKeyPart,
		defaults.BlobFuseCredsAccountNamePartSuffix,
		defaults.BlobFuseCredsAccountKeyPartSuffix,
		models.SecretTypeAzureBlobFuseVolume)
}

func (eh SecretHandler) getCsiAzureSecrets(ctx context.Context, component radixv1.RadixCommonDeployComponent, envNamespace string, volumeMount radixv1.RadixVolumeMount) (models.Secret, models.Secret) {
	return eh.getAzureVolumeMountSecrets(ctx, envNamespace, component,
		defaults.GetCsiAzureVolumeMountCredsSecretName(component.GetName(), volumeMount.Name),
		volumeMount.Name,
		defaults.CsiAzureCredsAccountNamePart,
		defaults.CsiAzureCredsAccountKeyPart,
		defaults.CsiAzureCredsAccountNamePartSuffix,
		defaults.CsiAzureCredsAccountKeyPartSuffix,
		models.SecretTypeCsiAzureBlobVolume)
}

func (eh SecretHandler) getAzureVolumeMountSecrets(ctx context.Context, envNamespace string, component radixv1.RadixCommonDeployComponent, secretName, volumeMountName, accountNamePart, accountKeyPart, accountNamePartSuffix, accountKeyPartSuffix string, secretType models.SecretType) (models.Secret, models.Secret) {
	accountkeyStatus := models.Consistent.String()
	accountnameStatus := models.Consistent.String()

	secretValue, err := eh.userAccount.Client.CoreV1().Secrets(envNamespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		log.Warnf("Error on retrieving secret %s. Message: %s", secretName, err.Error())
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
	// "accountkey"
	accountKeySecretDTO := models.Secret{
		Name:        secretName + accountKeyPartSuffix,
		DisplayName: "Account Key",
		Resource:    volumeMountName,
		Component:   component.GetName(),
		Status:      accountkeyStatus,
		Type:        secretType,
		ID:          models.SecretIdAccountKey}
	// "accountname"
	accountNameSecretDTO := models.Secret{
		Name:        secretName + accountNamePartSuffix,
		DisplayName: "Account Name",
		Resource:    volumeMountName,
		Component:   component.GetName(),
		Status:      accountnameStatus,
		Type:        secretType,
		ID:          models.SecretIdAccountName}
	return accountKeySecretDTO, accountNameSecretDTO
}

func (eh SecretHandler) getSecretsFromVolumeMounts(ctx context.Context, activeDeployment *radixv1.RadixDeployment, envNamespace string) ([]models.Secret, error) {
	var secrets []models.Secret
	for _, component := range activeDeployment.Spec.Components {
		secrets = append(secrets, eh.getCredentialSecretsForBlobVolumes(ctx, &component, envNamespace)...)
	}
	for _, job := range activeDeployment.Spec.Jobs {
		secrets = append(secrets, eh.getCredentialSecretsForBlobVolumes(ctx, &job, envNamespace)...)
	}
	return secrets, nil
}

func (eh SecretHandler) getSecretsFromAuthentication(ctx context.Context, activeDeployment *radixv1.RadixDeployment, envNamespace string) ([]models.Secret, error) {
	var secrets []models.Secret

	for _, component := range activeDeployment.Spec.Components {
		authSecrets, err := eh.getSecretsFromComponentAuthentication(ctx, &component, envNamespace)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, authSecrets...)
	}

	return secrets, nil
}

func (eh SecretHandler) getSecretsFromComponentAuthentication(ctx context.Context, component radixv1.RadixCommonDeployComponent, envNamespace string) ([]models.Secret, error) {
	var secrets []models.Secret
	secrets = append(secrets, eh.getSecretsFromComponentAuthenticationClientCertificate(ctx, component, envNamespace)...)

	oauthSecrets, err := eh.getSecretsFromComponentAuthenticationOAuth2(ctx, component, envNamespace)
	if err != nil {
		return nil, err
	}
	secrets = append(secrets, oauthSecrets...)

	return secrets, nil
}

func (eh SecretHandler) getSecretRefsSecrets(appName string, radixDeployment *radixv1.RadixDeployment, envNamespace string) ([]models.Secret, error) {
	secretProviderClassMapForDeployment, err := eh.getAzureKeyVaultSecretProviderClassMapForAppDeployment(appName, envNamespace, radixDeployment.GetName())
	if err != nil {
		return nil, err
	}
	csiSecretStoreSecretMap, err := eh.getCsiSecretStoreSecretMap(envNamespace)
	if err != nil {
		return nil, err
	}
	var secrets []models.Secret
	for _, component := range radixDeployment.Spec.Components {
		secretRefs := component.GetSecretRefs()
		componentSecrets, err := eh.getComponentSecretRefsSecrets(envNamespace, component.GetName(), &secretRefs, secretProviderClassMapForDeployment, csiSecretStoreSecretMap)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, componentSecrets...)
	}
	for _, jobComponent := range radixDeployment.Spec.Jobs {
		secretRefs := jobComponent.GetSecretRefs()
		jobComponentSecrets, err := eh.getComponentSecretRefsSecrets(envNamespace, jobComponent.GetName(), &secretRefs, secretProviderClassMapForDeployment, csiSecretStoreSecretMap)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, jobComponentSecrets...)
	}
	return secrets, nil
}

func (eh SecretHandler) getComponentSecretRefsSecrets(envNamespace string, componentName string, secretRefs *radixv1.RadixSecretRefs,
	secretProviderClassMap map[string]secretsstorev1.SecretProviderClass, csiSecretStoreSecretMap map[string]corev1.Secret) ([]models.Secret, error) {
	var secrets []models.Secret
	for _, azureKeyVault := range secretRefs.AzureKeyVaults {
		if azureKeyVault.UseAzureIdentity == nil || !*azureKeyVault.UseAzureIdentity {
			credSecrets, err := eh.getCredentialSecretsForSecretRefsAzureKeyVault(envNamespace, componentName, azureKeyVault.Name)
			if err != nil {
				return nil, err
			}
			secrets = append(secrets, credSecrets...)
		}
		secretStatus := getAzureKeyVaultSecretStatus(componentName, azureKeyVault.Name, secretProviderClassMap, csiSecretStoreSecretMap)
		for _, item := range azureKeyVault.Items {
			secrets = append(secrets, models.Secret{
				Name:        secret.GetSecretNameForAzureKeyVaultItem(componentName, azureKeyVault.Name, &item),
				DisplayName: secret.GetSecretDisplayNameForAzureKeyVaultItem(&item),
				Type:        models.SecretTypeCsiAzureKeyVaultItem,
				Resource:    azureKeyVault.Name,
				Component:   componentName,
				Status:      secretStatus,
				ID:          secret.GetSecretIdForAzureKeyVaultItem(&item),
			})
		}
	}
	return secrets, nil
}

func getAzureKeyVaultSecretStatus(componentName, azureKeyVaultName string, secretProviderClassMap map[string]secretsstorev1.SecretProviderClass, csiSecretStoreSecretMap map[string]corev1.Secret) string {
	secretStatus := models.NotAvailable.String()
	secretProviderClass := getComponentSecretProviderClassMapForAzureKeyVault(componentName, secretProviderClassMap, azureKeyVaultName)
	if secretProviderClass != nil {
		secretStatus = models.Consistent.String()
		for _, secretObject := range secretProviderClass.Spec.SecretObjects {
			if _, ok := csiSecretStoreSecretMap[secretObject.SecretName]; !ok {
				secretStatus = models.NotAvailable.String() // Secrets does not exist for the secretProviderClass secret object
				break
			}
		}
	}
	return secretStatus
}

func getComponentSecretProviderClassMapForAzureKeyVault(componentName string, componentSecretProviderClassMap map[string]secretsstorev1.SecretProviderClass, azureKeyVaultName string) *secretsstorev1.SecretProviderClass {
	for _, secretProviderClass := range componentSecretProviderClassMap {
		if strings.EqualFold(secretProviderClass.ObjectMeta.Labels[kube.RadixComponentLabel], componentName) &&
			strings.EqualFold(secretProviderClass.ObjectMeta.Labels[kube.RadixSecretRefNameLabel], azureKeyVaultName) {
			return &secretProviderClass
		}
	}
	return nil
}

func (eh SecretHandler) getAzureKeyVaultSecretVersionsMap(appName, envNamespace, componentName, azureKeyVaultName string) (secretIdToPodNameToSecretVersionMap, error) {
	secretProviderClassMap, err := eh.getAzureKeyVaultSecretProviderClassMapForAppComponentStorage(appName, envNamespace, componentName, azureKeyVaultName)
	if err != nil {
		return nil, err
	}
	secretsInPodStatusList, err := eh.serviceAccount.SecretProviderClient.SecretsstoreV1().SecretProviderClassPodStatuses(envNamespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	secretStatusMap := make(secretIdToPodNameToSecretVersionMap)
	for _, secretsInPod := range secretsInPodStatusList.Items {
		if _, ok := secretProviderClassMap[secretsInPod.Status.SecretProviderClassName]; !ok {
			continue
		}
		for _, secretVersion := range secretsInPod.Status.Objects {
			if _, ok := secretStatusMap[secretVersion.ID]; !ok {
				secretStatusMap[secretVersion.ID] = make(podNameToSecretVersionMap)
			}
			secretStatusMap[secretVersion.ID][secretsInPod.Status.PodName] = secretVersion.Version
		}
	}
	return secretStatusMap, nil // map[secretType/secretName][podName]secretVersion
}

func (eh SecretHandler) getAzureKeyVaultSecretProviderClassMapForAppDeployment(appName, envNamespace, deploymentName string) (map[string]secretsstorev1.SecretProviderClass, error) {
	labelSelector := labels.Set{
		kube.RadixAppLabel:           appName,
		kube.RadixDeploymentLabel:    deploymentName,
		kube.RadixSecretRefTypeLabel: string(radixv1.RadixSecretRefTypeAzureKeyVault),
	}.String()
	return eh.getSecretProviderClassMapForLabelSelector(envNamespace, labelSelector)
}

func (eh SecretHandler) getAzureKeyVaultSecretProviderClassMapForAppComponentStorage(appName, envNamespace, componentName, azureKeyVaultName string) (map[string]secretsstorev1.SecretProviderClass, error) {
	labelSelector := getAzureKeyVaultSecretRefSecretProviderClassLabels(appName, componentName, azureKeyVaultName).String()
	return eh.getSecretProviderClassMapForLabelSelector(envNamespace, labelSelector)
}

func getAzureKeyVaultSecretRefSecretProviderClassLabels(appName string, componentName string, azureKeyVaultName string) labels.Set {
	return labels.Set{
		kube.RadixAppLabel:           appName,
		kube.RadixComponentLabel:     componentName,
		kube.RadixSecretRefNameLabel: strings.ToLower(azureKeyVaultName),
		kube.RadixSecretRefTypeLabel: string(radixv1.RadixSecretRefTypeAzureKeyVault),
	}
}

func (eh SecretHandler) getSecretProviderClassMapForLabelSelector(envNamespace, labelSelector string) (map[string]secretsstorev1.SecretProviderClass, error) {
	secretProviderClassList, err := eh.serviceAccount.SecretProviderClient.SecretsstoreV1().SecretProviderClasses(envNamespace).
		List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, err
	}
	secretProviderClassMap := make(map[string]secretsstorev1.SecretProviderClass)
	for _, secretProviderClass := range secretProviderClassList.Items {
		secretProviderClass := secretProviderClass
		secretProviderClassMap[secretProviderClass.GetName()] = secretProviderClass
	}
	return secretProviderClassMap, nil
}

func (eh SecretHandler) getSecretsFromComponentAuthenticationClientCertificate(ctx context.Context, component radixv1.RadixCommonDeployComponent, envNamespace string) []models.Secret {
	var secrets []models.Secret
	if auth := component.GetAuthentication(); auth != nil && component.IsPublic() && deployment.IsSecretRequiredForClientCertificate(auth.ClientCertificate) {
		secretName := operatorutils.GetComponentClientCertificateSecretName(component.GetName())
		secretStatus := models.Consistent.String()

		secret, err := eh.userAccount.Client.CoreV1().Secrets(envNamespace).Get(ctx, secretName, metav1.GetOptions{})
		if err != nil {
			secretStatus = models.Pending.String()
		} else {
			secretValue := strings.TrimSpace(string(secret.Data["ca.crt"]))
			if strings.EqualFold(secretValue, secretDefaultData) {
				secretStatus = models.Pending.String()
			}
		}

		secrets = append(secrets, models.Secret{Name: secretName,
			DisplayName: "",
			Type:        models.SecretTypeClientCertificateAuth, Component: component.GetName(), Status: secretStatus})
	}

	return secrets
}

func (eh SecretHandler) getSecretsFromComponentAuthenticationOAuth2(ctx context.Context, component radixv1.RadixCommonDeployComponent, envNamespace string) ([]models.Secret, error) {
	var secrets []models.Secret
	if auth := component.GetAuthentication(); component.IsPublic() && auth != nil && auth.OAuth2 != nil {
		oauth2, err := defaults.NewOAuth2Config(defaults.WithOAuth2Defaults()).MergeWith(auth.OAuth2)
		if err != nil {
			return nil, err
		}

		clientSecretStatus := models.Consistent.String()
		cookieSecretStatus := models.Consistent.String()
		redisPasswordStatus := models.Consistent.String()

		secretName := operatorutils.GetAuxiliaryComponentSecretName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix)
		secret, err := eh.userAccount.Client.CoreV1().Secrets(envNamespace).Get(ctx, secretName, metav1.GetOptions{})
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

		secrets = append(secrets, models.Secret{Name: component.GetName() + suffix.OAuth2ClientSecret,
			DisplayName: "Client Secret",
			Type:        models.SecretTypeOAuth2Proxy, Component: component.GetName(),
			Status: clientSecretStatus})
		secrets = append(secrets, models.Secret{Name: component.GetName() + suffix.OAuth2CookieSecret,
			DisplayName: "Cookie Secret",
			Type:        models.SecretTypeOAuth2Proxy, Component: component.GetName(), Status: cookieSecretStatus})

		if oauth2.SessionStoreType == radixv1.SessionStoreRedis {
			secrets = append(secrets, models.Secret{Name: component.GetName() + suffix.OAuth2RedisPassword,
				DisplayName: "Redis Password",
				Type:        models.SecretTypeOAuth2Proxy, Component: component.GetName(), Status: redisPasswordStatus})
		}
	}

	return secrets, nil
}

func (eh SecretHandler) getSecretsFromTLSCertificates(ctx context.Context, rd *radixv1.RadixDeployment, envNamespace string) []models.Secret {
	var secrets []models.Secret
	tlsValidator := eh.tlsSecretValidator
	if tlsValidator == nil {
		tlsValidator = tlsvalidator.DefaultValidator()
	}

	for _, component := range rd.Spec.Components {
		for _, externalAlias := range component.DNSExternalAlias {
			var certData, keyData []byte
			certStatus := models.Consistent
			keyStatus := models.Consistent

			secretValue, err := eh.userAccount.Client.CoreV1().Secrets(envNamespace).Get(ctx, externalAlias, metav1.GetOptions{})
			if err != nil {
				log.Warnf("Error on retrieving secret %s. Message: %s", externalAlias, err.Error())
				certStatus = models.Pending
				keyStatus = models.Pending
			} else {
				certData = secretValue.Data[corev1.TLSCertKey]
				if certValue := strings.TrimSpace(string(certData)); len(certValue) == 0 || strings.EqualFold(certValue, secretDefaultData) {
					certStatus = models.Pending
					certData = nil
				}

				keyData = secretValue.Data[corev1.TLSPrivateKeyKey]
				if keyValue := strings.TrimSpace(string(keyData)); len(keyValue) == 0 || strings.EqualFold(keyValue, secretDefaultData) {
					keyStatus = models.Pending
					keyData = nil
				}
			}

			var tlsCerts []models.TLSCertificate
			var certStatusMessages []string
			if certStatus == models.Consistent {
				tlsCerts = append(tlsCerts, models.ParseTLSCertificatesFromPEM(certData)...)

				if certIsValid, messages := tlsValidator.ValidateTLSCertificate(certData, keyData, externalAlias); !certIsValid {
					certStatus = models.Invalid
					certStatusMessages = append(certStatusMessages, messages...)
				}
			}

			var keyStatusMessages []string
			if keyStatus == models.Consistent {
				if keyIsValid, messages := tlsValidator.ValidateTLSKey(keyData); !keyIsValid {
					keyStatus = models.Invalid
					keyStatusMessages = append(keyStatusMessages, messages...)
				}
			}

			secrets = append(secrets,
				models.Secret{
					Name:            externalAlias + suffix.ExternalDNSTLSCert,
					DisplayName:     "Certificate",
					Resource:        externalAlias,
					Type:            models.SecretTypeClientCert,
					Component:       component.GetName(),
					Status:          certStatus.String(),
					ID:              models.SecretIdCert,
					StatusMessages:  certStatusMessages,
					TLSCertificates: tlsCerts,
				},
				models.Secret{
					Name:           externalAlias + suffix.ExternalDNSTLSKey,
					DisplayName:    "Key",
					Resource:       externalAlias,
					Type:           models.SecretTypeClientCert,
					Component:      component.GetName(),
					Status:         keyStatus.String(),
					StatusMessages: keyStatusMessages,
					ID:             models.SecretIdKey,
				},
			)
		}
	}

	return secrets
}

// GetAzureKeyVaultSecretVersions Gets list of Azure Key vault secret versions for the storage in the component
func (eh SecretHandler) GetAzureKeyVaultSecretVersions(appName, envName, componentName, azureKeyVaultName, secretId string) ([]models.AzureKeyVaultSecretVersion, error) {
	var envNamespace = operatorutils.GetEnvironmentNamespace(appName, envName)
	azureKeyVaultSecretMap, err := eh.getAzureKeyVaultSecretVersionsMap(appName, envNamespace, componentName, azureKeyVaultName)
	if err != nil {
		return nil, err
	}
	podList, err := eh.userAccount.Client.CoreV1().Pods(envNamespace).List(context.Background(), metav1.ListOptions{LabelSelector: labelselector.ForComponent(appName, componentName).String()})
	if err != nil {
		return nil, err
	}
	sortUtils.Pods(podList.Items, sortUtils.ByPodCreationTimestamp, sortUtils.Descending)
	return eh.getAzKeyVaultSecretVersions(appName, envNamespace, componentName, podList.Items, azureKeyVaultSecretMap[secretId])
}

func (eh SecretHandler) getAzKeyVaultSecretVersions(appName string, envNamespace string, componentName string, pods []corev1.Pod, podSecretVersionMap podNameToSecretVersionMap) ([]models.AzureKeyVaultSecretVersion, error) {
	jobMap, err := eh.getJobMap(appName, envNamespace, componentName)
	if err != nil {
		return nil, err
	}
	var azKeyVaultSecretVersions []models.AzureKeyVaultSecretVersion
	for _, pod := range pods {
		secretVersion, ok := podSecretVersionMap[pod.GetName()]
		if !ok {
			continue
		}
		podCreated := pod.GetCreationTimestamp()
		azureKeyVaultSecretVersion := models.AzureKeyVaultSecretVersion{
			ReplicaName:    pod.GetName(),
			ReplicaCreated: radixutils.FormatTime(&podCreated),
			Version:        secretVersion,
		}
		if _, ok := pod.ObjectMeta.Labels[kube.RadixPodIsJobAuxObjectLabel]; ok {
			azureKeyVaultSecretVersion.ReplicaName = "New jobs"
			azKeyVaultSecretVersions = append(azKeyVaultSecretVersions, azureKeyVaultSecretVersion)
			continue
		}
		if !strings.EqualFold(pod.ObjectMeta.Labels[kube.RadixJobTypeLabel], kube.RadixJobTypeJobSchedule) {
			azKeyVaultSecretVersions = append(azKeyVaultSecretVersions, azureKeyVaultSecretVersion)
			continue
		}
		jobName := pod.ObjectMeta.Labels[k8sJobNameLabel]
		job, ok := jobMap[jobName]
		if !ok {
			continue
		}
		azureKeyVaultSecretVersion.JobName = jobName
		jobCreated := job.GetCreationTimestamp()
		azureKeyVaultSecretVersion.JobCreated = radixutils.FormatTime(&jobCreated)
		if batchName, ok := pod.ObjectMeta.Labels[kube.RadixBatchNameLabel]; ok {
			if batch, ok := jobMap[batchName]; ok {
				azureKeyVaultSecretVersion.BatchName = batchName
				batchCreated := batch.GetCreationTimestamp()
				azureKeyVaultSecretVersion.BatchCreated = radixutils.FormatTime(&batchCreated)
			}
		}
		azKeyVaultSecretVersions = append(azKeyVaultSecretVersions, azureKeyVaultSecretVersion)
	}
	return azKeyVaultSecretVersions, nil
}

func (eh SecretHandler) getJobMap(appName, namespace, componentName string) (map[string]batchv1.Job, error) {
	jobMap := make(map[string]batchv1.Job)
	jobList, err := eh.userAccount.Client.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: labelselector.JobAndBatchJobsForComponent(appName, componentName)})
	if err != nil {
		return nil, err
	}
	for _, job := range jobList.Items {
		job := job
		jobMap[job.GetName()] = job
	}
	return jobMap, nil
}

func (eh SecretHandler) getCsiSecretStoreSecretMap(namespace string) (map[string]corev1.Secret, error) {
	secretList, err := eh.serviceAccount.Client.CoreV1().Secrets(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labels.Set{secretStoreCsiManagedLabel: "true"}.String(),
	})
	if err != nil {
		return nil, err
	}
	secretMap := make(map[string]corev1.Secret)
	for _, secretItem := range secretList.Items {
		secretItem := secretItem
		secretMap[secretItem.GetName()] = secretItem
	}
	return secretMap, nil
}
