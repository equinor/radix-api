package jobs

import (
	"context"
	"testing"
	"time"

	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"

	deployMock "github.com/equinor/radix-api/api/deployments/mock"
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/models"
	radixmodels "github.com/equinor/radix-common/models"
	radixutils "github.com/equinor/radix-common/utils"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

type JobHandlerTestSuite struct {
	suite.Suite
	accounts                models.Accounts
	inKubeClient            *kubefake.Clientset
	inRadixClient           *radixfake.Clientset
	outKubeClient           *kubefake.Clientset
	outRadixClient          *radixfake.Clientset
	inSecretProviderClient  *secretproviderfake.Clientset
	outSecretProviderClient *secretproviderfake.Clientset
}

type jobCreatedScenario struct {
	scenarioName      string
	jobName           string
	jobStatusCreated  metav1.Time
	creationTimestamp metav1.Time
	expectedCreated   string
}

type jobStatusScenario struct {
	scenarioName   string
	jobName        string
	condition      v1.RadixJobCondition
	stop           bool
	expectedStatus string
}

func TestRunJobHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(JobHandlerTestSuite))
}

func (s *JobHandlerTestSuite) SetupTest() {
	s.inKubeClient, s.inRadixClient, s.outKubeClient, s.outRadixClient, s.inSecretProviderClient, s.outSecretProviderClient = s.getUtils()
	accounts := models.NewAccounts(s.inKubeClient, s.inRadixClient, s.inSecretProviderClient, nil, s.outKubeClient, s.outRadixClient, s.outSecretProviderClient, nil, "", radixmodels.Impersonation{})
	s.accounts = accounts
}

func (s *JobHandlerTestSuite) Test_GetApplicationJob() {
	jobName, appName, branch, commitId, pipeline, triggeredBy := "a_job", "an_app", "a_branch", "a_commitid", v1.BuildDeploy, "a_user"
	started, ended := metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)), metav1.NewTime(time.Date(2020, 1, 2, 0, 0, 0, 0, time.Local))
	step1Name, step1Pod, step1Condition, step1Started, step1Ended, step1Components := "step1_name", "step1_pod", v1.JobRunning, metav1.Now(), metav1.NewTime(time.Now().Add(1*time.Hour)), []string{"step1_comp1", "step1_comp2"}
	step2Name := "step2_name"

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
			Steps: []v1.RadixJobStep{
				{Name: step1Name, PodName: step1Pod, Condition: step1Condition, Started: &step1Started, Ended: &step1Ended, Components: step1Components},
				{Name: step2Name},
			},
		},
	}
	s.outRadixClient.RadixV1().RadixJobs(rj.Namespace).Create(context.Background(), rj, metav1.CreateOptions{})

	deploymentName := "a_deployment"
	deploySummary := deploymentModels.DeploymentSummary{
		Name:        deploymentName,
		Environment: "any_env",
		ActiveFrom:  "any_from",
		ActiveTo:    "any_to",
		DeploymentSummaryPipelineJobInfo: deploymentModels.DeploymentSummaryPipelineJobInfo{
			CreatedByJob: "any_job",
		},
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
		ctrl := gomock.NewController(s.T())
		defer ctrl.Finish()
		dh := deployMock.NewMockDeployHandler(ctrl)
		h := Init(s.accounts, dh)
		actualJob, err := h.GetApplicationJob(appName, "missing_job")
		s.True(k8serrors.IsNotFound(err))
		s.Nil(actualJob)
	})

	s.Run("deployHandle.GetDeploymentsForJob return error", func() {
		ctrl := gomock.NewController(s.T())
		defer ctrl.Finish()

		dh := deployMock.NewMockDeployHandler(ctrl)
		dh.EXPECT().GetDeploymentsForJob(appName, jobName).Return(nil, assert.AnError).Times(1)
		dh.EXPECT().GetDeploymentWithName(gomock.Any(), gomock.Any()).Times(0)
		h := Init(s.accounts, dh)

		actualJob, actualErr := h.GetApplicationJob(appName, jobName)
		s.Equal(assert.AnError, actualErr)
		s.Nil(actualJob)
	})

	s.Run("empty deploymentSummary list should not call GetDeploymentWithName", func() {
		ctrl := gomock.NewController(s.T())
		defer ctrl.Finish()

		dh := deployMock.NewMockDeployHandler(ctrl)
		dh.EXPECT().GetDeploymentsForJob(appName, jobName).Return(nil, nil).Times(1)
		dh.EXPECT().GetDeploymentWithName(gomock.Any(), gomock.Any()).Times(0)
		h := Init(s.accounts, dh)

		actualJob, actualErr := h.GetApplicationJob(appName, jobName)
		s.NoError(actualErr)
		s.NotNil(actualJob)
	})

	s.Run("deployHandle.GetDeploymentWithName return error", func() {
		ctrl := gomock.NewController(s.T())
		defer ctrl.Finish()

		deployList := []*deploymentModels.DeploymentSummary{&deploySummary}
		dh := deployMock.NewMockDeployHandler(ctrl)
		dh.EXPECT().GetDeploymentsForJob(appName, jobName).Return(deployList, nil).Times(1)
		dh.EXPECT().GetDeploymentWithName(appName, deploymentName).Return(nil, assert.AnError).Times(1)
		h := Init(s.accounts, dh)

		actualJob, actualErr := h.GetApplicationJob(appName, jobName)
		s.Equal(assert.AnError, actualErr)
		s.Nil(actualJob)

	})

	s.Run("valid jobSummary", func() {
		ctrl := gomock.NewController(s.T())
		defer ctrl.Finish()

		deployList := []*deploymentModels.DeploymentSummary{&deploySummary}
		dh := deployMock.NewMockDeployHandler(ctrl)
		dh.EXPECT().GetDeploymentsForJob(appName, jobName).Return(deployList, nil).Times(1)
		dh.EXPECT().GetDeploymentWithName(appName, deploymentName).Return(&deployment, nil).Times(1)
		h := Init(s.accounts, dh)

		actualJob, actualErr := h.GetApplicationJob(appName, jobName)
		s.NoError(actualErr)
		s.Equal(jobName, actualJob.Name)
		s.Equal(branch, actualJob.Branch)
		s.Equal(commitId, actualJob.CommitID)
		s.Equal(triggeredBy, actualJob.TriggeredBy)
		s.Equal(radixutils.FormatTime(&started), actualJob.Started)
		s.Equal(radixutils.FormatTime(&ended), actualJob.Ended)
		s.Equal(string(pipeline), actualJob.Pipeline)
		s.ElementsMatch(deployList, actualJob.Deployments)

		expectedComponents := []deploymentModels.ComponentSummary{
			{Name: comp1Name, Type: comp1Type, Image: comp1Image},
			{Name: comp2Name, Type: comp2Type, Image: comp2Image},
		}

		s.ElementsMatch(slice.PointersOf(expectedComponents), actualJob.Components)
		expectedSteps := []jobModels.Step{
			{Name: step1Name, PodName: step1Pod, Status: string(step1Condition), Started: radixutils.FormatTime(&step1Started), Ended: radixutils.FormatTime(&step1Ended), Components: step1Components},
			{Name: step2Name},
		}
		s.ElementsMatch(expectedSteps, actualJob.Steps)
	})
}

func (s *JobHandlerTestSuite) Test_GetApplicationJob_Created() {
	appName, emptyTime := "any_app", metav1.Time{}
	scenarios := []jobCreatedScenario{
		{scenarioName: "both creation time and status.Created is empty", jobName: "job1", expectedCreated: ""},
		{scenarioName: "use CreationTimeStamp", jobName: "job2", expectedCreated: "2020-01-01T00:00:00Z", creationTimestamp: metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))},
		{scenarioName: "use Created from Status", jobName: "job3", expectedCreated: "2020-01-02T00:00:00Z", creationTimestamp: metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)), jobStatusCreated: metav1.NewTime(time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC))},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.scenarioName, func() {
			ctrl := gomock.NewController(s.T())
			defer ctrl.Finish()

			dh := deployMock.NewMockDeployHandler(ctrl)
			dh.EXPECT().GetDeploymentsForJob(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			h := Init(s.accounts, dh)
			rj := v1.RadixJob{ObjectMeta: metav1.ObjectMeta{Name: scenario.jobName, Namespace: utils.GetAppNamespace(appName), CreationTimestamp: scenario.creationTimestamp}}
			if scenario.jobStatusCreated != emptyTime {
				rj.Status.Created = &scenario.jobStatusCreated
			}
			_, err := s.outRadixClient.RadixV1().RadixJobs(rj.Namespace).Create(context.Background(), &rj, metav1.CreateOptions{})
			s.NoError(err)
			actualJob, err := h.GetApplicationJob(appName, scenario.jobName)
			s.NoError(err)
			s.Equal(scenario.expectedCreated, actualJob.Created)
		})
	}
}

func (s *JobHandlerTestSuite) Test_GetApplicationJob_Status() {
	appName := "any_app"
	scenarios := []jobStatusScenario{
		{scenarioName: "status is set to condition when stop is false", jobName: "job1", condition: v1.JobFailed, stop: false, expectedStatus: jobModels.Failed.String()},
		{scenarioName: "status is Stopping when stop is true and condition is not Stopped", jobName: "job2", condition: v1.JobRunning, stop: true, expectedStatus: jobModels.Stopping.String()},
		{scenarioName: "status is Stopped when stop is true and condition is Stopped", jobName: "job3", condition: v1.JobStopped, stop: true, expectedStatus: jobModels.Stopped.String()},
		{scenarioName: "status is Waiting when condition is empty", jobName: "job4", expectedStatus: jobModels.Waiting.String()},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.scenarioName, func() {
			ctrl := gomock.NewController(s.T())
			defer ctrl.Finish()

			dh := deployMock.NewMockDeployHandler(ctrl)
			dh.EXPECT().GetDeploymentsForJob(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			h := Init(s.accounts, dh)
			rj := v1.RadixJob{
				ObjectMeta: metav1.ObjectMeta{Name: scenario.jobName, Namespace: utils.GetAppNamespace(appName)},
				Spec:       v1.RadixJobSpec{Stop: scenario.stop},
				Status:     v1.RadixJobStatus{Condition: scenario.condition},
			}

			_, err := s.outRadixClient.RadixV1().RadixJobs(rj.Namespace).Create(context.Background(), &rj, metav1.CreateOptions{})
			s.NoError(err)
			actualJob, err := h.GetApplicationJob(appName, scenario.jobName)
			s.NoError(err)
			s.Equal(scenario.expectedStatus, actualJob.Status)
		})
	}
}

func (s *JobHandlerTestSuite) getUtils() (inKubeClient *kubefake.Clientset, inRadixClient *radixfake.Clientset, outKubeClient *kubefake.Clientset, outRadixClient *radixfake.Clientset, inSecretProviderClient *secretproviderfake.Clientset, outSecretProviderClient *secretproviderfake.Clientset) {
	inKubeClient, outKubeClient = kubefake.NewSimpleClientset(), kubefake.NewSimpleClientset()
	inRadixClient, outRadixClient = radixfake.NewSimpleClientset(), radixfake.NewSimpleClientset()
	inSecretProviderClient, outSecretProviderClient = secretproviderfake.NewSimpleClientset(), secretproviderfake.NewSimpleClientset()
	return
}
