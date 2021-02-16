package buildstatus

import (
	"os"
	"testing"

	controllertest "github.com/equinor/radix-api/api/test"
	"github.com/equinor/radix-api/api/test/mock"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	"gotest.tools/assert"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

const (
	clusterName       = "AnyClusterName"
	containerRegistry = "any.container.registry"
	dnsZone           = "dev.radix.equinor.com"
	appAliasDNSZone   = "app.dev.radix.equinor.com"
)

func setupTest() (*commontest.Utils, *kubefake.Clientset, *fake.Clientset) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := fake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient)
	commonTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, clusterName)

	return &commonTestUtils, kubeclient, radixclient
}

func TestGetBuildStatus(t *testing.T) {
	commonTestUtils, kubeclient, radixclient := setupTest()

	// Mock setup
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fakeBuildStatus := mock.NewMockStatus(ctrl)

	sampleResponse := []byte("This is a test")

	fakeBuildStatus.EXPECT().WriteSvg(gomock.Any()).Return(&sampleResponse).AnyTimes()

	commonTestUtils.ApplyRegistration(builders.ARadixRegistration())
	commonTestUtils.ApplyApplication(builders.ARadixApplication().WithAppName("my-app").WithEnvironment("test", "master"))
	commonTestUtils.ApplyJob(builders.ARadixBuildDeployJob().
		WithAppName("my-app").
		WithStatus(builders.ACompletedJobStatus()))

	controllerTestUtils := controllertest.NewTestUtils(
		kubeclient,
		radixclient,
		NewBuildStatusController(fakeBuildStatus),
	)

	responseChannel := controllerTestUtils.ExecuteUnAuthorizedRequest("GET", "/api/v1/applications/my-app/environments/test/buildstatus")
	response := <-responseChannel

	assert.Equal(t, response.Result().StatusCode, 200)
}
