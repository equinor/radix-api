package secrets

import (
	"context"
	"fmt"
	"strings"
	"testing"

	deployMock "github.com/equinor/radix-api/api/deployments/mock"
	secretModels "github.com/equinor/radix-api/api/secrets/models"
	"github.com/equinor/radix-api/api/secrets/suffix"
	"github.com/equinor/radix-api/api/utils/secret"
	"github.com/equinor/radix-api/models"
	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
	secretProviderClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	tenantId = "123456789"
)

type secretHandlerTestSuite struct {
	suite.Suite
}

func TestRunSecretHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(secretHandlerTestSuite))
}

type secretDescription struct {
	secretName string
	secretData map[string][]byte
	labels     map[string]string
}

type getSecretScenario struct {
	name                   string
	components             []v1.RadixDeployComponent
	jobs                   []v1.RadixDeployJobComponent
	init                   *func(*SecretHandler) //scenario optional custom init function
	existingSecrets        []secretDescription
	expectedSecrets        []secretModels.Secret
	expectedSecretVersions map[string]map[string]map[string]map[string]map[string]bool //map[componentName]map[azureKeyVaultName]map[secretId]map[version]map[replicaName]bool
}

type changeSecretScenario struct {
	name                        string
	components                  []v1.RadixDeployComponent
	jobs                        []v1.RadixDeployJobComponent
	secretName                  string
	secretDataKey               string
	secretValue                 string
	secretExists                bool
	changingSecretComponentName string
	changingSecretName          string
	expectedError               bool
	changingSecretParams        secretModels.SecretParameters
}

type secretProviderClassAndSecret struct {
	secretName string
	className  string
}

func (s *secretHandlerTestSuite) TestSecretHandler_GetSecrets() {
	ctrl := gomock.NewController(s.T())
	defer ctrl.Finish()
	componentName1 := "component1"
	jobName1 := "job1"
	scenarios := []getSecretScenario{
		{
			name: "regular secrets with no existing secrets",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
				Secrets: []string{
					"SECRET_C1",
				},
			}},
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
				Secrets: []string{
					"SECRET_J1",
				},
			}},
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "SECRET_C1",
					DisplayName: "SECRET_C1",
					Type:        secretModels.SecretTypeGeneric,
					Resource:    "",
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
				},
				{
					Name:        "SECRET_J1",
					DisplayName: "SECRET_J1",
					Type:        secretModels.SecretTypeGeneric,
					Resource:    "",
					Component:   jobName1,
					Status:      secretModels.Pending.String(),
				},
			},
		},
		{
			name: "regular secrets with existing secrets",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
				Secrets: []string{
					"SECRET_C1",
				},
			}},
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
				Secrets: []string{
					"SECRET_J1",
				},
			}},
			existingSecrets: []secretDescription{
				{
					secretName: operatorUtils.GetComponentSecretName(componentName1),
					secretData: map[string][]byte{"SECRET_C1": []byte("current-value1")},
				},
				{
					secretName: operatorUtils.GetComponentSecretName(jobName1),
					secretData: map[string][]byte{"SECRET_J1": []byte("current-value2")},
				},
			},
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "SECRET_C1",
					DisplayName: "SECRET_C1",
					Type:        secretModels.SecretTypeGeneric,
					Resource:    "",
					Component:   componentName1,
					Status:      secretModels.Consistent.String(),
				},
				{
					Name:        "SECRET_J1",
					DisplayName: "SECRET_J1",
					Type:        secretModels.SecretTypeGeneric,
					Resource:    "",
					Component:   jobName1,
					Status:      secretModels.Consistent.String(),
				},
			},
		},
		{
			name:       "External alias secrets with no secrets, must build secrets from deploy component",
			components: []v1.RadixDeployComponent{{Name: componentName1, DNSExternalAlias: []string{"deployed-alias-1", "deployed-alias-2"}}},
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "deployed-alias-1-key",
					DisplayName: "Key",
					Type:        secretModels.SecretTypeClientCert,
					Resource:    "deployed-alias-1",
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
					ID:          secretModels.SecretIdKey,
				},
				{
					Name:        "deployed-alias-1-cert",
					DisplayName: "Certificate",
					Type:        secretModels.SecretTypeClientCert,
					Resource:    "deployed-alias-1",
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
					ID:          secretModels.SecretIdCert,
				},
				{
					Name:        "deployed-alias-2-key",
					DisplayName: "Key",
					Type:        secretModels.SecretTypeClientCert,
					Resource:    "deployed-alias-2",
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
					ID:          secretModels.SecretIdKey,
				},
				{
					Name:        "deployed-alias-2-cert",
					DisplayName: "Certificate",
					Type:        secretModels.SecretTypeClientCert,
					Resource:    "deployed-alias-2",
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
					ID:          secretModels.SecretIdCert,
				},
			},
		},
		{
			name:       "External alias secrets with existing secrets, must build secrets from deploy component",
			components: []v1.RadixDeployComponent{{Name: componentName1, DNSExternalAlias: []string{"deployed-alias"}}},
			existingSecrets: []secretDescription{
				{
					secretName: "deployed-alias",
					secretData: map[string][]byte{
						"tls.cer": []byte("current tls cert"),
						"tls.key": []byte("current tls key"),
					},
				},
			},
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "deployed-alias-key",
					DisplayName: "Key",
					Type:        secretModels.SecretTypeClientCert,
					Resource:    "deployed-alias",
					Component:   componentName1,
					Status:      secretModels.Consistent.String(),
					ID:          secretModels.SecretIdKey,
				},
				{
					Name:        "deployed-alias-cert",
					DisplayName: "Certificate",
					Type:        secretModels.SecretTypeClientCert,
					Resource:    "deployed-alias",
					Component:   componentName1,
					Status:      secretModels.Consistent.String(),
					ID:          secretModels.SecretIdCert,
				},
			},
		},
		{
			name: "Azure Blob volumes credential secrets with no secrets",
			components: []v1.RadixDeployComponent{
				{
					Name: componentName1,
					VolumeMounts: []v1.RadixVolumeMount{
						{
							Type:    v1.MountTypeBlobCsiAzure,
							Name:    "volume1",
							Storage: "container1",
						},
					},
				},
			},
			jobs: []v1.RadixDeployJobComponent{
				{
					Name: jobName1,
					VolumeMounts: []v1.RadixVolumeMount{
						{
							Type:    v1.MountTypeBlobCsiAzure,
							Name:    "volume2",
							Storage: "container2",
						},
					},
				},
			},
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-volume1-csiazurecreds-accountkey",
					DisplayName: "Account Key",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume1",
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
					ID:          secretModels.SecretIdAccountKey,
				},
				{
					Name:        "component1-volume1-csiazurecreds-accountname",
					DisplayName: "Account Name",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume1",
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
					ID:          secretModels.SecretIdAccountName,
				},
				{
					Name:        "job1-volume2-csiazurecreds-accountkey",
					DisplayName: "Account Key",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume2",
					Component:   jobName1,
					Status:      secretModels.Pending.String(),
					ID:          secretModels.SecretIdAccountKey,
				},
				{
					Name:        "job1-volume2-csiazurecreds-accountname",
					DisplayName: "Account Name",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume2",
					Component:   jobName1,
					Status:      secretModels.Pending.String(),
					ID:          secretModels.SecretIdAccountName,
				},
			},
		},
		{
			name: "Azure Blob volumes credential secrets with existing secrets",
			components: []v1.RadixDeployComponent{
				{
					Name: componentName1,
					VolumeMounts: []v1.RadixVolumeMount{
						{
							Type:    v1.MountTypeBlobCsiAzure,
							Name:    "volume1",
							Storage: "container1",
						},
					},
				},
			},
			jobs: []v1.RadixDeployJobComponent{
				{
					Name: jobName1,
					VolumeMounts: []v1.RadixVolumeMount{
						{
							Type:    v1.MountTypeBlobCsiAzure,
							Name:    "volume2",
							Storage: "container2",
						},
					},
				},
			},
			existingSecrets: []secretDescription{
				{
					secretName: "component1-volume1-csiazurecreds",
					secretData: map[string][]byte{
						"accountname": []byte("current account name1"),
						"accountkey":  []byte("current account key1"),
					},
				},
				{
					secretName: "job1-volume2-csiazurecreds",
					secretData: map[string][]byte{
						"accountname": []byte("current account name2"),
						"accountkey":  []byte("current account key2"),
					},
				},
			},
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-volume1-csiazurecreds-accountkey",
					DisplayName: "Account Key",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume1",
					Component:   componentName1,
					Status:      secretModels.Consistent.String(),
					ID:          secretModels.SecretIdAccountKey,
				},
				{
					Name:        "component1-volume1-csiazurecreds-accountname",
					DisplayName: "Account Name",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume1",
					Component:   componentName1,
					Status:      secretModels.Consistent.String(),
					ID:          secretModels.SecretIdAccountName,
				},
				{
					Name:        "job1-volume2-csiazurecreds-accountkey",
					DisplayName: "Account Key",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume2",
					Component:   jobName1,
					Status:      secretModels.Consistent.String(),
					ID:          secretModels.SecretIdAccountKey,
				},
				{
					Name:        "job1-volume2-csiazurecreds-accountname",
					DisplayName: "Account Name",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume2",
					Component:   jobName1,
					Status:      secretModels.Consistent.String(),
					ID:          secretModels.SecretIdAccountName,
				},
			},
		},
		{
			name: "No Azure Key vault credential secrets when there is no Azure key vault SecretRefs",
			components: []v1.RadixDeployComponent{
				{
					Name:       componentName1,
					SecretRefs: v1.RadixSecretRefs{AzureKeyVaults: nil},
				},
			},
			jobs: []v1.RadixDeployJobComponent{
				{
					Name:       jobName1,
					SecretRefs: v1.RadixSecretRefs{AzureKeyVaults: nil},
				},
			},
			expectedSecrets: nil,
		},
		{
			name: "Azure Key vault credential secrets when there are secret items with no secrets",
			components: []v1.RadixDeployComponent{
				{
					Name: componentName1,
					SecretRefs: v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{
						{
							Name: "keyVault1",
							Items: []v1.RadixAzureKeyVaultItem{
								{
									Name:   "secret1",
									EnvVar: "SECRET_REF1",
								},
							}},
					}},
				},
			},
			jobs: []v1.RadixDeployJobComponent{
				{
					Name: jobName1,
					SecretRefs: v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{
						{
							Name: "keyVault2",
							Items: []v1.RadixAzureKeyVaultItem{
								{
									Name:   "secret2",
									EnvVar: "SECRET_REF2",
								},
							}},
					}},
				},
			},
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-keyvault1-csiazkvcreds-azkv-clientid",
					DisplayName: "Client ID",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
					Resource:    "keyVault1",
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
					ID:          secretModels.SecretIdClientId,
				},
				{
					Name:        "component1-keyvault1-csiazkvcreds-azkv-clientsecret",
					DisplayName: "Client Secret",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
					Resource:    "keyVault1",
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
					ID:          secretModels.SecretIdClientSecret,
				},
				{
					Name:        "AzureKeyVaultItem-keyVault1--secret--secret1",
					DisplayName: "secret secret1",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultItem,
					Resource:    "keyVault1",
					Component:   componentName1,
					Status:      secretModels.NotAvailable.String(),
					ID:          "secret/secret1",
				},
				{
					Name:        "job1-keyvault2-csiazkvcreds-azkv-clientid",
					DisplayName: "Client ID",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
					Resource:    "keyVault2",
					Component:   jobName1,
					Status:      secretModels.Pending.String(),
					ID:          secretModels.SecretIdClientId,
				},
				{
					Name:        "job1-keyvault2-csiazkvcreds-azkv-clientsecret",
					DisplayName: "Client Secret",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
					Resource:    "keyVault2",
					Component:   jobName1,
					Status:      secretModels.Pending.String(),
					ID:          secretModels.SecretIdClientSecret,
				},
				{
					Name:        "AzureKeyVaultItem-keyVault2--secret--secret2",
					DisplayName: "secret secret2",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultItem,
					Resource:    "keyVault2",
					Component:   jobName1,
					Status:      secretModels.NotAvailable.String(),
					ID:          "secret/secret1",
				},
			},
		},
		{
			name: "Secrets from Authentication with PassCertificateToUpstream with no secrets",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
				Authentication: &v1.Authentication{
					ClientCertificate: &v1.ClientCertificate{PassCertificateToUpstream: utils.BoolPtr(true)},
				},
				Ports:      []v1.ComponentPort{{Name: "http", Port: 8000}},
				PublicPort: "http",
			}},
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-clientcertca",
					DisplayName: "",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
				},
			},
		},
		{
			name: "Secrets from Authentication with PassCertificateToUpstream with existing secrets",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
				Authentication: &v1.Authentication{
					ClientCertificate: &v1.ClientCertificate{PassCertificateToUpstream: utils.BoolPtr(true)},
				},
				Ports:      []v1.ComponentPort{{Name: "http", Port: 8000}},
				PublicPort: "http",
			}},
			existingSecrets: []secretDescription{
				{
					secretName: "component1-clientcertca",
					secretData: map[string][]byte{
						"ca.crt": []byte("current certificate"),
					},
				},
			},
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-clientcertca",
					DisplayName: "",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      secretModels.Consistent.String(),
				},
			},
		},
		{
			name: "Secrets from Authentication with VerificationTypeOn",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
				Authentication: &v1.Authentication{
					ClientCertificate: &v1.ClientCertificate{Verification: getVerificationTypePtr(v1.VerificationTypeOn)},
				},
				Ports:      []v1.ComponentPort{{Name: "http", Port: 8000}},
				PublicPort: "http",
			}},
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-clientcertca",
					DisplayName: "",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
				},
			},
		},
	}

	for _, scenario := range scenarios {
		appName := anyAppName
		environment := anyEnvironment
		deploymentName := "deployment1"

		s.Run(fmt.Sprintf("test GetSecretsForDeployment: %s", scenario.name), func() {
			secretHandler, _ := s.prepareTestRun(ctrl, &scenario, appName, environment, deploymentName)

			secrets, err := secretHandler.GetSecretsForDeployment(appName, environment, deploymentName)

			s.Nil(err)
			s.assertSecrets(&scenario, secrets)
		})
	}
}

func (s *secretHandlerTestSuite) TestSecretHandler_GetAzureKeyVaultSecretRefStatuses() {
	ctrl := gomock.NewController(s.T())
	defer ctrl.Finish()
	const (
		deployment1    = "deployment1"
		componentName1 = "component1"
		componentName2 = "component2"
		componentName3 = "component3"
		jobName1       = "job1"
		keyVaultName1  = "keyVault1"
		keyVaultName2  = "keyVault2"
		keyVaultName3  = "keyVault3"
		secret1        = "secret1"
		secret2        = "secret2"
		secret3        = "secret3"
		secretEnvVar1  = "SECRET_REF1"
		secretEnvVar2  = "SECRET_REF2"
		secretEnvVar3  = "SECRET_REF3"
	)

	scenarios := []getSecretScenario{
		createScenario("All secret version statuses exist, but for component3",
			func(scenario *getSecretScenario) {
				scenario.components = []v1.RadixDeployComponent{
					createRadixDeployComponent(componentName1, keyVaultName1, v1.RadixAzureKeyVaultObjectTypeSecret, secret1, secretEnvVar1),
					createRadixDeployComponent(componentName2, keyVaultName2, v1.RadixAzureKeyVaultObjectTypeSecret, secret3, secretEnvVar3),
					createRadixDeployComponent(componentName3, keyVaultName3, v1.RadixAzureKeyVaultObjectTypeSecret, secret1, secretEnvVar1),
				}
				scenario.jobs = []v1.RadixDeployJobComponent{createRadixDeployJobComponent(jobName1, keyVaultName2, secret2, secretEnvVar2)}
				scenario.setExpectedSecretVersion(componentName1, keyVaultName1, v1.RadixAzureKeyVaultObjectTypeSecret, secret1, "version-c11", "replica-name-c11")
				scenario.setExpectedSecretVersion(componentName1, keyVaultName1, v1.RadixAzureKeyVaultObjectTypeSecret, secret1, "version-c12", "replica-name-c11")
				scenario.setExpectedSecretVersion(componentName1, keyVaultName1, v1.RadixAzureKeyVaultObjectTypeSecret, secret1, "version-c11", "replica-name-c12")
				scenario.setExpectedSecretVersion(componentName1, keyVaultName1, v1.RadixAzureKeyVaultObjectTypeSecret, secret1, "version-c3", "replica-name-c13")
				scenario.setExpectedSecretVersion(componentName2, keyVaultName2, v1.RadixAzureKeyVaultObjectTypeSecret, secret3, "version-c21", "replica-name-c21")
				scenario.setExpectedSecretVersion(componentName2, keyVaultName2, v1.RadixAzureKeyVaultObjectTypeSecret, secret3, "version-c22", "replica-name-c21")
				scenario.setExpectedSecretVersion(jobName1, keyVaultName2, v1.RadixAzureKeyVaultObjectTypeSecret, secret2, "version-j1", "replica-name-j1")
				scenario.setExpectedSecretVersion(jobName1, keyVaultName2, v1.RadixAzureKeyVaultObjectTypeSecret, secret2, "version-j2", "replica-name-j1")
				scenario.setExpectedSecretVersion(jobName1, keyVaultName2, v1.RadixAzureKeyVaultObjectTypeSecret, secret2, "version-j1", "replica-name-j2")
				initFunc := func(secretHandler *SecretHandler) {
					//map[componentName]map[azureKeyVaultName]secretProviderClassAndSecret
					componentAzKeyVaultSecretProviderClassNameMap := map[string]map[string]secretProviderClassAndSecret{
						componentName1: createSecretProviderClass(secretHandler.serviceAccount.SecretProviderClient, deployment1, &scenario.components[0]),
						componentName2: createSecretProviderClass(secretHandler.serviceAccount.SecretProviderClient, deployment1, &scenario.components[1]),
						componentName3: createSecretProviderClass(secretHandler.serviceAccount.SecretProviderClient, deployment1, &scenario.components[2]),
						jobName1:       createSecretProviderClass(secretHandler.serviceAccount.SecretProviderClient, deployment1, &scenario.jobs[0]),
					}
					createSecretProviderClassPodStatuses(secretHandler.serviceAccount.SecretProviderClient, scenario, componentAzKeyVaultSecretProviderClassNameMap)
				}
				scenario.init = &initFunc
			}),
		createScenario("No secret version statuses exist",
			func(scenario *getSecretScenario) {
				scenario.components = []v1.RadixDeployComponent{
					createRadixDeployComponent(componentName1, keyVaultName1, v1.RadixAzureKeyVaultObjectTypeSecret, secret1, secretEnvVar1),
					createRadixDeployComponent(componentName2, keyVaultName2, v1.RadixAzureKeyVaultObjectTypeSecret, secret3, secretEnvVar3),
					createRadixDeployComponent(componentName3, keyVaultName3, v1.RadixAzureKeyVaultObjectTypeSecret, secret1, secretEnvVar1),
				}
				scenario.jobs = []v1.RadixDeployJobComponent{createRadixDeployJobComponent(jobName1, keyVaultName2, secret2, secretEnvVar2)}
				initFunc := func(secretHandler *SecretHandler) {
					//map[componentName]map[azureKeyVaultName]createSecretProviderClassName
					componentAzKeyVaultSecretProviderClassNameMap := map[string]map[string]secretProviderClassAndSecret{
						componentName1: createSecretProviderClass(secretHandler.serviceAccount.SecretProviderClient, deployment1, &scenario.components[0]),
						componentName2: createSecretProviderClass(secretHandler.serviceAccount.SecretProviderClient, deployment1, &scenario.components[1]),
						componentName3: createSecretProviderClass(secretHandler.serviceAccount.SecretProviderClient, deployment1, &scenario.components[2]),
						jobName1:       createSecretProviderClass(secretHandler.serviceAccount.SecretProviderClient, deployment1, &scenario.jobs[0]),
					}
					createSecretProviderClassPodStatuses(secretHandler.serviceAccount.SecretProviderClient, scenario, componentAzKeyVaultSecretProviderClassNameMap)
				}
				scenario.init = &initFunc
			}),
	}

	for _, scenario := range scenarios {
		appName := anyAppName
		environment := anyEnvironment
		deploymentName := deployment1

		s.Run(fmt.Sprintf("test GetSecretsStatus: %s", scenario.name), func() {
			secretHandler, _ := s.prepareTestRun(ctrl, &scenario, appName, environment, deploymentName)

			actualSecretVersions := make(map[string]map[string]map[string]map[string]map[string]bool) //map[componentName]map[azureKeyVaultName]map[secretId]map[version]map[replicaName]bool
			for _, component := range scenario.components {
				s.appendActualSecretVersions(&component, secretHandler, appName, environment, actualSecretVersions)
			}
			for _, jobComponent := range scenario.jobs {
				s.appendActualSecretVersions(&jobComponent, secretHandler, appName, environment, actualSecretVersions)
			}
			s.assertSecretVersionStatuses(scenario.expectedSecretVersions, actualSecretVersions)
		})
	}
}

func (s *secretHandlerTestSuite) appendActualSecretVersions(component v1.RadixCommonDeployComponent, secretHandler SecretHandler, appName string, environment string, actualSecretVersions map[string]map[string]map[string]map[string]map[string]bool) {
	azureKeyVaultMap := make(map[string]map[string]map[string]map[string]bool) //map[azureKeyVaultName]map[secretId]map[version]map[replicaName]bool
	for _, azureKeyVault := range component.GetSecretRefs().AzureKeyVaults {
		itemSecretMap := make(map[string]map[string]map[string]bool) //map[secretId]map[version]map[replicaName]bool
		for _, item := range azureKeyVault.Items {
			secretId := secret.GetSecretIdForAzureKeyVaultItem(&item)

			secretVersions, err := secretHandler.GetAzureKeyVaultSecretVersions(appName, environment, component.GetName(), azureKeyVault.Name, secretId)
			s.Nil(err)

			versionReplicaNameMap := make(map[string]map[string]bool) //map[version]map[replicaName]bool
			for _, secretVersion := range secretVersions {
				if _, ok := versionReplicaNameMap[secretVersion.Version]; !ok {
					versionReplicaNameMap[secretVersion.Version] = make(map[string]bool)
				}
				versionReplicaNameMap[secretVersion.Version][secretVersion.ReplicaName] = true
			}
			if len(versionReplicaNameMap) > 0 {
				itemSecretMap[secretId] = versionReplicaNameMap
			}
		}
		if len(itemSecretMap) > 0 {
			azureKeyVaultMap[azureKeyVault.Name] = itemSecretMap
		}
	}
	if len(azureKeyVaultMap) > 0 {
		actualSecretVersions[component.GetName()] = azureKeyVaultMap
	}
}

func (s *secretHandlerTestSuite) TestSecretHandler_GetAzureKeyVaultSecretRefVersionStatuses() {
	ctrl := gomock.NewController(s.T())
	defer ctrl.Finish()
	const deployment1 = "deployment1"
	const componentName1 = "component1"
	const jobName1 = "job1"

	scenarios := []getSecretScenario{
		createScenarioWithComponentAndJobWithCredSecretsAndOneSecretPerComponent(
			"Not available, when no secret provider class",
			func(scenario *getSecretScenario) {
				scenario.setExpectedSecretStatus(componentName1, secretModels.SecretIdClientId, secretModels.Consistent)
				scenario.setExpectedSecretStatus(componentName1, secretModels.SecretIdClientSecret, secretModels.Consistent)
				scenario.setExpectedSecretStatus(componentName1, "secret/secret1", secretModels.NotAvailable)
				scenario.setExpectedSecretStatus(jobName1, secretModels.SecretIdClientId, secretModels.Consistent)
				scenario.setExpectedSecretStatus(jobName1, secretModels.SecretIdClientSecret, secretModels.Consistent)
				scenario.setExpectedSecretStatus(jobName1, "secret/secret2", secretModels.NotAvailable)
			}),
		createScenarioWithComponentAndJobWithCredSecretsAndOneSecretPerComponent(
			"Not available, when exists secret provider class, but no secret",
			func(scenario *getSecretScenario) {
				scenario.setExpectedSecretStatus(componentName1, secretModels.SecretIdClientId, secretModels.Consistent)
				scenario.setExpectedSecretStatus(componentName1, secretModels.SecretIdClientSecret, secretModels.Consistent)
				scenario.setExpectedSecretStatus(componentName1, "secret/secret1", secretModels.NotAvailable)
				scenario.setExpectedSecretStatus(jobName1, secretModels.SecretIdClientId, secretModels.Consistent)
				scenario.setExpectedSecretStatus(jobName1, secretModels.SecretIdClientSecret, secretModels.Consistent)
				scenario.setExpectedSecretStatus(jobName1, "secret/secret2", secretModels.NotAvailable)
				initFunc := func(secretHandler *SecretHandler) {
					createSecretProviderClass(secretHandler.serviceAccount.SecretProviderClient, deployment1, &scenario.components[0])
					createSecretProviderClass(secretHandler.serviceAccount.SecretProviderClient, deployment1, &scenario.jobs[0])
				}
				scenario.init = &initFunc
			}),
		createScenarioWithComponentAndJobWithCredSecretsAndOneSecretPerComponent(
			"Consistent, when exists secret provider class and secret",
			func(scenario *getSecretScenario) {
				scenario.setExpectedSecretStatus(componentName1, secretModels.SecretIdClientId, secretModels.Consistent)
				scenario.setExpectedSecretStatus(componentName1, secretModels.SecretIdClientSecret, secretModels.Consistent)
				scenario.setExpectedSecretStatus(componentName1, "secret/secret1", secretModels.Consistent)
				scenario.setExpectedSecretStatus(jobName1, secretModels.SecretIdClientId, secretModels.Consistent)
				scenario.setExpectedSecretStatus(jobName1, secretModels.SecretIdClientSecret, secretModels.Consistent)
				scenario.setExpectedSecretStatus(jobName1, "secret/secret2", secretModels.Consistent)
				initFunc := func(secretHandler *SecretHandler) {
					componentSecretMap := createSecretProviderClass(secretHandler.serviceAccount.SecretProviderClient, deployment1, &scenario.components[0])
					jobSecretMap := createSecretProviderClass(secretHandler.serviceAccount.SecretProviderClient, deployment1, &scenario.jobs[0])
					for _, secretProviderClassAndSecret := range componentSecretMap {
						createAzureKeyVaultCsiDriverSecret(secretHandler.userAccount.Client, secretProviderClassAndSecret.secretName, map[string]string{"SECRET1": "val1"})
					}
					for _, secretProviderClassAndSecret := range jobSecretMap {
						createAzureKeyVaultCsiDriverSecret(secretHandler.userAccount.Client, secretProviderClassAndSecret.secretName, map[string]string{"SECRET2": "val2"})
					}
				}
				scenario.init = &initFunc
			}),
	}

	for _, scenario := range scenarios {
		appName := anyAppName
		environment := anyEnvironment
		deploymentName := deployment1

		s.Run(fmt.Sprintf("test GetSecretsStatus: %s", scenario.name), func() {
			secretHandler, _ := s.prepareTestRun(ctrl, &scenario, appName, environment, deploymentName)

			secrets, err := secretHandler.GetSecretsForDeployment(appName, environment, deploymentName)

			s.Nil(err)
			s.assertSecrets(&scenario, secrets)
		})
	}
}

func (s *secretHandlerTestSuite) TestSecretHandler_GetAuthenticationSecrets() {
	ctrl := gomock.NewController(s.T())
	defer ctrl.Finish()
	componentName1 := "component1"
	scenarios := []struct {
		name            string
		modifyComponent func(*v1.RadixDeployComponent)
		expectedError   bool
		expectedSecrets []secretModels.Secret
	}{
		{
			name: "Secrets from Authentication with PassCertificateToUpstream",
			modifyComponent: func(component *v1.RadixDeployComponent) {
				component.Authentication = &v1.Authentication{
					ClientCertificate: &v1.ClientCertificate{PassCertificateToUpstream: utils.BoolPtr(true)},
				}
				component.PublicPort = "http"
			},
			expectedError: false,
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-clientcertca",
					DisplayName: "",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
				},
			},
		},
		{
			name: "Secrets from Authentication with VerificationTypeOn",
			modifyComponent: func(component *v1.RadixDeployComponent) {
				component.Authentication = &v1.Authentication{
					ClientCertificate: &v1.ClientCertificate{Verification: getVerificationTypePtr(v1.VerificationTypeOn)},
				}
				component.PublicPort = "http"
			},
			expectedError: false,
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-clientcertca",
					DisplayName: "",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
				},
			},
		},
		{
			name: "Secrets from Authentication with VerificationTypeOn",
			modifyComponent: func(component *v1.RadixDeployComponent) {
				component.Authentication = &v1.Authentication{
					ClientCertificate: &v1.ClientCertificate{Verification: getVerificationTypePtr(v1.VerificationTypeOn)},
				}
				component.PublicPort = "http"
			},
			expectedError: false,
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-clientcertca",
					DisplayName: "",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
				},
			},
		},
		{
			name: "Secrets from Authentication with VerificationTypeOptional",
			modifyComponent: func(component *v1.RadixDeployComponent) {
				component.Authentication = &v1.Authentication{
					ClientCertificate: &v1.ClientCertificate{Verification: getVerificationTypePtr(v1.VerificationTypeOptional)},
				}
				component.PublicPort = "http"
			},
			expectedError: false,
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-clientcertca",
					DisplayName: "",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
				},
			},
		},
		{
			name: "Secrets from Authentication with VerificationTypeOptionalNoCa",
			modifyComponent: func(component *v1.RadixDeployComponent) {
				component.Authentication = &v1.Authentication{
					ClientCertificate: &v1.ClientCertificate{Verification: getVerificationTypePtr(v1.VerificationTypeOptionalNoCa)},
				}
				component.PublicPort = "http"
			},
			expectedError: false,
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-clientcertca",
					DisplayName: "",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
				},
			},
		},
		{
			name: "No secrets from Authentication with VerificationTypeOff",
			modifyComponent: func(component *v1.RadixDeployComponent) {
				component.Authentication = &v1.Authentication{
					ClientCertificate: &v1.ClientCertificate{Verification: getVerificationTypePtr(v1.VerificationTypeOff)},
				}
				component.PublicPort = "http"
			},
			expectedError:   false,
			expectedSecrets: []secretModels.Secret{},
		},
		{
			name: "No secrets from Authentication for not public port",
			modifyComponent: func(component *v1.RadixDeployComponent) {
				component.Authentication = &v1.Authentication{
					ClientCertificate: &v1.ClientCertificate{Verification: getVerificationTypePtr(v1.VerificationTypeOn)},
				}
			},
			expectedError:   false,
			expectedSecrets: []secretModels.Secret{},
		},
		{
			name: "No secrets from Authentication with No Verification and PassCertificateToUpstream",
			modifyComponent: func(component *v1.RadixDeployComponent) {
				component.Authentication = &v1.Authentication{
					ClientCertificate: &v1.ClientCertificate{},
				}
				component.PublicPort = "http"
			},
			expectedError:   false,
			expectedSecrets: []secretModels.Secret{},
		},
	}

	for _, scenario := range scenarios {
		s.Run(fmt.Sprintf("test GetSecrets: %s", scenario.name), func() {
			environment := anyEnvironment
			appName := anyAppName
			deploymentName := "deployment1"
			commonScenario := getSecretScenario{
				name: scenario.name,
				components: []v1.RadixDeployComponent{{
					Name:  componentName1,
					Ports: []v1.ComponentPort{{Name: "http", Port: 8000}},
				}},
				expectedSecrets: scenario.expectedSecrets,
			}
			scenario.modifyComponent(&commonScenario.components[0])

			secretHandler, _ := s.prepareTestRun(ctrl, &commonScenario, appName, environment, deploymentName)

			secrets, err := secretHandler.GetSecretsForDeployment(appName, environment, deploymentName)

			s.Nil(err)
			s.assertSecrets(&commonScenario, secrets)
		})
	}
}

func (s *secretHandlerTestSuite) TestSecretHandler_ChangeSecrets() {
	ctrl := gomock.NewController(s.T())
	defer ctrl.Finish()
	componentName1 := "component1"
	jobName1 := "job1"
	volumeName1 := "volume1"
	azureKeyVaultName1 := "azureKeyVault1"
	//goland:noinspection ALL
	scenarios := []changeSecretScenario{
		{
			name: "Change regular secret in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
				Secrets: []string{
					"SECRET_C1",
				},
			}},
			secretName:                  "component1-sdiatyab",
			secretDataKey:               "SECRET_C1",
			secretValue:                 "current-value",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          "SECRET_C1",
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "new-value",
			},
			expectedError: false,
		},
		{
			name: "Change regular secret in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
				Secrets: []string{
					"SECRET_C1",
				},
			}},
			secretName:                  "job1-jvqbisnq",
			secretDataKey:               "SECRET_C1",
			secretValue:                 "current-value",
			secretExists:                true,
			changingSecretComponentName: jobName1,
			changingSecretName:          "SECRET_C1",
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "new-value",
			},
			expectedError: false,
		},
		{
			name: "Failed change of not existing regular secret in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
				Secrets: []string{
					"SECRET_C1",
				},
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          "SECRET_C1",
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "new-value",
			},
			expectedError: true,
		},
		{
			name: "Failed change of not existing regular secret in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
				Secrets: []string{
					"SECRET_C1",
				},
			}},
			secretExists:                false,
			changingSecretComponentName: jobName1,
			changingSecretName:          "SECRET_C1",
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "new-value",
			},
			expectedError: true,
		},
		{
			name: "Change External DNS cert in the component",
			components: []v1.RadixDeployComponent{{
				Name:    componentName1,
				Secrets: []string{"some-external-dns-secret"},
			}},
			secretName:                  "some-external-dns-secret",
			secretDataKey:               tlsCertPart,
			secretValue:                 "current tls certificate text\nline2\nline3",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          "some-external-dns-secret-cert",
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "new tls certificate text\nline2\nline3",
				Type:        secretModels.SecretTypeClientCert,
			},
			expectedError: false,
		},
		{
			name: "Change External DNS cert in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name:    jobName1,
				Secrets: []string{"some-external-dns-secret"},
			}},
			secretName:                  "some-external-dns-secret",
			secretDataKey:               tlsCertPart,
			secretValue:                 "current tls certificate text\nline2\nline3",
			secretExists:                true,
			changingSecretComponentName: jobName1,
			changingSecretName:          "some-external-dns-secret-cert",
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "new tls certificate text\nline2\nline3",
				Type:        secretModels.SecretTypeClientCert,
			},
			expectedError: false,
		},
		{
			name: "Failed change of not existing External DNS cert in the component",
			components: []v1.RadixDeployComponent{{
				Name:    componentName1,
				Secrets: []string{"some-external-dns-secret"},
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          "some-external-dns-secret-cert",
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "new tls certificate text\nline2\nline3",
				Type:        secretModels.SecretTypeClientCert,
			},
			expectedError: true,
		},
		{
			name: "Failed change of not existing External DNS cert in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name:    jobName1,
				Secrets: []string{"some-external-dns-secret"},
			}},
			secretExists:                false,
			changingSecretComponentName: jobName1,
			changingSecretName:          "some-external-dns-secret-cert",
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "new tls certificate text\nline2\nline3",
				Type:        secretModels.SecretTypeClientCert,
			},
			expectedError: true,
		},
		{
			name: "Change External DNS key in the component",
			components: []v1.RadixDeployComponent{{
				Name:    componentName1,
				Secrets: []string{"some-external-dns-secret"},
			}},
			secretName:                  "some-external-dns-secret",
			secretDataKey:               tlsKeyPart,
			secretValue:                 "current tls key text\nline2\nline3",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          "some-external-dns-secret-key",
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "new tls key text\nline2\nline3",
				Type:        secretModels.SecretTypeClientCert,
			},
			expectedError: false,
		},
		{
			name: "Change External DNS key in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name:    jobName1,
				Secrets: []string{"some-external-dns-secret"},
			}},
			secretName:                  "some-external-dns-secret",
			secretDataKey:               tlsKeyPart,
			secretValue:                 "current tls key text\nline2\nline3",
			secretExists:                true,
			changingSecretComponentName: jobName1,
			changingSecretName:          "some-external-dns-secret-key",
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "new tls key text\nline2\nline3",
				Type:        secretModels.SecretTypeClientCert,
			},
			expectedError: false,
		},
		{
			name: "Failed change of not existing External DNS key in the component",
			components: []v1.RadixDeployComponent{{
				Name:    componentName1,
				Secrets: []string{"some-external-dns-secret"},
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          "some-external-dns-secret-key",
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "new tls key text\nline2\nline3",
				Type:        secretModels.SecretTypeClientCert,
			},
			expectedError: true,
		},
		{
			name: "Failed change of not existing External DNS key in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name:    jobName1,
				Secrets: []string{"some-external-dns-secret"},
			}},
			secretExists:                false,
			changingSecretComponentName: jobName1,
			changingSecretName:          "some-external-dns-secret-key",
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "new tls key text\nline2\nline3",
				Type:        secretModels.SecretTypeClientCert,
			},
			expectedError: true,
		},
		{
			name: "Change CSI Azure Blob volume account name in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretName:                  defaults.GetCsiAzureVolumeMountCredsSecretName(componentName1, volumeName1),
			secretDataKey:               defaults.CsiAzureCredsAccountNamePart,
			secretValue:                 "currentAccountName",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureVolumeMountCredsSecretName(componentName1, volumeName1) + defaults.CsiAzureCredsAccountNamePartSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newAccountName",
				Type:        secretModels.SecretTypeCsiAzureBlobVolume,
			},
			expectedError: false,
		},
		{
			name: "Change CSI Azure Blob volume account name in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
			}},
			secretName:                  defaults.GetCsiAzureVolumeMountCredsSecretName(jobName1, volumeName1),
			secretDataKey:               defaults.CsiAzureCredsAccountNamePart,
			secretValue:                 "currentAccountName",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureVolumeMountCredsSecretName(jobName1, volumeName1) + defaults.CsiAzureCredsAccountNamePartSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newAccountName",
				Type:        secretModels.SecretTypeCsiAzureBlobVolume,
			},
			expectedError: false,
		},
		{
			name: "Failed change of not existing CSI Azure Blob volume account name in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureVolumeMountCredsSecretName(componentName1, volumeName1) + defaults.CsiAzureCredsAccountNamePartSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newAccountName",
				Type:        secretModels.SecretTypeCsiAzureBlobVolume,
			},
			expectedError: true,
		},
		{
			name: "Failed change of not existing CSI Azure Blob volume account name in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureVolumeMountCredsSecretName(jobName1, volumeName1) + defaults.CsiAzureCredsAccountNamePartSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newAccountName",
				Type:        secretModels.SecretTypeCsiAzureBlobVolume,
			},
			expectedError: true,
		},
		{
			name: "Change CSI Azure Blob volume account key in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretName:                  defaults.GetCsiAzureVolumeMountCredsSecretName(componentName1, volumeName1),
			secretDataKey:               defaults.CsiAzureCredsAccountKeyPart,
			secretValue:                 "currentAccountKey",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureVolumeMountCredsSecretName(componentName1, volumeName1) + defaults.CsiAzureCredsAccountKeyPartSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newAccountKey",
				Type:        secretModels.SecretTypeCsiAzureBlobVolume,
			},
			expectedError: false,
		},
		{
			name: "Change CSI Azure Blob volume account key in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
			}},
			secretName:                  defaults.GetCsiAzureVolumeMountCredsSecretName(jobName1, volumeName1),
			secretDataKey:               defaults.CsiAzureCredsAccountKeyPart,
			secretValue:                 "currentAccountKey",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureVolumeMountCredsSecretName(jobName1, volumeName1) + defaults.CsiAzureCredsAccountKeyPartSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newAccountKey",
				Type:        secretModels.SecretTypeCsiAzureBlobVolume,
			},
			expectedError: false,
		},
		{
			name: "Failed change of not existing CSI Azure Blob volume account key in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureVolumeMountCredsSecretName(componentName1, volumeName1) + defaults.CsiAzureCredsAccountKeyPartSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newAccountKey",
				Type:        secretModels.SecretTypeCsiAzureBlobVolume,
			},
			expectedError: true,
		},
		{
			name: "Failed change of not existing CSI Azure Blob volume account key in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureVolumeMountCredsSecretName(jobName1, volumeName1) + defaults.CsiAzureCredsAccountKeyPartSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newAccountKey",
				Type:        secretModels.SecretTypeCsiAzureBlobVolume,
			},
			expectedError: true,
		},
		{
			name: "Change CSI Azure Key vault client ID in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretName:                  defaults.GetCsiAzureKeyVaultCredsSecretName(componentName1, azureKeyVaultName1),
			secretDataKey:               defaults.CsiAzureKeyVaultCredsClientIdPart,
			secretValue:                 "currentClientId",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureKeyVaultCredsSecretName(componentName1, azureKeyVaultName1) + defaults.CsiAzureKeyVaultCredsClientIdSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientId",
				Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
			},
			expectedError: false,
		},
		{
			name: "Change CSI Azure Key vault client ID in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
			}},
			secretName:                  defaults.GetCsiAzureKeyVaultCredsSecretName(jobName1, azureKeyVaultName1),
			secretDataKey:               defaults.CsiAzureKeyVaultCredsClientIdPart,
			secretValue:                 "currentClientId",
			secretExists:                true,
			changingSecretComponentName: jobName1,
			changingSecretName:          defaults.GetCsiAzureKeyVaultCredsSecretName(jobName1, azureKeyVaultName1) + defaults.CsiAzureKeyVaultCredsClientIdSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientId",
				Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
			},
			expectedError: false,
		},
		{
			name: "Failed change of not existing CSI Azure Key vault client ID in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureKeyVaultCredsSecretName(componentName1, azureKeyVaultName1) + defaults.CsiAzureKeyVaultCredsClientIdSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientId",
				Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
			},
			expectedError: true,
		},
		{
			name: "Failed change of not existing CSI Azure Key vault client ID in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
			}},
			secretExists:                false,
			changingSecretComponentName: jobName1,
			changingSecretName:          defaults.GetCsiAzureKeyVaultCredsSecretName(jobName1, azureKeyVaultName1) + defaults.CsiAzureKeyVaultCredsClientIdSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientId",
				Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
			},
			expectedError: true,
		},
		{
			name: "Change CSI Azure Key vault client secret in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretName:                  defaults.GetCsiAzureKeyVaultCredsSecretName(componentName1, azureKeyVaultName1),
			secretDataKey:               defaults.CsiAzureKeyVaultCredsClientSecretPart,
			secretValue:                 "currentClientId",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureKeyVaultCredsSecretName(componentName1, azureKeyVaultName1) + defaults.CsiAzureKeyVaultCredsClientSecretSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientId",
				Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
			},
			expectedError: false,
		},
		{
			name: "Change CSI Azure Key vault client secret in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
			}},
			secretName:                  defaults.GetCsiAzureKeyVaultCredsSecretName(jobName1, azureKeyVaultName1),
			secretDataKey:               defaults.CsiAzureKeyVaultCredsClientSecretPart,
			secretValue:                 "currentClientSecret",
			secretExists:                true,
			changingSecretComponentName: jobName1,
			changingSecretName:          defaults.GetCsiAzureKeyVaultCredsSecretName(jobName1, azureKeyVaultName1) + defaults.CsiAzureKeyVaultCredsClientSecretSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientSecret",
				Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
			},
			expectedError: false,
		},
		{
			name: "Failed change of not existing CSI Azure Key vault client secret in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureKeyVaultCredsSecretName(componentName1, azureKeyVaultName1) + defaults.CsiAzureKeyVaultCredsClientSecretSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientId",
				Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
			},
			expectedError: true,
		},
		{
			name: "Failed change of not existing CSI Azure Key vault client secret in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
			}},
			secretExists:                false,
			changingSecretComponentName: jobName1,
			changingSecretName:          defaults.GetCsiAzureKeyVaultCredsSecretName(jobName1, azureKeyVaultName1) + defaults.CsiAzureKeyVaultCredsClientSecretSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientSecret",
				Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
			},
			expectedError: true,
		},
		{
			name: "Change OAuth2 client secret key in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretName:                  operatorUtils.GetAuxiliaryComponentSecretName(componentName1, defaults.OAuthProxyAuxiliaryComponentSuffix),
			secretDataKey:               defaults.OAuthClientSecretKeyName,
			secretValue:                 "currentClientSecretKey",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          operatorUtils.GetAuxiliaryComponentSecretName(componentName1, defaults.OAuthProxyAuxiliaryComponentSuffix) + suffix.OAuth2ClientSecret,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientSecretKey",
				Type:        secretModels.SecretTypeOAuth2Proxy,
			},
			expectedError: false,
		},
		{
			name: "Change OAuth2 client secret key in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
			}},
			secretName:                  operatorUtils.GetAuxiliaryComponentSecretName(jobName1, defaults.OAuthProxyAuxiliaryComponentSuffix),
			secretDataKey:               defaults.OAuthClientSecretKeyName,
			secretValue:                 "currentClientSecretKey",
			secretExists:                true,
			changingSecretComponentName: jobName1,
			changingSecretName:          operatorUtils.GetAuxiliaryComponentSecretName(jobName1, defaults.OAuthProxyAuxiliaryComponentSuffix) + suffix.OAuth2ClientSecret,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientSecretKey",
				Type:        secretModels.SecretTypeOAuth2Proxy,
			},
			expectedError: false,
		},
		{
			name: "Failed change of not existing OAuth2 client secret key in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          operatorUtils.GetAuxiliaryComponentSecretName(componentName1, defaults.OAuthProxyAuxiliaryComponentSuffix) + suffix.OAuth2ClientSecret,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientSecretKey",
				Type:        secretModels.SecretTypeOAuth2Proxy,
			},
			expectedError: true,
		},
		{
			name: "Failed change of not existing OAuth2 client secret key in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
			}},
			secretExists:                false,
			changingSecretComponentName: jobName1,
			changingSecretName:          operatorUtils.GetAuxiliaryComponentSecretName(jobName1, defaults.OAuthProxyAuxiliaryComponentSuffix) + suffix.OAuth2ClientSecret,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientSecretKey",
				Type:        secretModels.SecretTypeOAuth2Proxy,
			},
			expectedError: true,
		},
		{
			name: "Change OAuth2 cookie secret in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretName:                  operatorUtils.GetAuxiliaryComponentSecretName(componentName1, defaults.OAuthProxyAuxiliaryComponentSuffix),
			secretDataKey:               defaults.OAuthCookieSecretKeyName,
			secretValue:                 "currentCookieSecretKey",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          operatorUtils.GetAuxiliaryComponentSecretName(componentName1, defaults.OAuthProxyAuxiliaryComponentSuffix) + suffix.OAuth2CookieSecret,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newCookieSecretKey",
				Type:        secretModels.SecretTypeOAuth2Proxy,
			},
			expectedError: false,
		},
		{
			name: "Change OAuth2 cookie secret in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
			}},
			secretName:                  operatorUtils.GetAuxiliaryComponentSecretName(jobName1, defaults.OAuthProxyAuxiliaryComponentSuffix),
			secretDataKey:               defaults.OAuthCookieSecretKeyName,
			secretValue:                 "currentCookieSecretKey",
			secretExists:                true,
			changingSecretComponentName: jobName1,
			changingSecretName:          operatorUtils.GetAuxiliaryComponentSecretName(jobName1, defaults.OAuthProxyAuxiliaryComponentSuffix) + suffix.OAuth2CookieSecret,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newCookieSecretKey",
				Type:        secretModels.SecretTypeOAuth2Proxy,
			},
			expectedError: false,
		},
		{
			name: "Failed change of not existing OAuth2 cookie secret in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          operatorUtils.GetAuxiliaryComponentSecretName(componentName1, defaults.OAuthProxyAuxiliaryComponentSuffix) + suffix.OAuth2CookieSecret,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newCookieSecretKey",
				Type:        secretModels.SecretTypeOAuth2Proxy,
			},
			expectedError: true,
		},
		{
			name: "Failed change of not existing OAuth2 cookie secret in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
			}},
			secretExists:                false,
			changingSecretComponentName: jobName1,
			changingSecretName:          operatorUtils.GetAuxiliaryComponentSecretName(jobName1, defaults.OAuthProxyAuxiliaryComponentSuffix) + suffix.OAuth2CookieSecret,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newCookieSecretKey",
				Type:        secretModels.SecretTypeOAuth2Proxy,
			},
			expectedError: true,
		},
		{
			name: "Change OAuth2 Redis password in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretName:                  operatorUtils.GetAuxiliaryComponentSecretName(componentName1, defaults.OAuthProxyAuxiliaryComponentSuffix),
			secretDataKey:               defaults.OAuthRedisPasswordKeyName,
			secretValue:                 "currentRedisPassword",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          operatorUtils.GetAuxiliaryComponentSecretName(componentName1, defaults.OAuthProxyAuxiliaryComponentSuffix) + suffix.OAuth2RedisPassword,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newRedisPassword",
				Type:        secretModels.SecretTypeOAuth2Proxy,
			},
			expectedError: false,
		},
		{
			name: "Change OAuth2 Redis password in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
			}},
			secretName:                  operatorUtils.GetAuxiliaryComponentSecretName(jobName1, defaults.OAuthProxyAuxiliaryComponentSuffix),
			secretDataKey:               defaults.OAuthRedisPasswordKeyName,
			secretValue:                 "currentRedisPassword",
			secretExists:                true,
			changingSecretComponentName: jobName1,
			changingSecretName:          operatorUtils.GetAuxiliaryComponentSecretName(jobName1, defaults.OAuthProxyAuxiliaryComponentSuffix) + suffix.OAuth2RedisPassword,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newRedisPassword",
				Type:        secretModels.SecretTypeOAuth2Proxy,
			},
			expectedError: false,
		},
		{
			name: "Failed change of not existing OAuth2 Redis password in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          operatorUtils.GetAuxiliaryComponentSecretName(componentName1, defaults.OAuthProxyAuxiliaryComponentSuffix) + suffix.OAuth2RedisPassword,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newRedisPassword",
				Type:        secretModels.SecretTypeOAuth2Proxy,
			},
			expectedError: true,
		},
		{
			name: "Failed change of not existing OAuth2 Redis password in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
			}},
			secretExists:                false,
			changingSecretComponentName: jobName1,
			changingSecretName:          operatorUtils.GetAuxiliaryComponentSecretName(jobName1, defaults.OAuthProxyAuxiliaryComponentSuffix) + suffix.OAuth2RedisPassword,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newRedisPassword",
				Type:        secretModels.SecretTypeOAuth2Proxy,
			},
			expectedError: true,
		},
		{
			name: "Change client certificate in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretName:                  "client-certificate1-clientcertca",
			secretDataKey:               "ca.crt",
			secretValue:                 "current client certificate\nline2\nline3",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          "client-certificate1" + suffix.ClientCertificate,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "new client certificate\nline2\nline3",
				Type:        secretModels.SecretTypeClientCert,
			},
			expectedError: false,
		},
		{
			name: "Failed change of not existing client certificate in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          "client-certificate1" + suffix.ClientCertificate,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "new client certificate\nline2\nline3",
				Type:        secretModels.SecretTypeClientCert,
			},
			expectedError: true,
		},
	}

	for _, scenario := range scenarios {
		s.Run(fmt.Sprintf("test GetSecrets: %s", scenario.name), func() {
			appName := anyAppName
			envName := anyEnvironment
			userAccount, serviceAccount, kubeClient, _ := s.getUtils()
			secretHandler := SecretHandler{
				userAccount:    *userAccount,
				serviceAccount: *serviceAccount,
			}
			appEnvNamespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
			if scenario.secretExists {
				_, _ = kubeClient.CoreV1().Secrets(appEnvNamespace).Create(context.Background(), &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: scenario.secretName, Namespace: appEnvNamespace},
					Data:       map[string][]byte{scenario.secretDataKey: []byte(scenario.secretValue)},
				}, metav1.CreateOptions{})
			}

			err := secretHandler.ChangeComponentSecret(appName, envName, scenario.changingSecretComponentName, scenario.changingSecretName, scenario.changingSecretParams)

			s.Equal(scenario.expectedError, err != nil, getErrorMessage(err))
			if scenario.secretExists && err == nil {
				changedSecret, _ := kubeClient.CoreV1().Secrets(appEnvNamespace).Get(context.Background(), scenario.secretName, metav1.GetOptions{})
				s.NotNil(changedSecret)
				s.Equal(scenario.changingSecretParams.SecretValue, string(changedSecret.Data[scenario.secretDataKey]))
			}
		})
	}
}

func getErrorMessage(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

func (s *secretHandlerTestSuite) assertSecrets(scenario *getSecretScenario, secrets []secretModels.Secret) {
	s.Equal(len(scenario.expectedSecrets), len(secrets))
	secretMap := getSecretMap(secrets)
	for _, expectedSecret := range scenario.expectedSecrets {
		secret, exists := secretMap[expectedSecret.Name]
		s.True(exists, "Missed secret %s", expectedSecret.Name)
		s.Equal(expectedSecret.Type, secret.Type, "Not expected secret Type for %s", expectedSecret.String())
		s.Equal(expectedSecret.Component, secret.Component, "Not expected secret Component for %s", expectedSecret.String())
		s.Equal(expectedSecret.DisplayName, secret.DisplayName, "Not expected secret Component for %s", expectedSecret.String())
		s.Equal(expectedSecret.Status, secret.Status, "Not expected secret Status for %s", expectedSecret.String())
		s.Equal(expectedSecret.Resource, secret.Resource, "Not expected secret Resource for %s", expectedSecret.String())
	}
}

func (s *secretHandlerTestSuite) assertSecretVersionStatuses(expectedVersionsMap map[string]map[string]map[string]map[string]map[string]bool, actualVersionsMap map[string]map[string]map[string]map[string]map[string]bool) {
	//maps: map[componentName]map[azureKeyVaultName]map[secretId]map[version]map[replicaName]bool
	s.Equal(len(expectedVersionsMap), len(actualVersionsMap), "Not equal component count")
	for componentName, actualAzureKeyVaultMap := range actualVersionsMap {
		expectedAzureKeyVaultMap, ok := expectedVersionsMap[componentName]
		s.True(ok, "Missing AzureKeyVaults for the component %s", componentName)
		s.Equal(len(expectedAzureKeyVaultMap), len(actualAzureKeyVaultMap), "Not equal AzureKeyVaults count for the component %s", componentName)
		for azKeyVaultName, actualItemsMap := range actualAzureKeyVaultMap {
			expectedItemsMap, ok := expectedAzureKeyVaultMap[azKeyVaultName]
			s.True(ok, "Missing AzureKeyVault items for the component %s, Azure Key vault %s", componentName, azKeyVaultName)
			s.Equal(len(expectedItemsMap), len(actualItemsMap), "Not equal AzureKeyVault items count for the component %s, Azure Key vault %s", componentName, azKeyVaultName)
			for secretId, actualVersionsMap := range actualItemsMap {
				expectedVersionsMap, ok := expectedItemsMap[secretId]
				s.True(ok, "Missing AzureKeyVault item secretId for the component %s, Azure Key vault %s secretId %s", componentName, azKeyVaultName, secretId)
				s.Equal(len(expectedVersionsMap), len(actualVersionsMap), "Not equal AzureKeyVault items count for the component %s, Azure Key vault %s secretId %s", componentName, azKeyVaultName, secretId)
				for version, actualReplicaNamesMap := range actualVersionsMap {
					expectedReplicaNamesMap, ok := expectedVersionsMap[version]
					s.True(ok, "Missing AzureKeyVault item secretId version for the component %s, Azure Key vault %s secretId %s version %s", componentName, azKeyVaultName, secretId, version)
					s.Equal(len(expectedReplicaNamesMap), len(actualReplicaNamesMap), "Not equal AzureKeyVault items count for the component %s, Azure Key vault %s secretId %s version %s", componentName, azKeyVaultName, secretId, version)
					for replicaName := range actualReplicaNamesMap {
						_, ok := expectedReplicaNamesMap[replicaName]
						s.True(ok, "Missing AzureKeyVault item secretId version replica for the component %s, Azure Key vault %s secretId %s version %s replicaName %s", componentName, azKeyVaultName, secretId, version, replicaName)
					}
				}
			}
		}
	}
}

func (s *secretHandlerTestSuite) prepareTestRun(ctrl *gomock.Controller, scenario *getSecretScenario, appName, envName, deploymentName string) (SecretHandler, *deployMock.MockDeployHandler) {
	userAccount, serviceAccount, kubeClient, radixClient := s.getUtils()
	deployHandler := deployMock.NewMockDeployHandler(ctrl)
	secretHandler := SecretHandler{
		userAccount:    *userAccount,
		serviceAccount: *serviceAccount,
		deployHandler:  deployHandler,
	}
	appAppNamespace := operatorUtils.GetAppNamespace(appName)
	ra := &v1.RadixApplication{
		ObjectMeta: metav1.ObjectMeta{Name: appName, Namespace: appAppNamespace},
		Spec: v1.RadixApplicationSpec{
			Environments: []v1.Environment{{Name: envName}},
			Components:   getRadixComponents(scenario.components, envName),
			Jobs:         getRadixJobComponents(scenario.jobs, envName),
		},
	}
	_, _ = radixClient.RadixV1().RadixApplications(appAppNamespace).Create(context.Background(), ra, metav1.CreateOptions{})
	radixDeployment := v1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: deploymentName},
		Spec: v1.RadixDeploymentSpec{
			Environment: envName,
			Components:  scenario.components,
			Jobs:        scenario.jobs,
		},
	}
	envNamespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	_, _ = radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.Background(), &radixDeployment, metav1.CreateOptions{})
	if scenario.init != nil {
		(*scenario.init)(&secretHandler) //scenario optional custom init function
	}
	for _, secret := range scenario.existingSecrets {
		_, _ = kubeClient.CoreV1().Secrets(envNamespace).Create(context.Background(), &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secret.secretName,
				Namespace: envNamespace,
				Labels:    secret.labels,
			},
			Data: secret.secretData,
		}, metav1.CreateOptions{})
	}
	return secretHandler, deployHandler
}

func getRadixComponents(components []v1.RadixDeployComponent, envName string) []v1.RadixComponent {
	var radixComponents []v1.RadixComponent
	for _, radixDeployComponent := range components {
		radixComponents = append(radixComponents, v1.RadixComponent{
			Name:       radixDeployComponent.Name,
			Variables:  radixDeployComponent.GetEnvironmentVariables(),
			Secrets:    radixDeployComponent.Secrets,
			SecretRefs: radixDeployComponent.SecretRefs,
			EnvironmentConfig: []v1.RadixEnvironmentConfig{{
				Environment:  envName,
				VolumeMounts: radixDeployComponent.VolumeMounts,
			}},
		})
	}
	return radixComponents
}

func getRadixJobComponents(jobComponents []v1.RadixDeployJobComponent, envName string) []v1.RadixJobComponent {
	var radixComponents []v1.RadixJobComponent
	for _, radixDeployJobComponent := range jobComponents {
		radixComponents = append(radixComponents, v1.RadixJobComponent{
			Name:       radixDeployJobComponent.Name,
			Variables:  radixDeployJobComponent.GetEnvironmentVariables(),
			Secrets:    radixDeployJobComponent.Secrets,
			SecretRefs: radixDeployJobComponent.SecretRefs,
			EnvironmentConfig: []v1.RadixJobComponentEnvironmentConfig{{
				Environment:  envName,
				VolumeMounts: radixDeployJobComponent.VolumeMounts,
			}},
		})
	}
	return radixComponents
}

func getSecretMap(secrets []secretModels.Secret) map[string]secretModels.Secret {
	secretMap := make(map[string]secretModels.Secret, len(secrets))
	for _, secret := range secrets {
		secret := secret
		secretMap[secret.Name] = secret
	}
	return secretMap
}

func (s *secretHandlerTestSuite) getUtils() (*models.Account, *models.Account, *kubefake.Clientset, *radixfake.Clientset) {
	kubeClient := kubefake.NewSimpleClientset()
	radixClient := radixfake.NewSimpleClientset()
	secretProviderClient := secretproviderfake.NewSimpleClientset()
	userAccount := models.Account{
		Client:               kubeClient,
		RadixClient:          radixClient,
		SecretProviderClient: secretProviderClient,
	}
	serviceAccount := models.Account{
		Client:               kubeClient,
		RadixClient:          radixClient,
		SecretProviderClient: secretProviderClient,
	}
	return &userAccount, &serviceAccount, kubeClient, radixClient
}

func getVerificationTypePtr(verificationType v1.VerificationType) *v1.VerificationType {
	return &verificationType
}

func (scenario *getSecretScenario) setExpectedSecretStatus(componentName, secretId string, status secretModels.SecretStatus) {
	for i, expectedSecret := range scenario.expectedSecrets {
		if strings.EqualFold(expectedSecret.Component, componentName) && strings.EqualFold(expectedSecret.ID, secretId) {
			scenario.expectedSecrets[i].Status = status.String()
			return
		}
	}
}

func (scenario *getSecretScenario) setExpectedSecretVersion(componentName, azureKeyVaultName string, secretType v1.RadixAzureKeyVaultObjectType, secretName, version, replicaName string) {
	secretId := fmt.Sprintf("%s/%s", string(secretType), secretName)
	if scenario.expectedSecretVersions == nil {
		scenario.expectedSecretVersions = make(map[string]map[string]map[string]map[string]map[string]bool) //map[componentName]map[azureKeyVaultName]map[secretId]map[version]map[replicaName]bool
	}
	if _, ok := scenario.expectedSecretVersions[componentName]; !ok {
		scenario.expectedSecretVersions[componentName] = make(map[string]map[string]map[string]map[string]bool)
	}
	if _, ok := scenario.expectedSecretVersions[componentName][azureKeyVaultName]; !ok {
		scenario.expectedSecretVersions[componentName][azureKeyVaultName] = make(map[string]map[string]map[string]bool)
	}
	if _, ok := scenario.expectedSecretVersions[componentName][azureKeyVaultName][secretId]; !ok {
		scenario.expectedSecretVersions[componentName][azureKeyVaultName][secretId] = make(map[string]map[string]bool)
	}
	if _, ok := scenario.expectedSecretVersions[componentName][azureKeyVaultName][secretId][version]; !ok {
		scenario.expectedSecretVersions[componentName][azureKeyVaultName][secretId][version] = make(map[string]bool)
	}
	scenario.expectedSecretVersions[componentName][azureKeyVaultName][secretId][version][replicaName] = true
}

func createAzureKeyVaultCsiDriverSecret(kubeClient kubernetes.Interface, secretName string, data map[string]string) {
	secretData := make(map[string][]byte)
	for key, value := range data {
		secretData[key] = []byte(value)
	}
	namespace := operatorUtils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	_, err := kubeClient.CoreV1().Secrets(namespace).Create(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   secretName,
			Labels: map[string]string{secretStoreCsiManagedLabel: "true"},
		},
		Data: secretData,
	}, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
}

func createScenario(name string, modify func(*getSecretScenario)) getSecretScenario {
	scenario := getSecretScenario{
		name: name,
	}
	modify(&scenario)
	return scenario
}

func createScenarioWithComponentAndJobWithCredSecretsAndOneSecretPerComponent(name string, modify func(*getSecretScenario)) getSecretScenario {
	scenario := createScenario(name, func(scenario *getSecretScenario) {
		const (
			componentName1 = "component1"
			jobName1       = "job1"
			keyVaultName1  = "keyVault1"
			keyVaultName2  = "keyVault2"
		)
		scenario.components = []v1.RadixDeployComponent{
			createRadixDeployComponent(componentName1, keyVaultName1, "", "secret1", "SECRET_REF1"),
		}
		scenario.jobs = []v1.RadixDeployJobComponent{
			createRadixDeployJobComponent(jobName1, keyVaultName2, "secret2", "SECRET_REF2"),
		}
		scenario.existingSecrets = []secretDescription{
			{
				secretName: "component1-keyvault1-csiazkvcreds",
				secretData: map[string][]byte{
					"clientid":     []byte("current client id1"),
					"clientsecret": []byte("current client secret1"),
				},
			},
			{
				secretName: "job1-keyvault2-csiazkvcreds",
				secretData: map[string][]byte{
					"clientid":     []byte("current client id2"),
					"clientsecret": []byte("current client secret2"),
				},
			},
		}
		scenario.expectedSecrets = []secretModels.Secret{
			{
				Name:        "component1-keyvault1-csiazkvcreds-azkv-clientid",
				DisplayName: "Client ID",
				Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
				Resource:    keyVaultName1,
				Component:   componentName1,
				Status:      secretModels.Pending.String(),
				ID:          secretModels.SecretIdClientId,
			},
			{
				Name:        "component1-keyvault1-csiazkvcreds-azkv-clientsecret",
				DisplayName: "Client Secret",
				Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
				Resource:    keyVaultName1,
				Component:   componentName1,
				Status:      secretModels.Pending.String(),
				ID:          secretModels.SecretIdClientSecret,
			},
			{
				Name:        "AzureKeyVaultItem-keyVault1--secret--secret1",
				DisplayName: "secret secret1",
				Type:        secretModels.SecretTypeCsiAzureKeyVaultItem,
				Resource:    keyVaultName1,
				Component:   componentName1,
				Status:      secretModels.Pending.String(),
				ID:          "secret/secret1",
			},
			{
				Name:        "job1-keyvault2-csiazkvcreds-azkv-clientid",
				DisplayName: "Client ID",
				Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
				Resource:    keyVaultName2,
				Component:   jobName1,
				Status:      secretModels.Pending.String(),
				ID:          secretModels.SecretIdClientId,
			},
			{
				Name:        "job1-keyvault2-csiazkvcreds-azkv-clientsecret",
				DisplayName: "Client Secret",
				Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
				Resource:    keyVaultName2,
				Component:   jobName1,
				Status:      secretModels.Pending.String(),
				ID:          secretModels.SecretIdClientSecret,
			},
			{
				Name:        "AzureKeyVaultItem-keyVault2--secret--secret2",
				DisplayName: "secret secret2",
				Type:        secretModels.SecretTypeCsiAzureKeyVaultItem,
				Resource:    keyVaultName2,
				Component:   jobName1,
				Status:      secretModels.NotAvailable.String(),
				ID:          "secret/secret2",
			},
		}

	})
	modify(&scenario)
	return scenario
}

func createRadixDeployComponent(componentName, keyVaultName string, secretObjectType v1.RadixAzureKeyVaultObjectType, secretName, envVarName string) v1.RadixDeployComponent {
	return v1.RadixDeployComponent{
		Name: componentName,
		SecretRefs: v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{
			{
				Name: keyVaultName,
				Items: []v1.RadixAzureKeyVaultItem{
					{
						Name:   secretName,
						EnvVar: envVarName,
						Type:   &secretObjectType,
					},
				}},
		}},
	}
}

func createRadixDeployJobComponent(componentName, keyVaultName, secretName, envVarName string) v1.RadixDeployJobComponent {
	return v1.RadixDeployJobComponent{
		Name: componentName,
		SecretRefs: v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{
			{
				Name: keyVaultName,
				Items: []v1.RadixAzureKeyVaultItem{
					{
						Name:   secretName,
						EnvVar: envVarName,
					},
				}},
		}},
	}
}

func createSecretProviderClass(secretProviderClient secretProviderClient.Interface, radixDeploymentName string, component v1.RadixCommonDeployComponent) map[string]secretProviderClassAndSecret {
	azureKeyVaultSecretProviderClassNameMap := make(map[string]secretProviderClassAndSecret)
	for _, azureKeyVault := range component.GetSecretRefs().AzureKeyVaults {
		secretProviderClass, err := kube.BuildAzureKeyVaultSecretProviderClass(tenantId, anyAppName, radixDeploymentName, component.GetName(), azureKeyVault)
		if err != nil {
			panic(err)
		}
		namespace := operatorUtils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
		_, err = secretProviderClient.SecretsstoreV1().SecretProviderClasses(namespace).Create(context.Background(),
			secretProviderClass, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
		azureKeyVaultSecretProviderClassNameMap[azureKeyVault.Name] = secretProviderClassAndSecret{
			secretName: secretProviderClass.Spec.SecretObjects[0].SecretName,
			className:  secretProviderClass.GetName(),
		}
	}
	return azureKeyVaultSecretProviderClassNameMap //map[componentName]map[azureKeyVaultName]createSecretProviderClassName
}

func createSecretProviderClassPodStatuses(secretProviderClient secretProviderClient.Interface, scenario *getSecretScenario, componentAzKeyVaultSecretProviderClassNameMap map[string]map[string]secretProviderClassAndSecret) {
	namespace := operatorUtils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	//scenario.expectedSecretVersions: map[componentName]map[azureKeyVaultName]map[secretId]map[version]map[replicaName]bool
	for componentName, azKeyVaultMap := range scenario.expectedSecretVersions {
		//componentAzKeyVaultSecretProviderClassNameMap map[componentName]map[AzureKeyVault]SecretName
		secretProviderClassNameMap, ok := componentAzKeyVaultSecretProviderClassNameMap[componentName]
		if !ok {
			continue
		}
		for azKeyVaultName, secretIdMap := range azKeyVaultMap {
			secretProviderClassAndSecret, ok := secretProviderClassNameMap[azKeyVaultName]
			if !ok {
				continue
			}
			classObjectsMap := getReplicaNameToSecretProviderClassObjectsMap(secretIdMap)
			createSecretProviderClassPodStatusesForAzureKeyVault(secretProviderClient, namespace, classObjectsMap, secretProviderClassAndSecret.className)
		}
	}
}

func createSecretProviderClassPodStatusesForAzureKeyVault(secretProviderClient secretProviderClient.Interface, namespace string, secretProviderClassObjectsMap map[string][]secretsstorev1.SecretProviderClassObject, secretProviderClassName string) {
	//secretProviderClassObjects map[replicaName]SecretProviderClassObject
	for replicaName, secretProviderClassObjects := range secretProviderClassObjectsMap {
		_, err := secretProviderClient.SecretsstoreV1().SecretProviderClassPodStatuses(namespace).Create(context.Background(),
			&secretsstorev1.SecretProviderClassPodStatus{
				ObjectMeta: metav1.ObjectMeta{Name: utils.RandString(10)}, //Name is not important
				Status: secretsstorev1.SecretProviderClassPodStatusStatus{
					PodName:                 replicaName,
					SecretProviderClassName: secretProviderClassName,
					Objects:                 secretProviderClassObjects, //Secret id/version pairs
				},
			}, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
	}
}

func getReplicaNameToSecretProviderClassObjectsMap(secretIdMap map[string]map[string]map[string]bool) map[string][]secretsstorev1.SecretProviderClassObject {
	//secretIdMap: map[secretId]map[version]map[replicaName]bool
	objectsMap := make(map[string][]secretsstorev1.SecretProviderClassObject) //map[replicaName]SecretProviderClassObject
	for secretId, versionReplicaNameMap := range secretIdMap {
		for version, replicaNameMap := range versionReplicaNameMap {
			for replicaName := range replicaNameMap {
				if _, ok := objectsMap[replicaName]; !ok {
					objectsMap[replicaName] = []secretsstorev1.SecretProviderClassObject{}
				}
				objectsMap[replicaName] = append(objectsMap[replicaName], secretsstorev1.SecretProviderClassObject{
					ID:      secretId,
					Version: version,
				})
			}
		}
	}
	return objectsMap
}
