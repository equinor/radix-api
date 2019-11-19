package utils

import (
	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

// ApplyApplicationWithSync syncs based on application builder, and default builder for registration.
func ApplyApplicationWithSync(client kubernetes.Interface, radixclient radixclient.Interface, commonTestUtils *commontest.Utils, applicationBuilder utils.ApplicationBuilder) {
	registrationBuilder := applicationBuilder.GetRegistrationBuilder()

	kubeUtils, _ := kube.New(client, radixclient)
	commonTestUtils.ApplyRegistration(registrationBuilder)
	commonTestUtils.ApplyApplication(applicationBuilder)

	registration, _ := application.NewApplication(client, kubeUtils, radixclient, registrationBuilder.BuildRR())
	applicationconfig, _ := applicationconfig.NewApplicationConfig(client, kubeUtils, radixclient, registrationBuilder.BuildRR(), applicationBuilder.BuildRA())

	registration.OnSync()
	applicationconfig.OnSync()
}

// ApplyDeploymentWithSync syncs based on deployment builder, and default builders for application and registration.
func ApplyDeploymentWithSync(client kubernetes.Interface, radixclient radixclient.Interface, commonTestUtils *commontest.Utils, deploymentBuilder builders.DeploymentBuilder) {
	applicationBuilder := deploymentBuilder.GetApplicationBuilder()
	registrationBuilder := applicationBuilder.GetRegistrationBuilder()

	ApplyApplicationWithSync(client, radixclient, commonTestUtils, applicationBuilder)

	kubeUtils, _ := kube.New(client, radixclient)
	rd, _ := commonTestUtils.ApplyDeployment(deploymentBuilder)
	deployment, _ := deployment.NewDeployment(client, kubeUtils, radixclient, nil, registrationBuilder.BuildRR(), rd)
	deployment.OnSync()
}
