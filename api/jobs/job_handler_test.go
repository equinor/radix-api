package jobs

import (
	"context"
	"testing"
	"time"

	deployMock "github.com/equinor/radix-api/api/deployments/mock"
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-api/models"
	radixmodels "github.com/equinor/radix-common/models"
	radixutils "github.com/equinor/radix-common/utils"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

type JobHandlerTestSuite struct {
	suite.Suite
	accounts    models.Accounts
	testUtils   *commontest.Utils
	kubeClient  kubernetes.Interface
	radixClient radixclient.Interface
}

func TestRunJobHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(JobHandlerTestSuite))
}

func (s *JobHandlerTestSuite) SetupTest() {
	s.testUtils, s.kubeClient, s.radixClient = s.getUtils()
	accounts := models.NewAccounts(s.kubeClient, s.radixClient, s.kubeClient, s.radixClient, "", radixmodels.Impersonation{})
	s.accounts = accounts
}

func (s *JobHandlerTestSuite) Test_GetApplicationJob() {
	jobName, appName, branch, commitId, pipeline, triggeredBy := "a_job", "an_app", "a_branch", "a_commitid", v1.BuildDeploy, "a_user"
	started, ended := metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)), metav1.NewTime(time.Date(2020, 1, 2, 0, 0, 0, 0, time.Local))
	rj := &v1.RadixJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: utils.GetAppNamespace(appName),
		},
		Spec: v1.RadixJobSpec{
			Build: v1.RadixBuildSpec{
				Branch:   branch,
				CommitID: commitId,
			},
			PipeLineType: pipeline,
			TriggeredBy:  triggeredBy,
		},
		Status: v1.RadixJobStatus{
			Started: &started,
			Ended:   &ended,
		},
	}
	s.radixClient.RadixV1().RadixJobs(rj.Namespace).Create(context.Background(), rj, metav1.CreateOptions{})

	deploymentName := "a_deployment"
	deploySummary := deploymentModels.DeploymentSummary{
		Name:         deploymentName,
		CreatedByJob: "any_job",
		Environment:  "any_env",
		ActiveFrom:   "any_from",
		ActiveTo:     "any_to",
	}

	comp1Name, comp1Type, comp1Image := "comp1", "type1", "image1"
	comp2Name, comp2Type, comp2Image := "comp2", "type2", "image2"
	deployment := deploymentModels.Deployment{
		Components: []*deploymentModels.Component{
			{Name: comp1Name, Type: comp1Type, Image: comp1Image},
			{Name: comp2Name, Type: comp2Type, Image: comp2Image},
		},
	}

	s.Run("radixjob does not exist", func() {
		//s.T().Parallel()
		ctrl := gomock.NewController(s.T())
		defer ctrl.Finish()
		dh := deployMock.NewMockDeployHandler(ctrl)
		h := Init(s.accounts, dh)
		actualJob, err := h.GetApplicationJob(appName, "missing_job")
		assert.True(s.T(), k8serrors.IsNotFound(err))
		assert.Nil(s.T(), actualJob)
	})

	s.Run("deployHandle.GetDeploymentsForJob return error", func() {
		//s.T().Parallel()
		ctrl := gomock.NewController(s.T())
		defer ctrl.Finish()

		dh := deployMock.NewMockDeployHandler(ctrl)
		dh.EXPECT().GetDeploymentsForJob(appName, jobName).Return(nil, assert.AnError).Times(1)
		dh.EXPECT().GetDeploymentWithName(gomock.Any(), gomock.Any()).Times(0)
		h := Init(s.accounts, dh)

		actualJob, actualErr := h.GetApplicationJob(appName, jobName)
		assert.Equal(s.T(), assert.AnError, actualErr)
		assert.Nil(s.T(), actualJob)
	})

	s.Run("empty deploymentSummary list should not call GetDeploymentWithName", func() {
		//s.T().Parallel()
		ctrl := gomock.NewController(s.T())
		defer ctrl.Finish()

		dh := deployMock.NewMockDeployHandler(ctrl)
		dh.EXPECT().GetDeploymentsForJob(appName, jobName).Return(nil, nil).Times(1)
		dh.EXPECT().GetDeploymentWithName(gomock.Any(), gomock.Any()).Times(0)
		h := Init(s.accounts, dh)

		actualJob, actualErr := h.GetApplicationJob(appName, jobName)
		assert.NoError(s.T(), actualErr)
		assert.NotNil(s.T(), actualJob)
	})

	s.Run("deployHandle.GetDeploymentWithName return error", func() {
		//s.T().Parallel()
		ctrl := gomock.NewController(s.T())
		defer ctrl.Finish()

		deployList := []*deploymentModels.DeploymentSummary{&deploySummary}
		dh := deployMock.NewMockDeployHandler(ctrl)
		dh.EXPECT().GetDeploymentsForJob(appName, jobName).Return(deployList, nil).Times(1)
		dh.EXPECT().GetDeploymentWithName(appName, deploymentName).Return(nil, assert.AnError).Times(1)
		h := Init(s.accounts, dh)

		actualJob, actualErr := h.GetApplicationJob(appName, jobName)
		assert.Equal(s.T(), assert.AnError, actualErr)
		assert.Nil(s.T(), actualJob)

	})

	s.Run("valid jobSummary", func() {
		//s.T().Parallel()
		ctrl := gomock.NewController(s.T())
		defer ctrl.Finish()

		deployList := []*deploymentModels.DeploymentSummary{&deploySummary}
		dh := deployMock.NewMockDeployHandler(ctrl)
		dh.EXPECT().GetDeploymentsForJob(appName, jobName).Return(deployList, nil).Times(1)
		dh.EXPECT().GetDeploymentWithName(appName, deploymentName).Return(&deployment, nil).Times(1)
		h := Init(s.accounts, dh)

		actualJob, actualErr := h.GetApplicationJob(appName, jobName)
		assert.NoError(s.T(), actualErr)
		assert.Equal(s.T(), jobName, actualJob.Name)
		assert.Equal(s.T(), branch, actualJob.Branch)
		assert.Equal(s.T(), commitId, actualJob.CommitID)
		assert.Equal(s.T(), triggeredBy, actualJob.TriggeredBy)
		assert.Equal(s.T(), radixutils.FormatTime(&started), actualJob.Started)
		assert.Equal(s.T(), radixutils.FormatTime(&ended), actualJob.Ended)
		assert.Equal(s.T(), string(pipeline), actualJob.Pipeline)
		// Test: Status, Created, Components, Deployments, Steps

	})

}

func (s *JobHandlerTestSuite) getUtils() (*commontest.Utils, kubernetes.Interface, radixclient.Interface) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := fake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient)

	return &commonTestUtils, kubeclient, radixclient
}
