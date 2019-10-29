package utils

import (
	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

func SyncRadixOperatorControllers(client kubernetes.Interface, radixclient radixclient.Interface, commonTestUtils *commontest.Utils, deploymentBuilder builders.DeploymentBuilder) {
	rd, _ := commonTestUtils.ApplyDeployment(deploymentBuilder)
	applicationBuilder := deploymentBuilder.GetApplicationBuilder()
	registrationBuilder := applicationBuilder.GetRegistrationBuilder()

	kubeUtils, _ := kube.New(client, radixclient)
	registration, _ := application.NewApplication(client, kubeUtils, radixclient, registrationBuilder.BuildRR())
	applicationconfig, _ := applicationconfig.NewApplicationConfig(client, kubeUtils, radixclient, registrationBuilder.BuildRR(), applicationBuilder.BuildRA())
	deployment, _ := deployment.NewDeployment(client, kubeUtils, radixclient, nil, registrationBuilder.BuildRR(), rd)

	registration.OnSync()
	applicationconfig.OnSync()
	deployment.OnSync()
}
