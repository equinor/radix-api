package utils

import (
	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
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
	clusterName    = "AnyClusterName"
	egressIps      = "0.0.0.0"
	subscriptionId = "bd9f9eaa-2703-47c6-b5e0-faf4e058df73"
)

func SetupTest() (*commontest.Utils, kubernetes.Interface, radixclient.Interface, prometheusclient.Interface, secretsstorevclient.Interface) {
	kubeClient := kubefake.NewSimpleClientset()
	radixClient := fake.NewSimpleClientset()
	prometheusClient := prometheusfake.NewSimpleClientset()
	secretProviderClient := secretproviderfake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeClient, radixClient, secretProviderClient)
	commonTestUtils.CreateClusterPrerequisites(clusterName, egressIps, subscriptionId)

	return &commonTestUtils, kubeClient, radixClient, prometheusClient, secretProviderClient
}

// ApplyRegistrationWithSync syncs based on registration builder
func ApplyRegistrationWithSync(client kubernetes.Interface, radixclient radixclient.Interface, commonTestUtils *commontest.Utils, registrationBuilder operatorutils.RegistrationBuilder) {
	kubeUtils, _ := kube.New(client, radixclient, nil)
	_, err := commonTestUtils.ApplyRegistration(registrationBuilder)
	if err != nil {
		panic(err)
	}

	registration, _ := application.NewApplication(client, kubeUtils, radixclient, registrationBuilder.BuildRR())
	err = registration.OnSync()
	if err != nil {
		panic(err)
	}
}

// ApplyApplicationWithSync syncs based on application builder, and default builder for registration.
func ApplyApplicationWithSync(client kubernetes.Interface, radixclient radixclient.Interface, commonTestUtils *commontest.Utils, applicationBuilder operatorutils.ApplicationBuilder) {
	registrationBuilder := applicationBuilder.GetRegistrationBuilder()

	ApplyRegistrationWithSync(client, radixclient, commonTestUtils, registrationBuilder)

	kubeUtils, _ := kube.New(client, radixclient, nil)
	_, err := commonTestUtils.ApplyApplication(applicationBuilder)
	if err != nil {
		panic(err)
	}

	applicationConfig := applicationconfig.NewApplicationConfig(client, kubeUtils, radixclient, registrationBuilder.BuildRR(), applicationBuilder.BuildRA(), &dnsalias.DNSConfig{DNSZone: "dev.radix.equinor.com"})
	err = applicationConfig.OnSync()
	if err != nil {
		panic(err)
	}
}

// ApplyDeploymentWithSync syncs based on deployment builder, and default builders for application and registration.
func ApplyDeploymentWithSync(client kubernetes.Interface, radixclient radixclient.Interface, prometheusClient prometheusclient.Interface, commonTestUtils *commontest.Utils, secretproviderclient secretsstorevclient.Interface, deploymentBuilder operatorutils.DeploymentBuilder) {
	applicationBuilder := deploymentBuilder.GetApplicationBuilder()
	registrationBuilder := applicationBuilder.GetRegistrationBuilder()

	ApplyApplicationWithSync(client, radixclient, commonTestUtils, applicationBuilder)

	kubeUtils, _ := kube.New(client, radixclient, secretproviderclient)
	rd, _ := commonTestUtils.ApplyDeployment(deploymentBuilder)
	depl := deployment.NewDeploymentSyncer(client, kubeUtils, radixclient, prometheusClient, registrationBuilder.BuildRR(), rd, "123456", 443, 10, []ingress.AnnotationProvider{}, []deployment.AuxiliaryResourceManager{})
	err := depl.OnSync()
	if err != nil {
		panic(err)
	}
}
