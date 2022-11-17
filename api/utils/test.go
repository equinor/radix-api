package utils

import (
	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretsstorevclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	clusterName       = "AnyClusterName"
	containerRegistry = "any.container.registry"
	egressIps         = "0.0.0.0"
)

func SetupTest() (*commontest.Utils, kubernetes.Interface, radixclient.Interface, prometheusclient.Interface, secretsstorevclient.Interface) {
	kubeClient := kubefake.NewSimpleClientset()
	radixClient := fake.NewSimpleClientset()
	prometheusClient := prometheusfake.NewSimpleClientset()
	secretProviderClient := secretproviderfake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeClient, radixClient, secretProviderClient)
	commonTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry, egressIps)

	return &commonTestUtils, kubeClient, radixClient, prometheusClient, secretProviderClient
}

// ApplyRegistrationWithSync syncs based on registration builder
func ApplyRegistrationWithSync(client kubernetes.Interface, radixclient radixclient.Interface, commonTestUtils *commontest.Utils, registrationBuilder operatorutils.RegistrationBuilder) {
	kubeUtils, _ := kube.New(client, radixclient, nil)
	commonTestUtils.ApplyRegistration(registrationBuilder)

	registration, _ := application.NewApplication(client, kubeUtils, radixclient, registrationBuilder.BuildRR())
	registration.OnSync()
}

// ApplyApplicationWithSync syncs based on application builder, and default builder for registration.
func ApplyApplicationWithSync(client kubernetes.Interface, radixclient radixclient.Interface, commonTestUtils *commontest.Utils, applicationBuilder operatorutils.ApplicationBuilder) {
	registrationBuilder := applicationBuilder.GetRegistrationBuilder()

	ApplyRegistrationWithSync(client, radixclient, commonTestUtils, registrationBuilder)

	kubeUtils, _ := kube.New(client, radixclient, nil)
	commonTestUtils.ApplyApplication(applicationBuilder)

	applicationconfig, _ := applicationconfig.NewApplicationConfig(client, kubeUtils, radixclient, registrationBuilder.BuildRR(), applicationBuilder.BuildRA())
	applicationconfig.OnSync()
}

// ApplyDeploymentWithSync syncs based on deployment builder, and default builders for application and registration.
func ApplyDeploymentWithSync(client kubernetes.Interface, radixclient radixclient.Interface, promclient prometheusclient.Interface, commonTestUtils *commontest.Utils, secretproviderclient secretsstorevclient.Interface, deploymentBuilder operatorutils.DeploymentBuilder) {
	applicationBuilder := deploymentBuilder.GetApplicationBuilder()
	registrationBuilder := applicationBuilder.GetRegistrationBuilder()

	ApplyApplicationWithSync(client, radixclient, commonTestUtils, applicationBuilder)

	kubeUtils, _ := kube.New(client, radixclient, secretproviderclient)
	rd, _ := commonTestUtils.ApplyDeployment(deploymentBuilder)
	deployment := deployment.NewDeployment(client, kubeUtils, radixclient, promclient, registrationBuilder.BuildRR(), rd, "123456", 443, []deployment.IngressAnnotationProvider{}, []deployment.AuxiliaryResourceManager{})
	_ = deployment.OnSync()
}
