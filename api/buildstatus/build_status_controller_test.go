package buildstatus

import (
	"errors"
	"io"
	"os"
	"testing"
	"time"

	authnmock "github.com/equinor/radix-api/api/utils/token/mock"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"

	certclientfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	controllertest "github.com/equinor/radix-api/api/test"
	"github.com/equinor/radix-api/api/test/mock"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

const (
	clusterName    = "AnyClusterName"
	egressIps      = "0.0.0.0"
	subscriptionId = "12347718-c8f8-4995-bfbb-02655ff1f89c"
)

func setupTest(t *testing.T) (*commontest.Utils, *kubefake.Clientset, *radixfake.Clientset, *kedafake.Clientset, *secretproviderfake.Clientset, *certclientfake.Clientset) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := radixfake.NewSimpleClientset()
	kedaClient := kedafake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	certClient := certclientfake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient)
	err := commonTestUtils.CreateClusterPrerequisites(clusterName, egressIps, subscriptionId)
	require.NoError(t, err)
	_ = os.Setenv(defaults.ActiveClusternameEnvironmentVariable, clusterName)

	return &commonTestUtils, kubeclient, radixclient, kedaClient, secretproviderclient, certClient
}

func TestGetBuildStatus(t *testing.T) {
	commonTestUtils, kubeclient, radixclient, kedaClient, secretproviderclient, certClient := setupTest(t)

	jobStartReferenceTime := time.Date(2020, 1, 10, 0, 0, 0, 0, time.UTC)
	_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration())
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(builders.ARadixApplication().WithAppName("my-app").WithEnvironment("test", "master"))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyJob(
		builders.NewJobBuilder().WithCreated(jobStartReferenceTime).
			WithBranch("master").WithJobName("bd-test-1").WithPipelineType(v1.BuildDeploy).WithAppName("my-app").
			WithStatus(builders.NewJobStatusBuilder().WithCondition(v1.JobSucceeded).WithStarted(jobStartReferenceTime).WithEnded(jobStartReferenceTime.Add(1 * time.Hour))),
	)
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyJob(
		builders.NewJobBuilder().WithCreated(jobStartReferenceTime.Add(1 * time.Hour)).
			WithBranch("master").WithJobName("bd-test-2").WithPipelineType(v1.BuildDeploy).WithAppName("my-app").
			WithStatus(builders.NewJobStatusBuilder().WithCondition(v1.JobRunning).WithStarted(jobStartReferenceTime.Add(2 * time.Hour))),
	)
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyJob(
		builders.NewJobBuilder().WithCreated(jobStartReferenceTime).
			WithBranch("master").WithJobName("d-test-1").WithPipelineType(v1.Deploy).WithAppName("my-app").
			WithStatus(builders.NewJobStatusBuilder().WithCondition(v1.JobFailed).WithStarted(jobStartReferenceTime).WithEnded(jobStartReferenceTime.Add(1 * time.Hour))),
	)
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyJob(
		builders.NewJobBuilder().WithCreated(jobStartReferenceTime.Add(1 * time.Hour)).
			WithBranch("master").WithJobName("d-test-2").WithPipelineType(v1.Deploy).WithAppName("my-app").
			WithStatus(builders.NewJobStatusBuilder().WithCondition(v1.JobSucceeded).WithStarted(jobStartReferenceTime.Add(2 * time.Hour))),
	)
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyJob(
		builders.NewJobBuilder().WithCreated(jobStartReferenceTime).
			WithBranch("master").WithJobName("p-test-1").WithPipelineType(v1.Promote).WithAppName("my-app").
			WithStatus(builders.NewJobStatusBuilder().WithCondition(v1.JobStopped).WithStarted(jobStartReferenceTime).WithEnded(jobStartReferenceTime.Add(1 * time.Hour))),
	)
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyJob(
		builders.NewJobBuilder().WithCreated(jobStartReferenceTime.Add(1 * time.Hour)).
			WithBranch("master").WithJobName("p-test-2").WithPipelineType(v1.Promote).WithAppName("my-app").
			WithStatus(builders.NewJobStatusBuilder().WithCondition(v1.JobFailed).WithStarted(jobStartReferenceTime.Add(2 * time.Hour))),
	)
	require.NoError(t, err)

	t.Run("return success status and badge data", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		fakeBuildStatus := mock.NewMockPipelineBadge(ctrl)
		expected := []byte("badge")

		fakeBuildStatus.EXPECT().
			GetBadge(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(expected, nil).
			Times(1)

		mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
		mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
		controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient, certClient, mockValidator, NewBuildStatusController(fakeBuildStatus))
		responseChannel := controllerTestUtils.ExecuteUnAuthorizedRequest("GET", "/api/v1/applications/my-app/environments/test/buildstatus")
		response := <-responseChannel

		assert.Equal(t, response.Result().StatusCode, 200)
		actual, _ := io.ReadAll(response.Body)
		assert.Equal(t, string(expected), string(actual))

	})

	t.Run("build-deploy in master - JobRunning", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		fakeBuildStatus := mock.NewMockPipelineBadge(ctrl)

		var actualCondition v1.RadixJobCondition
		var actualPipeline v1.RadixPipelineType

		fakeBuildStatus.EXPECT().
			GetBadge(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(c v1.RadixJobCondition, p v1.RadixPipelineType) ([]byte, error) {
				actualCondition = c
				actualPipeline = p
				return nil, nil
			})

		mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
		mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
		controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient, certClient, mockValidator, NewBuildStatusController(fakeBuildStatus))

		responseChannel := controllerTestUtils.ExecuteUnAuthorizedRequest("GET", "/api/v1/applications/my-app/environments/test/buildstatus")
		response := <-responseChannel

		assert.Equal(t, response.Result().StatusCode, 200)
		assert.Equal(t, v1.JobRunning, actualCondition)
		assert.Equal(t, v1.BuildDeploy, actualPipeline)
	})

	t.Run("deploy in master - JobRunning", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		fakeBuildStatus := mock.NewMockPipelineBadge(ctrl)

		var actualCondition v1.RadixJobCondition
		var actualPipeline v1.RadixPipelineType

		fakeBuildStatus.EXPECT().
			GetBadge(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(c v1.RadixJobCondition, p v1.RadixPipelineType) ([]byte, error) {
				actualCondition = c
				actualPipeline = p
				return nil, nil
			})

		mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
		mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
		controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient, certClient, mockValidator, NewBuildStatusController(fakeBuildStatus))

		responseChannel := controllerTestUtils.ExecuteUnAuthorizedRequest("GET", "/api/v1/applications/my-app/environments/test/buildstatus?pipeline=deploy")
		response := <-responseChannel

		assert.Equal(t, response.Result().StatusCode, 200)
		assert.Equal(t, v1.JobSucceeded, actualCondition)
		assert.Equal(t, v1.Deploy, actualPipeline)
	})

	t.Run("promote in master - JobFailed", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		fakeBuildStatus := mock.NewMockPipelineBadge(ctrl)

		var actualCondition v1.RadixJobCondition
		var actualPipeline v1.RadixPipelineType

		fakeBuildStatus.EXPECT().
			GetBadge(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(c v1.RadixJobCondition, p v1.RadixPipelineType) ([]byte, error) {
				actualCondition = c
				actualPipeline = p
				return nil, nil
			})

		mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
		mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
		controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient, certClient, mockValidator, NewBuildStatusController(fakeBuildStatus))

		responseChannel := controllerTestUtils.ExecuteUnAuthorizedRequest("GET", "/api/v1/applications/my-app/environments/test/buildstatus?pipeline=promote")
		response := <-responseChannel

		assert.Equal(t, response.Result().StatusCode, 200)
		assert.Equal(t, v1.JobFailed, actualCondition)
		assert.Equal(t, v1.Promote, actualPipeline)
	})

	t.Run("return status 500", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		fakeBuildStatus := mock.NewMockPipelineBadge(ctrl)

		fakeBuildStatus.EXPECT().
			GetBadge(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, errors.New("error")).
			Times(1)

		mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
		mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
		controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient, certClient, mockValidator, NewBuildStatusController(fakeBuildStatus))

		responseChannel := controllerTestUtils.ExecuteUnAuthorizedRequest("GET", "/api/v1/applications/my-app/environments/test/buildstatus")
		response := <-responseChannel

		assert.Equal(t, response.Result().StatusCode, 500)
	})
}
