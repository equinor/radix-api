package secrets

import (
	"context"
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"testing"

	deployMock "github.com/equinor/radix-api/api/deployments/mock"
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	secretModels "github.com/equinor/radix-api/api/secrets/models"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
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

type getSecretScenario struct {
	name            string
	appName         string
	envName         string
	deploymentName  string
	components      []v1.RadixDeployComponent
	jobs            []v1.RadixDeployJobComponent
	externalAliases []v1.ExternalAlias
	volumeMounts    []v1.RadixVolumeMount
	expectedError   bool
	expectedSecrets []secretModels.Secret
}

type changeSecretScenario struct {
	name                        string
	appName                     string
	envName                     string
	deploymentName              string
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
	deploymentName1 := "deployment1"
	componentName1 := "component1"
	jobName1 := "job1"
	scenarios := []getSecretScenario{
		{
			name:           "regular secrets",
			appName:        anyAppName,
			envName:        anyEnvironment,
			deploymentName: deploymentName1,
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
			expectedError: false,
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
			name:           "External alias secrets",
			appName:        anyAppName,
			envName:        anyEnvironment,
			deploymentName: deploymentName1,
			components:     []v1.RadixDeployComponent{{Name: componentName1}},
			externalAliases: []v1.ExternalAlias{{
				Alias:       "someExternalAlias",
				Environment: anyEnvironment,
				Component:   componentName1,
			},
			},
			expectedError: false,
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
			name:           "Azure Blob volumes credential secrets",
			appName:        anyAppName,
			envName:        anyEnvironment,
			deploymentName: deploymentName1,
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
			expectedError: false,
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
			name:           "No Azure Key vault credential secrets when there is no Azure key vault SecretRefs",
			appName:        anyAppName,
			envName:        anyEnvironment,
			deploymentName: deploymentName1,
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
			expectedError:   false,
			expectedSecrets: nil,
		},
		{
			name:           "Azure Key vault credential secrets when there are secret items",
			appName:        anyAppName,
			envName:        anyEnvironment,
			deploymentName: deploymentName1,
			components: []v1.RadixDeployComponent{
				{
					Name: componentName1,
					SecretRefs: v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{
						{
							Name: "keyVault1",
							Items: []v1.RadixAzureKeyVaultItem{
								v1.RadixAzureKeyVaultItem{
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
								v1.RadixAzureKeyVaultItem{
									Name:   "secret2",
									EnvVar: "SECRET_REF2",
								},
							}},
					}},
				},
			},
			expectedError: false,
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
		//TODO{
		//	name:           "Secrets from Authentication",
		//	appName:        anyAppName,
		//	envName:        anyEnvironment,
		//	deploymentName: deploymentName1,
		//	Components: []v1.RadixDeployComponent{{Name:       componentName1}},
	}

	for _, scenario := range scenarios {
		s.Run(fmt.Sprintf("test GetSecrets: %s", scenario.name), func() {
			secretHandler, deployHandler := s.prepareTestRun(ctrl, &scenario)

			deployHandler.EXPECT().GetDeploymentsForApplicationEnvironment(scenario.appName, scenario.envName, false).
				Return([]*deploymentModels.DeploymentSummary{{Name: scenario.deploymentName, Environment: scenario.envName}}, nil)

			secrets, err := secretHandler.GetSecrets(scenario.appName, scenario.envName)

			s.assertSecrets(&scenario, err, secrets)
		})

		s.Run(fmt.Sprintf("test GetSecretsForDeployment: %s", scenario.name), func() {
			secretHandler, _ := s.prepareTestRun(ctrl, &scenario)

			secrets, err := secretHandler.GetSecretsForDeployment(scenario.appName, scenario.envName, scenario.deploymentName)

			s.assertSecrets(&scenario, err, secrets)
		})
	}
}

func (s *secretHandlerTestSuite) TestSecretHandler_ChangeSecrets() {
	ctrl := gomock.NewController(s.T())
	defer ctrl.Finish()
	deploymentName1 := "deployment1"
	componentName1 := "component1"
	jobName1 := "job1"
	volumeName1 := "volume1"
	azureKeyVaultName1 := "azureKeyVault1"
	//goland:noinspection ALL
	scenarios := []changeSecretScenario{
		{
			name:           "Change regular secret in the component",
			appName:        anyAppName,
			envName:        anyEnvironment,
			deploymentName: deploymentName1,
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
			name:           "Change regular secret in the component",
			appName:        anyAppName,
			envName:        anyEnvironment,
			deploymentName: deploymentName1,
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
			name:           "Change regular secret in the job",
			appName:        anyAppName,
			envName:        anyEnvironment,
			deploymentName: deploymentName1,
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
			name:           "Change External DNS cert in the component",
			appName:        anyAppName,
			envName:        anyEnvironment,
			deploymentName: deploymentName1,
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
			name:           "Change External DNS cert in the job",
			appName:        anyAppName,
			envName:        anyEnvironment,
			deploymentName: deploymentName1,
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
			name:           "Change External DNS key in the component",
			appName:        anyAppName,
			envName:        anyEnvironment,
			deploymentName: deploymentName1,
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
			name:           "Change External DNS key in the job",
			appName:        anyAppName,
			envName:        anyEnvironment,
			deploymentName: deploymentName1,
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
			name:           "Change CSI Azure Blob volume account name in the component",
			appName:        anyAppName,
			envName:        anyEnvironment,
			deploymentName: deploymentName1,
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
			name:           "Change CSI Azure Blob volume account name in the job",
			appName:        anyAppName,
			envName:        anyEnvironment,
			deploymentName: deploymentName1,
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
			name:           "Change CSI Azure Blob volume account key in the component",
			appName:        anyAppName,
			envName:        anyEnvironment,
			deploymentName: deploymentName1,
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
			name:           "Change CSI Azure Blob volume account key in the job",
			appName:        anyAppName,
			envName:        anyEnvironment,
			deploymentName: deploymentName1,
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
			name:           "Change CSI Azure Key vault client ID in the component",
			appName:        anyAppName,
			envName:        anyEnvironment,
			deploymentName: deploymentName1,
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
			name:           "Change CSI Azure Key vault client ID in the job",
			appName:        anyAppName,
			envName:        anyEnvironment,
			deploymentName: deploymentName1,
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
			name:           "Change CSI Azure Key vault client secret in the component",
			appName:        anyAppName,
			envName:        anyEnvironment,
			deploymentName: deploymentName1,
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
			name:           "Change CSI Azure Key vault client secret in the job",
			appName:        anyAppName,
			envName:        anyEnvironment,
			deploymentName: deploymentName1,
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
	}

	for _, scenario := range scenarios {
		s.Run(fmt.Sprintf("test GetSecrets: %s", scenario.name), func() {
			kubeClient, radixClient, _ := s.getUtils()
			secretHandler := SecretHandler{
				client:        kubeClient,
				radixclient:   radixClient,
				deployHandler: nil,
			}
			appEnvNamespace := utils.GetEnvironmentNamespace(scenario.appName, scenario.envName)
			if scenario.secretExists {
				kubeClient.CoreV1().Secrets(appEnvNamespace).Create(context.Background(), &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: scenario.secretName, Namespace: appEnvNamespace},
					Data:       map[string][]byte{scenario.secretDataKey: []byte(scenario.secretValue)},
				}, metav1.CreateOptions{})
			}

			err := secretHandler.ChangeComponentSecret(scenario.appName, scenario.envName, scenario.changingSecretComponentName, scenario.changingSecretName, scenario.changingSecretParams)

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

func (s *secretHandlerTestSuite) assertSecrets(scenario *getSecretScenario, err error, secrets []secretModels.Secret) {
	s.Equal(scenario.expectedError, err != nil)
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

func (s *secretHandlerTestSuite) prepareTestRun(ctrl *gomock.Controller, scenario *getSecretScenario) (SecretHandler, *deployMock.MockDeployHandler) {
	kubeClient, radixClient, _ := s.getUtils()
	deployHandler := deployMock.NewMockDeployHandler(ctrl)
	secretHandler := SecretHandler{
		client:        kubeClient,
		radixclient:   radixClient,
		deployHandler: deployHandler,
	}
	appAppNamespace := utils.GetAppNamespace(scenario.appName)
	ra := &v1.RadixApplication{
		ObjectMeta: metav1.ObjectMeta{Name: scenario.appName, Namespace: appAppNamespace},
		Spec: v1.RadixApplicationSpec{
			Environments:     []v1.Environment{{Name: scenario.envName}},
			DNSExternalAlias: scenario.externalAliases,
			Components:       getRadixComponents(scenario.components, scenario.envName),
			Jobs:             getRadixJobComponents(scenario.jobs, scenario.envName),
		},
	}
	radixClient.RadixV1().RadixApplications(appAppNamespace).Create(context.Background(), ra, metav1.CreateOptions{})
	radixDeployment := v1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: scenario.deploymentName},
		Spec: v1.RadixDeploymentSpec{
			Environment: scenario.envName,
			Components:  scenario.components,
			Jobs:        scenario.jobs,
		},
	}
	appEnvNamespace := utils.GetEnvironmentNamespace(scenario.appName, scenario.envName)
	radixClient.RadixV1().RadixDeployments(appEnvNamespace).Create(context.Background(), &radixDeployment, metav1.CreateOptions{})
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

func getComponents(radixDeployComponents []v1.RadixDeployComponent, radixDeployJobComponents []v1.RadixDeployJobComponent) []*deploymentModels.Component {
	var deploymentComponents []*deploymentModels.Component
	for _, radixDeployComponent := range radixDeployComponents {
		deploymentComponents = append(deploymentComponents, &deploymentModels.Component{
			Name:      radixDeployComponent.Name,
			Type:      string(v1.RadixComponentTypeComponent),
			Variables: radixDeployComponent.GetEnvironmentVariables(),
			Secrets:   radixDeployComponent.Secrets,
		})
	}
	for _, radixDeployJobComponent := range radixDeployJobComponents {
		deploymentComponents = append(deploymentComponents, &deploymentModels.Component{
			Name:      radixDeployJobComponent.Name,
			Type:      string(v1.RadixComponentTypeJobScheduler),
			Variables: radixDeployJobComponent.GetEnvironmentVariables(),
			Secrets:   radixDeployJobComponent.Secrets,
		})
	}
	return deploymentComponents
}

func (s *secretHandlerTestSuite) getUtils() (*kubefake.Clientset, *radixfake.Clientset, *secretproviderfake.Clientset) {
	return kubefake.NewSimpleClientset(), radixfake.NewSimpleClientset(), secretproviderfake.NewSimpleClientset()
}
