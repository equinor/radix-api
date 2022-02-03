package secrets

import (
	"context"
	deployMock "github.com/equinor/radix-api/api/deployments/mock"
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	eventsMock "github.com/equinor/radix-api/api/events/mock"
	secretModels "github.com/equinor/radix-api/api/secrets/models"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
	"testing"
)

type secretHandlerTestSuite struct {
	suite.Suite
}

func TestRunSecretHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(secretHandlerTestSuite))
}

func (s *secretHandlerTestSuite) TestSecretHandler_GetSecrets() {
	ctrl := gomock.NewController(s.T())
	defer ctrl.Finish()
	type testScenario struct {
		name            string
		appName         string
		envName         string
		deploymentName  string
		Components      []v1.RadixDeployComponent
		Jobs            []v1.RadixDeployJobComponent
		externalAliases []v1.ExternalAlias
		VolumeMounts    []v1.RadixVolumeMount
		want            []secretModels.Secret
		wantErr         assert.ErrorAssertionFunc
		initScenario    func(scenario testScenario)
		expectedError   bool
		expectedSecrets []secretModels.Secret
	}
	deploymentName1 := "deployment1"
	componentName1 := "component1"
	scenarios := []testScenario{
		{
			name:           "regular secrets",
			appName:        anyAppName,
			envName:        anyEnvironment,
			deploymentName: deploymentName1,
			Components: []v1.RadixDeployComponent{{
				Name: componentName1,
				Secrets: []string{
					"SECRET_C1",
				},
			}},
			Jobs: []v1.RadixDeployJobComponent{{
				Name: anyJobName,
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
					Component:   anyJobName,
					Status:      "Pending",
				},
			},
		},
		{
			name:           "External alias secrets",
			appName:        anyAppName,
			envName:        anyEnvironment,
			deploymentName: deploymentName1,
			Components:     []v1.RadixDeployComponent{{Name: componentName1}},
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
			Components: []v1.RadixDeployComponent{
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
			Jobs: []v1.RadixDeployJobComponent{
				{
					Name: componentName1,
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
					Name:        "component1-volume2-csiazurecreds-accountkey",
					DisplayName: "Account Key",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume2",
					Component:   componentName1,
					Status:      "Pending",
				},
				{
					Name:        "component1-volume2-csiazurecreds-accountname",
					DisplayName: "Account Name",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume2",
					Component:   componentName1,
					Status:      "Pending",
				},
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			kubeClient, radixClient, _ := s.getUtils()
			deployHandler := deployMock.NewMockDeployHandler(ctrl)
			eventHandler := eventsMock.NewMockEventHandler(ctrl)
			handler := SecretHandler{
				client:        kubeClient,
				radixclient:   radixClient,
				deployHandler: deployHandler,
				eventHandler:  eventHandler,
			}
			deployHandler.EXPECT().GetDeploymentsForApplicationEnvironment(scenario.appName, scenario.envName, false).
				Return([]*deploymentModels.DeploymentSummary{{Name: scenario.deploymentName}}, nil)
			deployHandler.EXPECT().GetDeploymentWithName(scenario.appName, deploymentName1).
				Return(&deploymentModels.Deployment{
					Name:       scenario.deploymentName,
					Components: getComponents(scenario.Components),
				}, nil)
			appAppNamespace := utils.GetAppNamespace(scenario.appName)
			ra := &v1.RadixApplication{
				ObjectMeta: metav1.ObjectMeta{Name: scenario.appName, Namespace: appAppNamespace},
				Spec:       v1.RadixApplicationSpec{DNSExternalAlias: scenario.externalAliases},
			}
			radixClient.RadixV1().RadixApplications(appAppNamespace).Create(context.Background(), ra, metav1.CreateOptions{})
			radixDeployment := v1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: scenario.deploymentName},
				Spec: v1.RadixDeploymentSpec{
					Components: scenario.Components,
					Jobs:       scenario.Jobs,
				},
			}
			appEnvNamespace := utils.GetEnvironmentNamespace(scenario.appName, scenario.envName)
			radixClient.RadixV1().RadixDeployments(appEnvNamespace).Create(context.Background(), &radixDeployment, metav1.CreateOptions{})

			secrets, err := handler.GetSecrets(scenario.appName, scenario.envName)

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
		})
	}
}

func getSecretMap(secrets []secretModels.Secret) map[string]secretModels.Secret {
	secretMap := make(map[string]secretModels.Secret, len(secrets))
	for _, secret := range secrets {
		secret := secret
		secretMap[secret.Name] = secret
	}
	return secretMap
}

func getComponents(radixDeployComponents []v1.RadixDeployComponent) []*deploymentModels.Component {
	var deploymentComponents []*deploymentModels.Component
	for _, radixDeployComponent := range radixDeployComponents {
		deploymentComponents = append(deploymentComponents, &deploymentModels.Component{
			Name: radixDeployComponent.Name,
		})
	}
	return deploymentComponents
}

func (s *secretHandlerTestSuite) getUtils() (*kubefake.Clientset, *radixfake.Clientset, *secretproviderfake.Clientset) {
	return kubefake.NewSimpleClientset(), radixfake.NewSimpleClientset(), secretproviderfake.NewSimpleClientset()
}
