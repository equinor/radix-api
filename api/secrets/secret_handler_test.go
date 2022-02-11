package secrets

import (
	"context"
	"fmt"
	"testing"

	deployMock "github.com/equinor/radix-api/api/deployments/mock"
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	secretModels "github.com/equinor/radix-api/api/secrets/models"
	"github.com/equinor/radix-api/api/secrets/suffix"
	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
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
}

type getSecretScenario struct {
	name            string
	components      []v1.RadixDeployComponent
	jobs            []v1.RadixDeployJobComponent
	externalAliases []v1.ExternalAlias
	volumeMounts    []v1.RadixVolumeMount
	existingSecrets []secretDescription
	expectedError   bool
	expectedSecrets []secretModels.Secret
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
					Status:      "Pending",
				},
				{
					Name:        "SECRET_J1",
					DisplayName: "SECRET_J1",
					Type:        secretModels.SecretTypeGeneric,
					Resource:    "",
					Component:   jobName1,
					Status:      "Pending",
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
					Status:      "Consistent",
				},
				{
					Name:        "SECRET_J1",
					DisplayName: "SECRET_J1",
					Type:        secretModels.SecretTypeGeneric,
					Resource:    "",
					Component:   jobName1,
					Status:      "Consistent",
				},
			},
		},
		{
			name:       "External alias secrets with no secrets",
			components: []v1.RadixDeployComponent{{Name: componentName1}},
			externalAliases: []v1.ExternalAlias{{
				Alias:       "someExternalAlias",
				Environment: anyEnvironment,
				Component:   componentName1,
			},
			},
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "someExternalAlias-key",
					DisplayName: "Key",
					Type:        secretModels.SecretTypeClientCert,
					Resource:    "someExternalAlias",
					Component:   componentName1,
					Status:      "Pending",
				},
				{
					Name:        "someExternalAlias-cert",
					DisplayName: "Certificate",
					Type:        secretModels.SecretTypeClientCert,
					Resource:    "someExternalAlias",
					Component:   componentName1,
					Status:      "Pending",
				},
			},
		},
		{
			name:       "External alias secrets with existing secrets",
			components: []v1.RadixDeployComponent{{Name: componentName1}},
			externalAliases: []v1.ExternalAlias{{
				Alias:       "someExternalAlias",
				Environment: anyEnvironment,
				Component:   componentName1,
			},
			},
			existingSecrets: []secretDescription{
				{
					secretName: "someExternalAlias",
					secretData: map[string][]byte{
						"tls.cer": []byte("current tls cert"),
						"tls.key": []byte("current tls key"),
					},
				},
			},
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "someExternalAlias-key",
					DisplayName: "Key",
					Type:        secretModels.SecretTypeClientCert,
					Resource:    "someExternalAlias",
					Component:   componentName1,
					Status:      "Consistent",
				},
				{
					Name:        "someExternalAlias-cert",
					DisplayName: "Certificate",
					Type:        secretModels.SecretTypeClientCert,
					Resource:    "someExternalAlias",
					Component:   componentName1,
					Status:      "Consistent",
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
					Status:      "Pending",
				},
				{
					Name:        "component1-volume1-csiazurecreds-accountname",
					DisplayName: "Account Name",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume1",
					Component:   componentName1,
					Status:      "Pending",
				},
				{
					Name:        "job1-volume2-csiazurecreds-accountkey",
					DisplayName: "Account Key",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume2",
					Component:   jobName1,
					Status:      "Pending",
				},
				{
					Name:        "job1-volume2-csiazurecreds-accountname",
					DisplayName: "Account Name",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume2",
					Component:   jobName1,
					Status:      "Pending",
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
					Status:      "Consistent",
				},
				{
					Name:        "component1-volume1-csiazurecreds-accountname",
					DisplayName: "Account Name",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume1",
					Component:   componentName1,
					Status:      "Consistent",
				},
				{
					Name:        "job1-volume2-csiazurecreds-accountkey",
					DisplayName: "Account Key",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume2",
					Component:   jobName1,
					Status:      "Consistent",
				},
				{
					Name:        "job1-volume2-csiazurecreds-accountname",
					DisplayName: "Account Name",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume2",
					Component:   jobName1,
					Status:      "Consistent",
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
					Name:        "component1-keyVault1-csiazkvcreds-azkv-clientid",
					DisplayName: "Client ID",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
					Resource:    "keyVault1",
					Component:   componentName1,
					Status:      "Pending",
				},
				{
					Name:        "component1-keyVault1-csiazkvcreds-azkv-clientsecret",
					DisplayName: "Client Secret",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
					Resource:    "keyVault1",
					Component:   componentName1,
					Status:      "Pending",
				},
				{
					Name:        "SECRET_REF1",
					DisplayName: "secret 'secret1'",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultItem,
					Resource:    "keyVault1",
					Component:   componentName1,
					Status:      "External",
				},
				{
					Name:        "job1-keyVault2-csiazkvcreds-azkv-clientid",
					DisplayName: "Client ID",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
					Resource:    "keyVault2",
					Component:   jobName1,
					Status:      "Pending",
				},
				{
					Name:        "job1-keyVault2-csiazkvcreds-azkv-clientsecret",
					DisplayName: "Client Secret",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
					Resource:    "keyVault2",
					Component:   jobName1,
					Status:      "Pending",
				},
				{
					Name:        "SECRET_REF2",
					DisplayName: "secret 'secret2'",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultItem,
					Resource:    "keyVault2",
					Component:   jobName1,
					Status:      "External",
				},
			},
		},
		{
			name: "Azure Key vault credential secrets when there are secret items with existing secrets",
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
			existingSecrets: []secretDescription{
				{
					secretName: "component1-keyVault1-csiazkvcreds",
					secretData: map[string][]byte{
						"clientid":     []byte("current client id1"),
						"clientsecret": []byte("current client secret1"),
					},
				},
				{
					secretName: "job1-keyVault2-csiazkvcreds",
					secretData: map[string][]byte{
						"clientid":     []byte("current client id2"),
						"clientsecret": []byte("current client secret2"),
					},
				},
			},
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-keyVault1-csiazkvcreds-azkv-clientid",
					DisplayName: "Client ID",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
					Resource:    "keyVault1",
					Component:   componentName1,
					Status:      "Consistent",
				},
				{
					Name:        "component1-keyVault1-csiazkvcreds-azkv-clientsecret",
					DisplayName: "Client Secret",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
					Resource:    "keyVault1",
					Component:   componentName1,
					Status:      "Consistent",
				},
				{
					Name:        "SECRET_REF1",
					DisplayName: "secret 'secret1'",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultItem,
					Resource:    "keyVault1",
					Component:   componentName1,
					Status:      "External",
				},
				{
					Name:        "job1-keyVault2-csiazkvcreds-azkv-clientid",
					DisplayName: "Client ID",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
					Resource:    "keyVault2",
					Component:   jobName1,
					Status:      "Consistent",
				},
				{
					Name:        "job1-keyVault2-csiazkvcreds-azkv-clientsecret",
					DisplayName: "Client Secret",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
					Resource:    "keyVault2",
					Component:   jobName1,
					Status:      "Consistent",
				},
				{
					Name:        "SECRET_REF2",
					DisplayName: "secret 'secret2'",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultItem,
					Resource:    "keyVault2",
					Component:   jobName1,
					Status:      "External",
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
					DisplayName: "Client certificate",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      "Pending",
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
					DisplayName: "Client certificate",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      "Consistent",
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
					DisplayName: "Client certificate",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      "Pending",
				},
			},
		},
	}

	for _, scenario := range scenarios {
		appName := anyAppName
		environment := anyEnvironment
		deploymentName := "deployment1"
		s.Run(fmt.Sprintf("test GetSecrets: %s", scenario.name), func() {
			secretHandler, deployHandler := s.prepareTestRun(ctrl, &scenario, appName, environment, deploymentName)

			deployHandler.EXPECT().GetDeploymentsForApplicationEnvironment(appName, environment, false).
				Return([]*deploymentModels.DeploymentSummary{{Name: deploymentName, Environment: environment}}, nil)

			secrets, err := secretHandler.GetSecrets(appName, environment)

			s.Nil(err)
			s.assertSecrets(&scenario, secrets)
		})

		s.Run(fmt.Sprintf("test GetSecretsForDeployment: %s", scenario.name), func() {
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
					DisplayName: "Client certificate",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      "Pending",
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
					DisplayName: "Client certificate",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      "Pending",
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
					DisplayName: "Client certificate",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      "Pending",
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
					DisplayName: "Client certificate",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      "Pending",
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
					DisplayName: "Client certificate",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      "Pending",
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

			secretHandler, deployHandler := s.prepareTestRun(ctrl, &commonScenario, appName, environment, deploymentName)

			deployHandler.EXPECT().GetDeploymentsForApplicationEnvironment(appName, environment, false).
				Return([]*deploymentModels.DeploymentSummary{{Name: deploymentName, Environment: environment}}, nil)

			secrets, err := secretHandler.GetSecrets(appName, environment)

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
			secretName:                  defaults.GetCsiAzureCredsSecretName(componentName1, volumeName1),
			secretDataKey:               defaults.CsiAzureCredsAccountNamePart,
			secretValue:                 "currentAccountName",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureCredsSecretName(componentName1, volumeName1) + defaults.CsiAzureCredsAccountNamePartSuffix,
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
			secretName:                  defaults.GetCsiAzureCredsSecretName(jobName1, volumeName1),
			secretDataKey:               defaults.CsiAzureCredsAccountNamePart,
			secretValue:                 "currentAccountName",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureCredsSecretName(jobName1, volumeName1) + defaults.CsiAzureCredsAccountNamePartSuffix,
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
			changingSecretName:          defaults.GetCsiAzureCredsSecretName(componentName1, volumeName1) + defaults.CsiAzureCredsAccountNamePartSuffix,
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
			changingSecretName:          defaults.GetCsiAzureCredsSecretName(jobName1, volumeName1) + defaults.CsiAzureCredsAccountNamePartSuffix,
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
			secretName:                  defaults.GetCsiAzureCredsSecretName(componentName1, volumeName1),
			secretDataKey:               defaults.CsiAzureCredsAccountKeyPart,
			secretValue:                 "currentAccountKey",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureCredsSecretName(componentName1, volumeName1) + defaults.CsiAzureCredsAccountKeyPartSuffix,
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
			secretName:                  defaults.GetCsiAzureCredsSecretName(jobName1, volumeName1),
			secretDataKey:               defaults.CsiAzureCredsAccountKeyPart,
			secretValue:                 "currentAccountKey",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureCredsSecretName(jobName1, volumeName1) + defaults.CsiAzureCredsAccountKeyPartSuffix,
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
			changingSecretName:          defaults.GetCsiAzureCredsSecretName(componentName1, volumeName1) + defaults.CsiAzureCredsAccountKeyPartSuffix,
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
			changingSecretName:          defaults.GetCsiAzureCredsSecretName(jobName1, volumeName1) + defaults.CsiAzureCredsAccountKeyPartSuffix,
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
			kubeClient, radixClient, _ := s.getUtils()
			secretHandler := SecretHandler{
				client:        kubeClient,
				radixclient:   radixClient,
				deployHandler: nil,
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
		s.True(exists, "Missed secret '%s'", expectedSecret.Name)
		s.Equal(expectedSecret.Type, secret.Type, "Not expected secret Type")
		s.Equal(expectedSecret.Component, secret.Component, "Not expected secret Component")
		s.Equal(expectedSecret.DisplayName, secret.DisplayName, "Not expected secret Component")
		s.Equal(expectedSecret.Status, secret.Status, "Not expected secret Status")
		s.Equal(expectedSecret.Resource, secret.Resource, "Not expected secret Resource")
	}
}

func (s *secretHandlerTestSuite) prepareTestRun(ctrl *gomock.Controller, scenario *getSecretScenario, appName, envName, deploymentName string) (SecretHandler, *deployMock.MockDeployHandler) {
	kubeClient, radixClient, _ := s.getUtils()
	deployHandler := deployMock.NewMockDeployHandler(ctrl)
	secretHandler := SecretHandler{
		client:        kubeClient,
		radixclient:   radixClient,
		deployHandler: deployHandler,
	}
	appAppNamespace := operatorUtils.GetAppNamespace(appName)
	ra := &v1.RadixApplication{
		ObjectMeta: metav1.ObjectMeta{Name: appName, Namespace: appAppNamespace},
		Spec: v1.RadixApplicationSpec{
			Environments:     []v1.Environment{{Name: envName}},
			DNSExternalAlias: scenario.externalAliases,
			Components:       getRadixComponents(scenario.components, envName),
			Jobs:             getRadixJobComponents(scenario.jobs, envName),
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
	appEnvNamespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	_, _ = radixClient.RadixV1().RadixDeployments(appEnvNamespace).Create(context.Background(), &radixDeployment, metav1.CreateOptions{})
	for _, secret := range scenario.existingSecrets {
		_, _ = kubeClient.CoreV1().Secrets(appEnvNamespace).Create(context.Background(), &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: secret.secretName, Namespace: appEnvNamespace},
			Data:       secret.secretData,
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

func (s *secretHandlerTestSuite) getUtils() (*kubefake.Clientset, *radixfake.Clientset, *secretproviderfake.Clientset) {
	return kubefake.NewSimpleClientset(), radixfake.NewSimpleClientset(), secretproviderfake.NewSimpleClientset()
}

func getVerificationTypePtr(verificationType v1.VerificationType) *v1.VerificationType {
	return &verificationType
}
