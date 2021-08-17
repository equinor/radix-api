package jobs

import (
	"context"
	"encoding/json"
	"testing"
	"time"

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
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

type JobHandlerTestSuite struct {
	suite.Suite
	accounts       models.Accounts
	inKubeClient   *kubefake.Clientset
	inRadixClient  *radixfake.Clientset
	outKubeClient  *kubefake.Clientset
	outRadixClient *radixfake.Clientset
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
	s.inKubeClient, s.inRadixClient, s.outKubeClient, s.outRadixClient = s.getUtils()
	accounts := models.NewAccounts(s.inKubeClient, s.inRadixClient, s.outKubeClient, s.outRadixClient, "", radixmodels.Impersonation{})
	s.accounts = accounts
}

func (s *JobHandlerTestSuite) Test_GetApplicationJob() {
	jobName, appName, branch, commitId, pipeline, triggeredBy := "a_job", "an_app", "a_branch", "a_commitid", v1.BuildDeploy, "a_user"
	started, ended := metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)), metav1.NewTime(time.Date(2020, 1, 2, 0, 0, 0, 0, time.Local))
	step1Name, step1Pod, step1Condition, step1Started, step1Ended, step1Components := "step1_name", "step1_pod", v1.JobRunning, metav1.Now(), metav1.NewTime(time.Now().Add(1*time.Hour)), []string{"step1_comp1", "step1_comp2"}
	step2Name, step2ScanStatus, step2ScanReason, step2ScanVuln := "step2_name", v1.ScanSuccess, "any_reason", v1.VulnerabilityMap{"v1": 5, "v2": 10}
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
				{Name: step2Name, Output: &v1.RadixJobStepOutput{
					Scan: &v1.RadixJobStepScanOutput{Status: step2ScanStatus, Reason: step2ScanReason, Vulnerabilities: step2ScanVuln},
				}},
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
			{Name: step2Name, VulnerabilityScan: &jobModels.VulnerabilityScan{Status: string(step2ScanStatus), Reason: step2ScanReason, Vulnerabilities: step2ScanVuln}},
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

func (s *JobHandlerTestSuite) Test_GetPipelineJobStepScanOutput() {
	s.T().Parallel()

	s.outRadixClient.Tracker().Add(&v1.RadixJob{
		ObjectMeta: metav1.ObjectMeta{Name: "anyjob", Namespace: "anyapp-app"},
		Status: v1.RadixJobStatus{
			Steps: []v1.RadixJobStep{
				{Name: "step-no-output"},
				{Name: "step-no-scan", Output: &v1.RadixJobStepOutput{}},
				{Name: "step-cm-not-defined", Output: &v1.RadixJobStepOutput{Scan: &v1.RadixJobStepScanOutput{
					Status:               v1.ScanSuccess,
					VulnerabilityListKey: "any-key",
				}}},
				{Name: "step-cm-key-not-defined", Output: &v1.RadixJobStepOutput{Scan: &v1.RadixJobStepScanOutput{
					Status:                     v1.ScanSuccess,
					VulnerabilityListConfigMap: "any-cm",
				}}},
				{Name: "step-cm-missing", Output: &v1.RadixJobStepOutput{Scan: &v1.RadixJobStepScanOutput{
					Status:                     v1.ScanSuccess,
					VulnerabilityListConfigMap: "any-cm",
					VulnerabilityListKey:       "any-key",
				}}},
				{Name: "step-cm-key-missing", Output: &v1.RadixJobStepOutput{Scan: &v1.RadixJobStepScanOutput{
					Status:                     v1.ScanSuccess,
					VulnerabilityListConfigMap: "cm",
					VulnerabilityListKey:       "any-key",
				}}},
				{Name: "step-cm-invalid-data", Output: &v1.RadixJobStepOutput{Scan: &v1.RadixJobStepScanOutput{
					Status:                     v1.ScanSuccess,
					VulnerabilityListConfigMap: "cm",
					VulnerabilityListKey:       "invalid-data",
				}}},
				{Name: "step-cm-valid", Output: &v1.RadixJobStepOutput{Scan: &v1.RadixJobStepScanOutput{
					Status:                     v1.ScanSuccess,
					VulnerabilityListConfigMap: "cm",
					VulnerabilityListKey:       "valid-data",
				}}},
			},
		},
	})

	vulnerabilities := []jobModels.Vulnerability{
		{
			PackageName:   "packageName1",
			Version:       "version1",
			Target:        "target1",
			Title:         "title1",
			Description:   "description1",
			Serverity:     "severity1",
			PublishedDate: "publishDate1",
			CWE:           []string{"cwe1.1", "cwe1.2"},
			CVE:           []string{"cve1.1", "cve1.2"},
			CVSS:          1,
			References:    []string{"ref1.1", "ref1.2"},
		},
		{
			PackageName:   "packageName2",
			Version:       "version2",
			Target:        "target2",
			Title:         "title2",
			Description:   "description2",
			Serverity:     "severity2",
			PublishedDate: "publishDate2",
			CWE:           []string{"cwe2.1", "cwe2.2"},
			CVE:           []string{"cve2.1", "cve2.2"},
			CVSS:          1,
			References:    []string{"ref2.1", "ref2.2"},
		},
	}
	vulnerabilityBytes, _ := json.Marshal(&vulnerabilities)
	s.outKubeClient.Tracker().Add(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "anyapp-app"},
		Data: map[string]string{
			"invalid-data": "",
			"valid-data":   string(vulnerabilityBytes),
		},
	})

	s.Run("RadixJob does not exist", func() {
		h := Init(s.accounts, nil)
		actual, err := h.GetPipelineJobStepScanOutput("anyapp", "non-existing-job", "anystep")
		s.Nil(actual)
		s.EqualError(err, k8serrors.NewNotFound(schema.GroupResource{Resource: "radixjobs.radix.equinor.com"}, "non-existing-job").Error())
	})
	s.Run("Step does not exist", func() {
		h := Init(s.accounts, nil)
		actual, err := h.GetPipelineJobStepScanOutput("anyapp", "anyjob", "non-existing-step")
		s.Nil(actual)
		s.EqualError(err, stepNotFoundError("non-existing-step").Error())
	})
	s.Run("Step Output not set", func() {
		h := Init(s.accounts, nil)
		actual, err := h.GetPipelineJobStepScanOutput("anyapp", "anyjob", "step-no-output")
		s.Nil(actual)
		s.EqualError(err, stepScanOutputNotDefined("step-no-output").Error())
	})
	s.Run("Step Output.Scan not set", func() {
		h := Init(s.accounts, nil)
		actual, err := h.GetPipelineJobStepScanOutput("anyapp", "anyjob", "step-no-scan")
		s.Nil(actual)
		s.EqualError(err, stepScanOutputNotDefined("step-no-scan").Error())
	})
	s.Run("Step Output.Scan.VulnerabilityListConfigMap not set", func() {
		h := Init(s.accounts, nil)
		actual, err := h.GetPipelineJobStepScanOutput("anyapp", "anyjob", "step-cm-not-defined")
		s.Nil(actual)
		s.EqualError(err, stepScanOutputInvalidConfig("step-cm-not-defined").Error())
	})
	s.Run("Step Output.Scan.VulnerabilityListKey not set", func() {
		h := Init(s.accounts, nil)
		actual, err := h.GetPipelineJobStepScanOutput("anyapp", "anyjob", "step-cm-key-not-defined")
		s.Nil(actual)
		s.EqualError(err, stepScanOutputInvalidConfig("step-cm-key-not-defined").Error())
	})
	s.Run("ConfigMap defined in step does not exist", func() {
		h := Init(s.accounts, nil)
		actual, err := h.GetPipelineJobStepScanOutput("anyapp", "anyjob", "step-cm-missing")
		s.Nil(actual)
		s.EqualError(err, k8serrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "any-cm").Error())
	})
	s.Run("Key defined in step does not exist in ConfigMap", func() {
		h := Init(s.accounts, nil)
		actual, err := h.GetPipelineJobStepScanOutput("anyapp", "anyjob", "step-cm-key-missing")
		s.Nil(actual)
		s.EqualError(err, stepScanOutputMissingKeyInConfigMap("step-cm-key-missing").Error())
	})
	s.Run("ConfigMap data for key is invalid", func() {
		h := Init(s.accounts, nil)
		actual, err := h.GetPipelineJobStepScanOutput("anyapp", "anyjob", "step-cm-invalid-data")
		s.Nil(actual)
		s.EqualError(err, stepScanOutputInvalidConfigMapData("step-cm-invalid-data").Error())
	})
	s.Run("Valid step and ConfigMap data", func() {
		h := Init(s.accounts, nil)
		actual, err := h.GetPipelineJobStepScanOutput("anyapp", "anyjob", "step-cm-valid")
		s.ElementsMatch(vulnerabilities, actual)
		s.NoError(err)
	})

}

func (s *JobHandlerTestSuite) getUtils() (inKubeClient *kubefake.Clientset, inRadixClient *radixfake.Clientset, outKubeClient *kubefake.Clientset, outRadixClient *radixfake.Clientset) {
	inKubeClient, outKubeClient = kubefake.NewSimpleClientset(), kubefake.NewSimpleClientset()
	inRadixClient, outRadixClient = radixfake.NewSimpleClientset(), radixfake.NewSimpleClientset()
	return
}
