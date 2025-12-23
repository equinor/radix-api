package utils

import (
	"context"
	"testing"

	certfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretsstorevclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	clusterName    = "AnyClusterName"
	subscriptionId = "bd9f9eaa-2703-47c6-b5e0-faf4e058df73"
)

func SetupTest(t *testing.T) (*commontest.Utils, *kubefake.Clientset, *radixfake.Clientset, *kedafake.Clientset, *prometheusfake.Clientset, *secretproviderfake.Clientset, *certfake.Clientset) {
	kubeClient := kubefake.NewSimpleClientset() //nolint:staticcheck
	radixClient := radixfake.NewSimpleClientset()
	kedaClient := kedafake.NewSimpleClientset()
	prometheusClient := prometheusfake.NewSimpleClientset()
	secretProviderClient := secretproviderfake.NewSimpleClientset()
	certClient := certfake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeClient, radixClient, kedaClient, secretProviderClient)
	err := commonTestUtils.CreateClusterPrerequisites(clusterName, subscriptionId)
	require.NoError(t, err)
	return &commonTestUtils, kubeClient, radixClient, kedaClient, prometheusClient, secretProviderClient, certClient
}

// ApplyRegistrationWithSync syncs based on registration builder
func ApplyRegistrationWithSync(client kubernetes.Interface, radixclient radixclient.Interface, kedaClient kedav2.Interface, commonTestUtils *commontest.Utils, registrationBuilder operatorutils.RegistrationBuilder) error {
	kubeUtils, _ := kube.New(client, radixclient, kedaClient, nil)
	_, err := commonTestUtils.ApplyRegistration(registrationBuilder)
	if err != nil {
		return err
	}

	registration := application.NewApplication(client, kubeUtils, radixclient, registrationBuilder.BuildRR())
	return registration.OnSync(context.Background())
}

// ApplyApplicationWithSync syncs based on application builder, and default builder for registration.
func ApplyApplicationWithSync(client kubernetes.Interface, radixclient radixclient.Interface, kedaClient kedav2.Interface, commonTestUtils *commontest.Utils, applicationBuilder operatorutils.ApplicationBuilder) error {
	registrationBuilder := applicationBuilder.GetRegistrationBuilder()

	err := ApplyRegistrationWithSync(client, radixclient, kedaClient, commonTestUtils, registrationBuilder)
	if err != nil {
		return err
	}

	kubeUtils, _ := kube.New(client, radixclient, kedaClient, nil)
	_, err = commonTestUtils.ApplyApplication(applicationBuilder)
	if err != nil {
		panic(err)
	}
	_, err = commonTestUtils.ApplyApplication(applicationBuilder)
	if err != nil {
		return err
	}

	applicationConfig := applicationconfig.NewApplicationConfig(client, kubeUtils, radixclient, registrationBuilder.BuildRR(), applicationBuilder.BuildRA())
	return applicationConfig.OnSync(context.Background())
}

// ApplyDeploymentWithSync syncs based on deployment builder, and default builders for application and registration.
func ApplyDeploymentWithSync(client kubernetes.Interface, radixclient radixclient.Interface, kedaClient kedav2.Interface, prometheusClient prometheusclient.Interface, commonTestUtils *commontest.Utils, secretproviderclient secretsstorevclient.Interface, certClient *certfake.Clientset, deploymentBuilder operatorutils.DeploymentBuilder) error {
	applicationBuilder := deploymentBuilder.GetApplicationBuilder()
	registrationBuilder := applicationBuilder.GetRegistrationBuilder()

	err := ApplyApplicationWithSync(client, radixclient, kedaClient, commonTestUtils, applicationBuilder)
	if err != nil {
		return err
	}

	kubeUtils, _ := kube.New(client, radixclient, kedaClient, secretproviderclient)
	rd, _ := commonTestUtils.ApplyDeployment(context.Background(), deploymentBuilder)
	deploymentSyncer := deployment.NewDeploymentSyncer(client, kubeUtils, radixclient, prometheusClient, certClient, registrationBuilder.BuildRR(), rd, []ingress.AnnotationProvider{}, []deployment.AuxiliaryResourceManager{}, &config.Config{})
	return deploymentSyncer.OnSync(context.Background())
}
