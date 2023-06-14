package models

import (
	"strings"

	secretModels "github.com/equinor/radix-api/api/secrets/models"
	"github.com/equinor/radix-api/api/secrets/suffix"
	"github.com/equinor/radix-api/api/utils/tlsvalidator"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	operatordeployment "github.com/equinor/radix-operator/pkg/apis/deployment"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
)

const secretDefaultData = "xx"

func BuildSecrets(secretList []corev1.Secret, rd *radixv1.RadixDeployment, tlsValidator tlsvalidator.Interface) []secretModels.Secret {
	var secrets []secretModels.Secret
	secrets = append(secrets, getSecretsForDeployment(secretList, rd)...)
	secrets = append(secrets, getSecretsForTLSCertificates(secretList, rd, tlsValidator)...)
	secrets = append(secrets, getSecretsForVolumeMounts(secretList, rd)...)
	secrets = append(secrets, getSecretsForAuthentication(secretList, rd)...)
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
		if len(secrets) <= 0 {
			continue
		}
		componentSecretsMap[component.Name] = secrets
	}
	for _, job := range rd.Spec.Jobs {
		secrets := getSecretsForComponent(&job)
		if len(secrets) <= 0 {
			continue
		}
		componentSecretsMap[job.Name] = secrets
	}

	var secretDTOsMap []secretModels.Secret
	for componentName, secretNamesMap := range componentSecretsMap {
		secretObjectName := operatorutils.GetComponentSecretName(componentName)
		i := slice.FindIndex(secretList, func(secret corev1.Secret) bool { return secret.Name == secretObjectName })
		// Mark secrets as Pending (exist in config, does not exist in cluster) due to no secret object in the cluster
		if i == -1 {
			for secretName := range secretNamesMap {
				secretDTO := secretModels.Secret{Name: secretName, DisplayName: secretName, Component: componentName, Status: secretModels.Pending.String(), Type: secretModels.SecretTypeGeneric}
				secretDTOsMap = append(secretDTOsMap, secretDTO)
			}
			continue
		}

		clusterSecretEntriesMap := secretList[i].Data
		for secretName := range secretNamesMap {
			status := secretModels.Consistent.String()
			if _, exists := clusterSecretEntriesMap[secretName]; !exists {
				status = secretModels.Pending.String()
			}
			secretDTO := secretModels.Secret{Name: secretName, DisplayName: secretName, Component: componentName, Status: status, Type: secretModels.SecretTypeGeneric}
			secretDTOsMap = append(secretDTOsMap, secretDTO)
		}
	}

	return secretDTOsMap
}

func getSecretsForTLSCertificates(secretList []corev1.Secret, rd *radixv1.RadixDeployment, tlsValidator tlsvalidator.Interface) []secretModels.Secret {
	var secrets []secretModels.Secret
	if tlsValidator == nil {
		tlsValidator = tlsvalidator.DefaultValidator()
	}

	for _, component := range rd.Spec.Components {
		for _, externalAlias := range component.DNSExternalAlias {
			var certData, keyData []byte
			certStatus := secretModels.Consistent
			keyStatus := secretModels.Consistent

			var secretValue *corev1.Secret
			if i := slice.FindIndex(secretList, func(secret corev1.Secret) bool { return secret.Name == externalAlias }); i >= 0 {
				secretValue = &secretList[i]
			}
			if secretValue == nil {
				certStatus = secretModels.Pending
				keyStatus = secretModels.Pending
			} else {
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
			}

			var tlsCerts []secretModels.TLSCertificate
			var certStatusMessages []string
			if certStatus == secretModels.Consistent {
				tlsCerts = append(tlsCerts, secretModels.ParseTLSCertificatesFromPEM(certData)...)

				if certIsValid, messages := tlsValidator.ValidateTLSCertificate(certData, keyData, externalAlias); !certIsValid {
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
					Name:            externalAlias + suffix.ExternalDNSTLSCert,
					DisplayName:     "Certificate",
					Resource:        externalAlias,
					Type:            secretModels.SecretTypeClientCert,
					Component:       component.GetName(),
					Status:          certStatus.String(),
					ID:              secretModels.SecretIdCert,
					StatusMessages:  certStatusMessages,
					TLSCertificates: tlsCerts,
				},
				secretModels.Secret{
					Name:           externalAlias + suffix.ExternalDNSTLSKey,
					DisplayName:    "Key",
					Resource:       externalAlias,
					Type:           secretModels.SecretTypeClientCert,
					Component:      component.GetName(),
					Status:         keyStatus.String(),
					StatusMessages: keyStatusMessages,
					ID:             secretModels.SecretIdKey,
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
		switch volumeMount.Type {
		case radixv1.MountTypeBlob:
			accountKeySecret, accountNameSecret := getBlobFuseSecrets(secretList, component, volumeMount)
			secrets = append(secrets, accountKeySecret)
			secrets = append(secrets, accountNameSecret)
		case radixv1.MountTypeBlobCsiAzure, radixv1.MountTypeFileCsiAzure:
			accountKeySecret, accountNameSecret := getCsiAzureSecrets(secretList, component, volumeMount)
			secrets = append(secrets, accountKeySecret)
			secrets = append(secrets, accountNameSecret)
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
		secretModels.SecretTypeAzureBlobFuseVolume)
}

func getCsiAzureSecrets(secretList []corev1.Secret, component radixv1.RadixCommonDeployComponent, volumeMount radixv1.RadixVolumeMount) (secretModels.Secret, secretModels.Secret) {
	return getAzureVolumeMountSecrets(secretList, component,
		defaults.GetCsiAzureVolumeMountCredsSecretName(component.GetName(), volumeMount.Name),
		volumeMount.Name,
		defaults.CsiAzureCredsAccountNamePart,
		defaults.CsiAzureCredsAccountKeyPart,
		defaults.CsiAzureCredsAccountNamePartSuffix,
		defaults.CsiAzureCredsAccountKeyPartSuffix,
		secretModels.SecretTypeCsiAzureBlobVolume)
}

func getAzureVolumeMountSecrets(secretList []corev1.Secret, component radixv1.RadixCommonDeployComponent, secretName, volumeMountName, accountNamePart, accountKeyPart, accountNamePartSuffix, accountKeyPartSuffix string, secretType secretModels.SecretType) (secretModels.Secret, secretModels.Secret) {
	accountkeyStatus := secretModels.Consistent.String()
	accountnameStatus := secretModels.Consistent.String()

	i := slice.FindIndex(secretList, func(secret corev1.Secret) bool { return secret.Name == secretName })
	if i == -1 {
		accountkeyStatus = secretModels.Pending.String()
		accountnameStatus = secretModels.Pending.String()
	} else {
		secretValue := secretList[i]
		accountkeyValue := strings.TrimSpace(string(secretValue.Data[accountKeyPart]))
		if strings.EqualFold(accountkeyValue, secretDefaultData) {
			accountkeyStatus = secretModels.Pending.String()
		}
		accountnameValue := strings.TrimSpace(string(secretValue.Data[accountNamePart]))
		if strings.EqualFold(accountnameValue, secretDefaultData) {
			accountnameStatus = secretModels.Pending.String()
		}
	}
	// "accountkey"
	accountKeySecretDTO := secretModels.Secret{
		Name:        secretName + accountKeyPartSuffix,
		DisplayName: "Account Key",
		Resource:    volumeMountName,
		Component:   component.GetName(),
		Status:      accountkeyStatus,
		Type:        secretType,
		ID:          secretModels.SecretIdAccountKey}
	// "accountname"
	accountNameSecretDTO := secretModels.Secret{
		Name:        secretName + accountNamePartSuffix,
		DisplayName: "Account Name",
		Resource:    volumeMountName,
		Component:   component.GetName(),
		Status:      accountnameStatus,
		Type:        secretType,
		ID:          secretModels.SecretIdAccountName}
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
	if auth := component.GetAuthentication(); auth != nil && component.IsPublic() && operatordeployment.IsSecretRequiredForClientCertificate(auth.ClientCertificate) {
		secretName := operatorutils.GetComponentClientCertificateSecretName(component.GetName())
		secretStatus := secretModels.Consistent.String()

		i := slice.FindIndex(secretList, func(secret corev1.Secret) bool { return secret.Name == secretName })
		if i == -1 {
			secretStatus = secretModels.Pending.String()
		} else {
			secretValue := strings.TrimSpace(string(secretList[i].Data["ca.crt"]))
			if strings.EqualFold(secretValue, secretDefaultData) {
				secretStatus = secretModels.Pending.String()
			}
		}

		secrets = append(secrets, secretModels.Secret{Name: secretName,
			DisplayName: "",
			Type:        secretModels.SecretTypeClientCertificateAuth, Component: component.GetName(), Status: secretStatus})
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
		i := slice.FindIndex(secretList, func(secret corev1.Secret) bool { return secret.Name == secretName })
		if i == -1 {
			clientSecretStatus = secretModels.Pending.String()
			cookieSecretStatus = secretModels.Pending.String()
			redisPasswordStatus = secretModels.Pending.String()
		} else {
			secret := secretList[i]
			if secretValue, found := secret.Data[defaults.OAuthClientSecretKeyName]; !found || len(strings.TrimSpace(string(secretValue))) == 0 {
				clientSecretStatus = secretModels.Pending.String()
			}
			if secretValue, found := secret.Data[defaults.OAuthCookieSecretKeyName]; !found || len(strings.TrimSpace(string(secretValue))) == 0 {
				cookieSecretStatus = secretModels.Pending.String()
			}
			if secretValue, found := secret.Data[defaults.OAuthRedisPasswordKeyName]; !found || len(strings.TrimSpace(string(secretValue))) == 0 {
				redisPasswordStatus = secretModels.Pending.String()
			}
		}

		secrets = append(secrets, secretModels.Secret{Name: component.GetName() + suffix.OAuth2ClientSecret,
			DisplayName: "Client Secret",
			Type:        secretModels.SecretTypeOAuth2Proxy, Component: component.GetName(),
			Status: clientSecretStatus})
		secrets = append(secrets, secretModels.Secret{Name: component.GetName() + suffix.OAuth2CookieSecret,
			DisplayName: "Cookie Secret",
			Type:        secretModels.SecretTypeOAuth2Proxy, Component: component.GetName(), Status: cookieSecretStatus})

		if oauth2.SessionStoreType == radixv1.SessionStoreRedis {
			secrets = append(secrets, secretModels.Secret{Name: component.GetName() + suffix.OAuth2RedisPassword,
				DisplayName: "Redis Password",
				Type:        secretModels.SecretTypeOAuth2Proxy, Component: component.GetName(), Status: redisPasswordStatus})
		}
	}

	return secrets
}

// func getSecretsForSecretRefs(appName string, radixDeployment *radixv1.RadixDeployment, envNamespace string) []secretModels.Secret {
// 	secretProviderClassMapForDeployment, err := eh.getAzureKeyVaultSecretProviderClassMapForAppDeployment(appName, envNamespace, radixDeployment.GetName())
// 	if err != nil {
// 		return nil, err
// 	}
// 	csiSecretStoreSecretMap, err := eh.getCsiSecretStoreSecretMap(envNamespace)
// 	if err != nil {
// 		return nil, err
// 	}
// 	var secrets []models.Secret
// 	for _, component := range radixDeployment.Spec.Components {
// 		secretRefs := component.GetSecretRefs()
// 		componentSecrets, err := eh.getComponentSecretRefsSecrets(envNamespace, component.GetName(), &secretRefs, secretProviderClassMapForDeployment, csiSecretStoreSecretMap)
// 		if err != nil {
// 			return nil, err
// 		}
// 		secrets = append(secrets, componentSecrets...)
// 	}
// 	for _, jobComponent := range radixDeployment.Spec.Jobs {
// 		secretRefs := jobComponent.GetSecretRefs()
// 		jobComponentSecrets, err := eh.getComponentSecretRefsSecrets(envNamespace, jobComponent.GetName(), &secretRefs, secretProviderClassMapForDeployment, csiSecretStoreSecretMap)
// 		if err != nil {
// 			return nil, err
// 		}
// 		secrets = append(secrets, jobComponentSecrets...)
// 	}
// 	return secrets, nil
// }
