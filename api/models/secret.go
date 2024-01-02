package models

import (
	"strings"

	secretModels "github.com/equinor/radix-api/api/secrets/models"
	"github.com/equinor/radix-api/api/secrets/suffix"
	"github.com/equinor/radix-api/api/utils/predicate"
	"github.com/equinor/radix-api/api/utils/secret"
	"github.com/equinor/radix-api/api/utils/tlsvalidator"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	operatordeployment "github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

const secretDefaultData = "xx"

// BuildSecrets builds a list of Secret models.
func BuildSecrets(secretList []corev1.Secret, secretProviderClassList []secretsstorev1.SecretProviderClass, rd *radixv1.RadixDeployment, tlsValidator tlsvalidator.TLSSecretValidator) []secretModels.Secret {
	var secrets []secretModels.Secret
	secrets = append(secrets, getSecretsForDeployment(secretList, rd)...)
	secrets = append(secrets, getSecretsForExternalDNS(secretList, rd, tlsValidator)...)
	secrets = append(secrets, getSecretsForVolumeMounts(secretList, rd)...)
	secrets = append(secrets, getSecretsForAuthentication(secretList, rd)...)
	secrets = append(secrets, getSecretsForSecretRefs(secretList, secretProviderClassList, rd)...)

	return secrets
}

func getSecretsForDeployment(secretList []corev1.Secret, rd *radixv1.RadixDeployment) []secretModels.Secret {
	getSecretsForComponent := func(component radixv1.RadixCommonDeployComponent) map[string]bool {
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

	componentSecretsMap := make(map[string]map[string]bool)
	for _, component := range rd.Spec.Components {
		secrets := getSecretsForComponent(&component)
		if len(secrets) > 0 {
			componentSecretsMap[component.Name] = secrets
		}
	}
	for _, job := range rd.Spec.Jobs {
		secrets := getSecretsForComponent(&job)
		if len(secrets) > 0 {
			componentSecretsMap[job.Name] = secrets
		}
	}

	var secretDTOsMap []secretModels.Secret
	for componentName, secretNamesMap := range componentSecretsMap {
		secretObjectName := operatorutils.GetComponentSecretName(componentName)
		secr, ok := slice.FindFirst(secretList, isSecretWithName(secretObjectName))
		if !ok {
			// Mark secrets as Pending (exist in config, does not exist in cluster) due to no secret object in the cluster
			for secretName := range secretNamesMap {
				secretDTOsMap = append(secretDTOsMap, secretModels.Secret{
					Name:        secretName,
					DisplayName: secretName,
					Type:        secretModels.SecretTypeGeneric,
					Component:   componentName,
					Status:      secretModels.Pending.String(),
				})
			}
			continue
		}

		clusterSecretEntriesMap := secr.Data
		for secretName := range secretNamesMap {
			status := secretModels.Consistent.String()
			if _, exists := clusterSecretEntriesMap[secretName]; !exists {
				status = secretModels.Pending.String()
			}

			secretDTOsMap = append(secretDTOsMap, secretModels.Secret{
				Name:        secretName,
				DisplayName: secretName,
				Type:        secretModels.SecretTypeGeneric,
				Component:   componentName,
				Status:      status,
			})
		}
	}

	return secretDTOsMap
}

func getSecretsForExternalDNS(secretList []corev1.Secret, rd *radixv1.RadixDeployment, tlsValidator tlsvalidator.TLSSecretValidator) []secretModels.Secret {
	if tlsValidator == nil {
		tlsValidator = tlsvalidator.DefaultValidator()
	}

	var secrets []secretModels.Secret
	for _, component := range rd.Spec.Components {
		for _, externalAlias := range component.GetExternalDNS() {
			if externalAlias.UseCertificateAutomation {
				continue
			}

			var certData, keyData []byte
			certStatus := secretModels.Consistent
			keyStatus := secretModels.Consistent

			if secretValue, ok := slice.FindFirst(secretList, isSecretWithName(externalAlias.FQDN)); ok {
				certData = secretValue.Data[corev1.TLSCertKey]
				if certValue := strings.TrimSpace(string(certData)); len(certValue) == 0 || strings.EqualFold(certValue, secretDefaultData) {
					certStatus = secretModels.Pending
					certData = nil
				}

				keyData = secretValue.Data[corev1.TLSPrivateKeyKey]
				if keyValue := strings.TrimSpace(string(keyData)); len(keyValue) == 0 || strings.EqualFold(keyValue, secretDefaultData) {
					keyStatus = secretModels.Pending
					keyData = nil
				}
			} else {
				certStatus = secretModels.Pending
				keyStatus = secretModels.Pending
			}

			var tlsCerts []secretModels.TLSCertificate
			var certStatusMessages []string
			if certStatus == secretModels.Consistent {
				tlsCerts = append(tlsCerts, secretModels.ParseTLSCertificatesFromPEM(certData)...)

				if certIsValid, messages := tlsValidator.ValidateTLSCertificate(certData, keyData, externalAlias.FQDN); !certIsValid {
					certStatus = secretModels.Invalid
					certStatusMessages = append(certStatusMessages, messages...)
				}
			}

			var keyStatusMessages []string
			if keyStatus == secretModels.Consistent {
				if keyIsValid, messages := tlsValidator.ValidateTLSKey(keyData); !keyIsValid {
					keyStatus = secretModels.Invalid
					keyStatusMessages = append(keyStatusMessages, messages...)
				}
			}

			secrets = append(secrets,
				secretModels.Secret{
					Name:            externalAlias.FQDN + suffix.ExternalDNSTLSCert,
					DisplayName:     "Certificate",
					Type:            secretModels.SecretTypeClientCert,
					Resource:        externalAlias.FQDN,
					ID:              secretModels.SecretIdCert,
					Component:       component.GetName(),
					Status:          certStatus.String(),
					StatusMessages:  certStatusMessages,
					TLSCertificates: tlsCerts,
				},
				secretModels.Secret{
					Name:           externalAlias.FQDN + suffix.ExternalDNSTLSKey,
					DisplayName:    "Key",
					Type:           secretModels.SecretTypeClientCert,
					Resource:       externalAlias.FQDN,
					Component:      component.GetName(),
					ID:             secretModels.SecretIdKey,
					Status:         keyStatus.String(),
					StatusMessages: keyStatusMessages,
				},
			)
		}
	}

	return secrets
}

func getSecretsForVolumeMounts(secretList []corev1.Secret, rd *radixv1.RadixDeployment) []secretModels.Secret {
	var secrets []secretModels.Secret
	for _, component := range rd.Spec.Components {
		secrets = append(secrets, getCredentialSecretsForBlobVolumes(secretList, &component)...)
	}
	for _, job := range rd.Spec.Jobs {
		secrets = append(secrets, getCredentialSecretsForBlobVolumes(secretList, &job)...)
	}

	return secrets
}

func getCredentialSecretsForBlobVolumes(secretList []corev1.Secret, component radixv1.RadixCommonDeployComponent) []secretModels.Secret {
	var secrets []secretModels.Secret
	for _, volumeMount := range component.GetVolumeMounts() {
		volumeMountType := operatordeployment.GetCsiAzureVolumeMountType(&volumeMount)
		switch volumeMountType {
		case radixv1.MountTypeBlob:
			accountKeySecret, accountNameSecret := getBlobFuseSecrets(secretList, component, volumeMount)
			secrets = append(secrets, accountKeySecret, accountNameSecret)
		case radixv1.MountTypeBlobFuse2FuseCsiAzure, radixv1.MountTypeBlobFuse2Fuse2CsiAzure, radixv1.MountTypeBlobFuse2NfsCsiAzure, radixv1.MountTypeAzureFileCsiAzure:
			accountKeySecret, accountNameSecret := getCsiAzureSecrets(secretList, component, volumeMount)
			secrets = append(secrets, accountKeySecret, accountNameSecret)
		}
	}

	return secrets
}

func getBlobFuseSecrets(secretList []corev1.Secret, component radixv1.RadixCommonDeployComponent, volumeMount radixv1.RadixVolumeMount) (secretModels.Secret, secretModels.Secret) {
	return getAzureVolumeMountSecrets(secretList, component,
		defaults.GetBlobFuseCredsSecretName(component.GetName(), volumeMount.Name),
		volumeMount.Name,
		defaults.BlobFuseCredsAccountNamePart,
		defaults.BlobFuseCredsAccountKeyPart,
		defaults.BlobFuseCredsAccountNamePartSuffix,
		defaults.BlobFuseCredsAccountKeyPartSuffix,
		secretModels.SecretTypeAzureBlobFuseVolume,
	)
}

func getCsiAzureSecrets(secretList []corev1.Secret, component radixv1.RadixCommonDeployComponent, volumeMount radixv1.RadixVolumeMount) (secretModels.Secret, secretModels.Secret) {
	return getAzureVolumeMountSecrets(secretList, component,
		defaults.GetCsiAzureVolumeMountCredsSecretName(component.GetName(), volumeMount.Name),
		volumeMount.Name,
		defaults.CsiAzureCredsAccountNamePart,
		defaults.CsiAzureCredsAccountKeyPart,
		defaults.CsiAzureCredsAccountNamePartSuffix,
		defaults.CsiAzureCredsAccountKeyPartSuffix,
		secretModels.SecretTypeCsiAzureBlobVolume,
	)
}

func getAzureVolumeMountSecrets(secretList []corev1.Secret, component radixv1.RadixCommonDeployComponent, secretName, volumeMountName, accountNamePart, accountKeyPart, accountNamePartSuffix, accountKeyPartSuffix string, secretType secretModels.SecretType) (secretModels.Secret, secretModels.Secret) {
	accountkeyStatus := secretModels.Consistent.String()
	accountnameStatus := secretModels.Consistent.String()

	if secretValue, ok := slice.FindFirst(secretList, isSecretWithName(secretName)); ok {
		accountkeyValue := strings.TrimSpace(string(secretValue.Data[accountKeyPart]))
		if strings.EqualFold(accountkeyValue, secretDefaultData) {
			accountkeyStatus = secretModels.Pending.String()
		}

		accountnameValue := strings.TrimSpace(string(secretValue.Data[accountNamePart]))
		if strings.EqualFold(accountnameValue, secretDefaultData) {
			accountnameStatus = secretModels.Pending.String()
		}
	} else {
		accountkeyStatus = secretModels.Pending.String()
		accountnameStatus = secretModels.Pending.String()
	}

	// "accountkey"
	accountKeySecretDTO := secretModels.Secret{
		Name:        secretName + accountKeyPartSuffix,
		DisplayName: "Account Key",
		Type:        secretType,
		Resource:    volumeMountName,
		ID:          secretModels.SecretIdAccountKey,
		Component:   component.GetName(),
		Status:      accountkeyStatus,
	}
	// "accountname"
	accountNameSecretDTO := secretModels.Secret{
		Name:        secretName + accountNamePartSuffix,
		DisplayName: "Account Name",
		Type:        secretType,
		Resource:    volumeMountName,
		ID:          secretModels.SecretIdAccountName,
		Component:   component.GetName(),
		Status:      accountnameStatus,
	}

	return accountKeySecretDTO, accountNameSecretDTO
}

func getSecretsForAuthentication(secretList []corev1.Secret, activeDeployment *radixv1.RadixDeployment) []secretModels.Secret {
	var secrets []secretModels.Secret
	for _, component := range activeDeployment.Spec.Components {
		authSecrets := getSecretsForComponentAuthentication(secretList, &component)
		secrets = append(secrets, authSecrets...)
	}

	return secrets
}

func getSecretsForComponentAuthentication(secretList []corev1.Secret, component radixv1.RadixCommonDeployComponent) []secretModels.Secret {
	var secrets []secretModels.Secret
	secrets = append(secrets, getSecretsForComponentAuthenticationClientCertificate(secretList, component)...)
	secrets = append(secrets, getSecretsForComponentAuthenticationOAuth2(secretList, component)...)

	return secrets
}

func getSecretsForComponentAuthenticationClientCertificate(secretList []corev1.Secret, component radixv1.RadixCommonDeployComponent) []secretModels.Secret {
	var secrets []secretModels.Secret
	if auth := component.GetAuthentication(); auth != nil && component.IsPublic() && ingress.IsSecretRequiredForClientCertificate(auth.ClientCertificate) {
		secretName := operatorutils.GetComponentClientCertificateSecretName(component.GetName())
		secretStatus := secretModels.Consistent.String()

		if secr, ok := slice.FindFirst(secretList, isSecretWithName(secretName)); ok {
			secretValue := strings.TrimSpace(string(secr.Data["ca.crt"]))
			if strings.EqualFold(secretValue, secretDefaultData) {
				secretStatus = secretModels.Pending.String()
			}
		} else {
			secretStatus = secretModels.Pending.String()
		}

		secrets = append(secrets, secretModels.Secret{
			Name:        secretName,
			DisplayName: "",
			Type:        secretModels.SecretTypeClientCertificateAuth,
			Component:   component.GetName(),
			Status:      secretStatus,
		})
	}

	return secrets
}

func getSecretsForComponentAuthenticationOAuth2(secretList []corev1.Secret, component radixv1.RadixCommonDeployComponent) []secretModels.Secret {
	var secrets []secretModels.Secret
	if auth := component.GetAuthentication(); component.IsPublic() && auth != nil && auth.OAuth2 != nil {
		oauth2, err := defaults.NewOAuth2Config(defaults.WithOAuth2Defaults()).MergeWith(auth.OAuth2)
		if err != nil {
			panic(err)
		}

		clientSecretStatus := secretModels.Consistent.String()
		cookieSecretStatus := secretModels.Consistent.String()
		redisPasswordStatus := secretModels.Consistent.String()

		secretName := operatorutils.GetAuxiliaryComponentSecretName(component.GetName(), defaults.OAuthProxyAuxiliaryComponentSuffix)
		if secr, ok := slice.FindFirst(secretList, isSecretWithName(secretName)); ok {
			if secretValue, found := secr.Data[defaults.OAuthClientSecretKeyName]; !found || len(strings.TrimSpace(string(secretValue))) == 0 {
				clientSecretStatus = secretModels.Pending.String()
			}
			if secretValue, found := secr.Data[defaults.OAuthCookieSecretKeyName]; !found || len(strings.TrimSpace(string(secretValue))) == 0 {
				cookieSecretStatus = secretModels.Pending.String()
			}
			if secretValue, found := secr.Data[defaults.OAuthRedisPasswordKeyName]; !found || len(strings.TrimSpace(string(secretValue))) == 0 {
				redisPasswordStatus = secretModels.Pending.String()
			}
		} else {
			clientSecretStatus = secretModels.Pending.String()
			cookieSecretStatus = secretModels.Pending.String()
			redisPasswordStatus = secretModels.Pending.String()
		}

		secrets = append(secrets, secretModels.Secret{
			Name:        component.GetName() + suffix.OAuth2ClientSecret,
			DisplayName: "Client Secret",
			Type:        secretModels.SecretTypeOAuth2Proxy,
			Component:   component.GetName(),
			Status:      clientSecretStatus,
		})
		secrets = append(secrets, secretModels.Secret{
			Name:        component.GetName() + suffix.OAuth2CookieSecret,
			DisplayName: "Cookie Secret",
			Type:        secretModels.SecretTypeOAuth2Proxy,
			Component:   component.GetName(),
			Status:      cookieSecretStatus,
		})

		if oauth2.SessionStoreType == radixv1.SessionStoreRedis {
			secrets = append(secrets, secretModels.Secret{
				Name:        component.GetName() + suffix.OAuth2RedisPassword,
				DisplayName: "Redis Password",
				Type:        secretModels.SecretTypeOAuth2Proxy,
				Component:   component.GetName(),
				Status:      redisPasswordStatus,
			})
		}
	}

	return secrets
}

func getSecretsForSecretRefs(secretList []corev1.Secret, secretProviderClassList []secretsstorev1.SecretProviderClass, rd *radixv1.RadixDeployment) []secretModels.Secret {
	secretProviderClassMapForDeployment := slice.Reduce(
		slice.FindAll(secretProviderClassList, predicate.IsSecretProviderClassForDeployment(rd.Name)),
		map[string]secretsstorev1.SecretProviderClass{},
		func(acc map[string]secretsstorev1.SecretProviderClass, spc secretsstorev1.SecretProviderClass) map[string]secretsstorev1.SecretProviderClass {
			acc[spc.GetName()] = spc
			return acc
		},
	)

	csiSecretStoreSecretMap := slice.Reduce(
		slice.FindAll(secretList, predicate.IsSecretForSecretStoreProviderClass),
		map[string]corev1.Secret{},
		func(acc map[string]corev1.Secret, secret corev1.Secret) map[string]corev1.Secret {
			acc[secret.GetName()] = secret
			return acc
		},
	)

	var secrets []secretModels.Secret
	for _, component := range rd.Spec.Components {
		secretRefs := component.GetSecretRefs()
		componentSecrets := getComponentSecretRefsSecrets(secretList, component.GetName(), &secretRefs, secretProviderClassMapForDeployment, csiSecretStoreSecretMap)
		secrets = append(secrets, componentSecrets...)
	}
	for _, jobComponent := range rd.Spec.Jobs {
		secretRefs := jobComponent.GetSecretRefs()
		jobComponentSecrets := getComponentSecretRefsSecrets(secretList, jobComponent.GetName(), &secretRefs, secretProviderClassMapForDeployment, csiSecretStoreSecretMap)
		secrets = append(secrets, jobComponentSecrets...)
	}

	return secrets
}

func getComponentSecretRefsSecrets(secretList []corev1.Secret, componentName string, secretRefs *radixv1.RadixSecretRefs,
	secretProviderClassMap map[string]secretsstorev1.SecretProviderClass, csiSecretStoreSecretMap map[string]corev1.Secret) []secretModels.Secret {
	var secrets []secretModels.Secret
	for _, azureKeyVault := range secretRefs.AzureKeyVaults {
		if azureKeyVault.UseAzureIdentity == nil || !*azureKeyVault.UseAzureIdentity {
			credSecrets := getCredentialSecretsForSecretRefsAzureKeyVault(secretList, componentName, azureKeyVault.Name)
			secrets = append(secrets, credSecrets...)
		}

		secretStatus := getAzureKeyVaultSecretStatus(componentName, azureKeyVault.Name, secretProviderClassMap, csiSecretStoreSecretMap)
		for _, item := range azureKeyVault.Items {
			secrets = append(secrets, secretModels.Secret{
				Name:        secret.GetSecretNameForAzureKeyVaultItem(componentName, azureKeyVault.Name, &item),
				DisplayName: secret.GetSecretDisplayNameForAzureKeyVaultItem(&item),
				Type:        secretModels.SecretTypeCsiAzureKeyVaultItem,
				Resource:    azureKeyVault.Name,
				ID:          secret.GetSecretIdForAzureKeyVaultItem(&item),
				Component:   componentName,
				Status:      secretStatus,
			})
		}
	}

	return secrets
}

func getAzureKeyVaultSecretStatus(componentName, azureKeyVaultName string, secretProviderClassMap map[string]secretsstorev1.SecretProviderClass, csiSecretStoreSecretMap map[string]corev1.Secret) string {
	secretProviderClass := getComponentSecretProviderClassMapForAzureKeyVault(componentName, secretProviderClassMap, azureKeyVaultName)
	if secretProviderClass == nil {
		return secretModels.NotAvailable.String()
	}

	secretStatus := secretModels.Consistent.String()
	for _, secretObject := range secretProviderClass.Spec.SecretObjects {
		if _, ok := csiSecretStoreSecretMap[secretObject.SecretName]; !ok {
			secretStatus = secretModels.NotAvailable.String() // Secrets does not exist for the secretProviderClass secret object
			break
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

func getCredentialSecretsForSecretRefsAzureKeyVault(secretList []corev1.Secret, componentName, azureKeyVaultName string) []secretModels.Secret {
	var secrets []secretModels.Secret
	secretName := defaults.GetCsiAzureKeyVaultCredsSecretName(componentName, azureKeyVaultName)
	clientIdStatus := secretModels.Consistent.String()
	clientSecretStatus := secretModels.Consistent.String()

	if secretValue, ok := slice.FindFirst(secretList, isSecretWithName(secretName)); ok {
		clientIdValue := strings.TrimSpace(string(secretValue.Data[defaults.CsiAzureKeyVaultCredsClientIdPart]))
		if strings.EqualFold(clientIdValue, secretDefaultData) {
			clientIdStatus = secretModels.Pending.String()
		}

		clientSecretValue := strings.TrimSpace(string(secretValue.Data[defaults.CsiAzureKeyVaultCredsClientSecretPart]))
		if strings.EqualFold(clientSecretValue, secretDefaultData) {
			clientSecretStatus = secretModels.Pending.String()
		}
	} else {
		clientIdStatus = secretModels.Pending.String()
		clientSecretStatus = secretModels.Pending.String()
	}

	secrets = append(secrets, secretModels.Secret{
		Name:        secretName + defaults.CsiAzureKeyVaultCredsClientIdSuffix,
		DisplayName: "Client ID",
		Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
		Resource:    azureKeyVaultName,
		ID:          secretModels.SecretIdClientId,
		Component:   componentName,
		Status:      clientIdStatus,
	})
	secrets = append(secrets, secretModels.Secret{
		Name:        secretName + defaults.CsiAzureKeyVaultCredsClientSecretSuffix,
		DisplayName: "Client Secret",
		Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
		Resource:    azureKeyVaultName,
		ID:          secretModels.SecretIdClientSecret,
		Component:   componentName,
		Status:      clientSecretStatus,
	})

	return secrets
}

func isSecretWithName(name string) func(secret corev1.Secret) bool {
	return func(secret corev1.Secret) bool {
		return secret.Name == name
	}
}
