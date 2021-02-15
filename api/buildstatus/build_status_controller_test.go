package buildstatus

import (
	"os"
	"testing"

	controllertest "github.com/equinor/radix-api/api/test"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"gotest.tools/assert"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

const (
	clusterName       = "AnyClusterName"
	containerRegistry = "any.container.registry"
	dnsZone           = "dev.radix.equinor.com"
	appAliasDNSZone   = "app.dev.radix.equinor.com"
)

func setupTest() (*commontest.Utils, *controllertest.Utils, *kubefake.Clientset, *fake.Clientset) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := fake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient)
	commonTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, clusterName)

	// controllerTestUtils is used for issuing HTTP request and processing responses
	controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, NewBuildStatusController())

	return &commonTestUtils, &controllerTestUtils, kubeclient, radixclient
}

func TestGetBuildStatus(t *testing.T) {
	commonTestUtils, _, kubeclient, radixclient := setupTest()

	commonTestUtils.ApplyRegistration(builders.ARadixRegistration())
	commonTestUtils.ApplyApplication(builders.ARadixApplication().WithAppName("my-app").WithEnvironment("test", "master"))
	commonTestUtils.ApplyJob(builders.ARadixBuildDeployJob().
		WithAppName("my-app").
		WithStatus(builders.ACompletedJobStatus()))

	controllerTestUtils := controllertest.NewTestUtils(
		kubeclient,
		radixclient,
		NewBuildStatusController(),
	)

	responseChannel := controllerTestUtils.ExecuteUnAuthorizedRequest("GET", "/api/v1/applications/my-app/buildstatus/test")
	response := <-responseChannel

	assert.Equal(t, response.Result().StatusCode, 200)
}
