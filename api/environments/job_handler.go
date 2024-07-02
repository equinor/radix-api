package environments

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	"github.com/equinor/radix-api/api/kubequery"
	"github.com/equinor/radix-api/api/models"
	"github.com/equinor/radix-api/api/utils"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	jobSchedulerV2Models "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-job-scheduler/pkg/batch"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetBatches Get batches
func (eh EnvironmentHandler) GetBatches(ctx context.Context, appName, envName, jobComponentName string) ([]deploymentModels.ScheduledBatchSummary, error) {
	radixBatches, err := eh.getRadixBatches(ctx, appName, envName, jobComponentName, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, err
	}
	jobComponent, err := eh.getActiveDeploymentJobComponent(ctx, appName, envName, jobComponentName, err)
	if err != nil {
		return nil, err
	}
	radixBatchStatuses := batch.GetRadixBatchStatuses(radixBatches, jobComponent)
	batchSummaryList := eh.getScheduledBatchSummaryList(radixBatches, radixBatchStatuses)
	sort.SliceStable(batchSummaryList, func(i, j int) bool {
		return utils.IsBefore(&batchSummaryList[j], &batchSummaryList[i])
	})
	return batchSummaryList, nil
}

// GetJobs Get jobs
func (eh EnvironmentHandler) GetJobs(ctx context.Context, appName, envName, jobComponentName string) ([]deploymentModels.ScheduledJobSummary, error) {
	singleJobBatches, err := eh.getRadixBatches(ctx, appName, envName, jobComponentName, kube.RadixBatchTypeJob)
	if err != nil {
		return nil, err
	}
	jobComponent, err := eh.getActiveDeploymentJobComponent(ctx, appName, envName, jobComponentName, err)
	if err != nil {
		return nil, err
	}
	radixBatchStatuses := batch.GetRadixBatchStatuses(singleJobBatches, jobComponent)
	jobSummaryList := eh.getScheduledSingleJobSummaryList(singleJobBatches, radixBatchStatuses)

	sort.SliceStable(jobSummaryList, func(i, j int) bool {
		return utils.IsBefore(&jobSummaryList[j], &jobSummaryList[i])
	})
	return jobSummaryList, nil
}

// GetBatch Gets batch by name
func (eh EnvironmentHandler) GetBatch(ctx context.Context, appName, envName, jobComponentName, batchName string) (*deploymentModels.ScheduledBatchSummary, error) {
	radixBatch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, err
	}
	jobComponent, err := eh.getActiveDeploymentJobComponent(ctx, appName, envName, jobComponentName, err)
	if err != nil {
		return nil, err
	}
	radixBatchStatus := batch.GetRadixBatchStatus(radixBatch, jobComponent)
	batchSummary := eh.getScheduledBatchSummary(radixBatch, &radixBatchStatus)
	batchSummary.JobList = eh.getScheduledJobSummaries(radixBatch, &radixBatchStatus)
	return &batchSummary, nil

}

// GetJob Gets job by name
func (eh EnvironmentHandler) GetJob(ctx context.Context, appName, envName, jobComponentName, jobName string) (*deploymentModels.ScheduledJobSummary, error) {
	batchName, batchJobName, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return nil, jobNotFoundError(jobName)
	}
	radixBatch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, "")
	if err != nil {
		return nil, err
	}
	radixBatchJob, err := findJobInRadixBatch(radixBatch, batchJobName)
	if err != nil {
		return nil, jobNotFoundError(batchJobName)
	}
	jobComponent, err := eh.getRadixJobDeployComponent(ctx, appName, envName, jobComponentName, radixBatch.Spec.RadixDeploymentJobRef.Name)
	if err != nil && !kubeerrors.IsNotFound(err) {
		return nil, err
	}
	radixBatchStatus := batch.GetRadixBatchStatus(radixBatch, jobComponent)
	jobSummary := eh.getScheduledJobSummary(radixBatch, *radixBatchJob, &radixBatchStatus, jobComponent)
	return &jobSummary, nil
}

// GetJobPayload Gets job payload
func (eh EnvironmentHandler) GetJobPayload(ctx context.Context, appName, envName, jobComponentName, jobName string) (io.ReadCloser, error) {
	return eh.getJobPayload(ctx, appName, envName, jobComponentName, jobName)
}

// RestartBatch Restart a scheduled or stopped batch
func (eh EnvironmentHandler) RestartBatch(ctx context.Context, appName, envName, jobComponentName, batchName string) error {
	radixBatch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return err
	}
	return batch.RestartRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, radixBatch)
}

// RestartJob Start running or stopped job by name
func (eh EnvironmentHandler) RestartJob(ctx context.Context, appName, envName, jobComponentName, jobName string) error {
	radixBatch, batchJobName, err := eh.getBatchJob(ctx, appName, envName, jobComponentName, jobName)
	if err != nil {
		return err
	}
	if _, err := findJobInRadixBatch(radixBatch, jobName); err != nil {
		return err
	}
	return batch.RestartRadixBatchJob(ctx, eh.accounts.UserAccount.RadixClient, radixBatch, batchJobName)
}

// CopyBatch Copy batch by name
func (eh EnvironmentHandler) CopyBatch(ctx context.Context, appName, envName, jobComponentName, batchName string, scheduledBatchRequest environmentModels.ScheduledBatchRequest) (*deploymentModels.ScheduledBatchSummary, error) {
	deploymentName := scheduledBatchRequest.DeploymentName
	jobComponent, err := eh.getRadixJobDeployComponent(ctx, appName, envName, jobComponentName, deploymentName)
	if err != nil {
		return nil, err
	}
	radixBatch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, err
	}
	batchStatus, err := batch.CopyRadixBatchOrJob(ctx, eh.accounts.UserAccount.RadixClient, radixBatch, "", jobComponent, deploymentName)
	if err != nil {
		return nil, err
	}
	return eh.getScheduledBatchSummary2(batchStatus, deploymentName), nil
}

// CopyJob Copy job by name
func (eh EnvironmentHandler) CopyJob(ctx context.Context, appName, envName, jobComponentName, jobName string, scheduledJobRequest environmentModels.ScheduledJobRequest) (*deploymentModels.ScheduledJobSummary, error) {
	deploymentName := scheduledJobRequest.DeploymentName
	jobComponent, err := eh.getRadixJobDeployComponent(ctx, appName, envName, jobComponentName, deploymentName)
	if err != nil {
		return nil, err
	}
	radixBatch, batchJobName, err := eh.getBatchJob(ctx, appName, envName, jobComponentName, jobName)
	if err != nil {
		return nil, err
	}
	batchStatus, err := batch.CopyRadixBatchOrJob(ctx, eh.accounts.UserAccount.RadixClient, radixBatch, batchJobName, jobComponent, deploymentName)
	if err != nil {
		return nil, err
	}
	return getScheduledJobStatus(batchStatus, jobName, deploymentName), nil
}

// StopBatch Stop batch by name
func (eh EnvironmentHandler) StopBatch(ctx context.Context, appName, envName, jobComponentName, batchName string) error {
	radixBatch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return err
	}
	return batch.StopRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, radixBatch)
}

// StopJob Stop job by name
func (eh EnvironmentHandler) StopJob(ctx context.Context, appName, envName, jobComponentName, jobName string) error {
	radixBatch, batchJobName, err := eh.getBatchJob(ctx, appName, envName, jobComponentName, jobName)
	if err != nil {
		return err
	}
	if _, err := findJobInRadixBatch(radixBatch, jobName); err != nil {
		return err
	}
	return batch.StopRadixBatchJob(ctx, eh.accounts.UserAccount.RadixClient, radixBatch, batchJobName)
}

// DeleteBatch Delete batch by name
func (eh EnvironmentHandler) DeleteBatch(ctx context.Context, appName, envName, jobComponentName, batchName string) error {
	radixBatch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return err
	}
	return batch.DeleteRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, radixBatch)
}

// DeleteJob Delete a job by name
func (eh EnvironmentHandler) DeleteJob(ctx context.Context, appName, envName, jobComponentName, jobName string) error {
	batchName, _, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return jobNotFoundError(jobName)
	}
	radixBatch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeJob)
	if err != nil {
		return err
	}
	return batch.DeleteRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, radixBatch)
}

func (eh EnvironmentHandler) getActiveRadixDeployment(ctx context.Context, appName string, envName string) (*radixv1.RadixDeployment, error) {
	rdList, err := kubequery.GetRadixDeploymentsForEnvironments(ctx, eh.accounts.UserAccount.RadixClient, appName, []string{envName}, 1)
	if err != nil {
		return nil, err
	}
	activeRd, ok := models.GetActiveDeploymentForAppEnv(appName, envName, rdList)
	if !ok {
		return nil, fmt.Errorf("no active deployment found for the app %s, environment %s", appName, envName)
	}
	return &activeRd, nil
}

func (eh EnvironmentHandler) getJobPayload(ctx context.Context, appName, envName, jobComponentName, jobName string) (io.ReadCloser, error) {
	batchName, batchJobName, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return nil, jobNotFoundError(jobName)
	}

	radixBatch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, "")
	if err != nil {
		return nil, err
	}

	jobs := slice.FindAll(radixBatch.Spec.Jobs, func(job radixv1.RadixBatchJob) bool { return job.Name == batchJobName })
	if len(jobs) == 0 {
		return nil, jobNotFoundError(jobName)
	}

	job := jobs[0]
	if job.PayloadSecretRef == nil {
		return io.NopCloser(&bytes.Buffer{}), nil
	}

	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	secret, err := eh.accounts.ServiceAccount.Client.CoreV1().Secrets(namespace).Get(ctx, job.PayloadSecretRef.Name, metav1.GetOptions{})
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return nil, environmentModels.ScheduledJobPayloadNotFoundError(appName, jobName)
		}
		return nil, err
	}

	payload, ok := secret.Data[job.PayloadSecretRef.Key]
	if !ok {
		return nil, environmentModels.ScheduledJobPayloadNotFoundError(appName, jobName)
	}

	return io.NopCloser(bytes.NewReader(payload)), nil
}

func (eh EnvironmentHandler) getRadixBatches(ctx context.Context, appName, envName, jobComponentName string, batchType kube.RadixBatchType) ([]*radixv1.RadixBatch, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	selector := radixLabels.Merge(
		radixLabels.ForApplicationName(appName),
		radixLabels.ForComponentName(jobComponentName),
		radixLabels.ForBatchType(batchType),
	)

	radixBatchList, err := eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}

	return slice.PointersOf(radixBatchList.Items).([]*radixv1.RadixBatch), nil
}

func (eh EnvironmentHandler) getRadixBatch(ctx context.Context, appName, envName, jobComponentName, batchName string, batchType kube.RadixBatchType) (*radixv1.RadixBatch, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	labelSelector := radixLabels.Merge(
		radixLabels.ForApplicationName(appName),
		radixLabels.ForComponentName(jobComponentName),
	)

	if batchType != "" {
		labelSelector = radixLabels.Merge(
			labelSelector,
			radixLabels.ForBatchType(batchType),
		)
	}

	radixBatch, err := eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(namespace).Get(ctx, batchName, metav1.GetOptions{})
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return nil, batchNotFoundError(batchName)
		}
		return nil, err
	}

	if !labelSelector.AsSelector().Matches(labels.Set(radixBatch.GetLabels())) {
		return nil, batchNotFoundError(batchName)
	}

	return radixBatch, nil
}

func (eh EnvironmentHandler) getScheduledBatchSummaryList(radixBatches []*radixv1.RadixBatch, batchStatuses []jobSchedulerV2Models.RadixBatch) []deploymentModels.ScheduledBatchSummary {
	batchStatusesMap := slice.Reduce(batchStatuses, make(map[string]*jobSchedulerV2Models.RadixBatch), func(acc map[string]*jobSchedulerV2Models.RadixBatch, batchStatus jobSchedulerV2Models.RadixBatch) map[string]*jobSchedulerV2Models.RadixBatch {
		acc[batchStatus.Name] = &batchStatus
		return acc
	})
	var summaries []deploymentModels.ScheduledBatchSummary
	for _, radixBatch := range radixBatches {
		batchStatus := batchStatusesMap[radixBatch.Name]
		summaries = append(summaries, eh.getScheduledBatchSummary(radixBatch, batchStatus))
	}
	return summaries
}

func (eh EnvironmentHandler) getScheduledBatchSummary(radixBatch *radixv1.RadixBatch, radixBatchStatus *jobSchedulerV2Models.RadixBatch) deploymentModels.ScheduledBatchSummary {
	summary := deploymentModels.ScheduledBatchSummary{
		Name:           radixBatch.Name,
		TotalJobCount:  len(radixBatch.Spec.Jobs),
		DeploymentName: radixBatch.Spec.RadixDeploymentJobRef.Name,
		JobList:        eh.getScheduledJobSummaries(radixBatch, radixBatchStatus),
	}
	if radixBatchStatus != nil {
		summary.Status = string(radixBatchStatus.Status)
		summary.Created = radixBatchStatus.CreationTime
		summary.Started = radixBatchStatus.Started
		summary.Ended = radixBatchStatus.Ended
	} else {
		summary.Status = string(radixBatch.Status.Condition.Type)
		summary.Created = radixutils.FormatTimestamp(radixBatch.GetCreationTimestamp().Time)
		summary.Started = radixutils.FormatTime(radixBatch.Status.Condition.ActiveTime)
		summary.Ended = radixutils.FormatTime(radixBatch.Status.Condition.CompletionTime)
	}
	return summary
}

func (eh EnvironmentHandler) getScheduledBatchSummary2(batchStatus *jobSchedulerV2Models.RadixBatch, deploymentName string) *deploymentModels.ScheduledBatchSummary {
	return &deploymentModels.ScheduledBatchSummary{
		Name:           batchStatus.Name,
		DeploymentName: deploymentName,
		Status:         string(batchStatus.Status),
		TotalJobCount:  len(batchStatus.JobStatuses),
		Created:        batchStatus.CreationTime,
		Started:        batchStatus.Started,
		Ended:          batchStatus.Ended,
	}
}

func getScheduledJobStatus(radixBatch *jobSchedulerV2Models.RadixBatch, jobName, deploymentName string) *deploymentModels.ScheduledJobSummary {
	jobSummary := deploymentModels.ScheduledJobSummary{
		Name:           jobName,
		DeploymentName: deploymentName,
		BatchName:      radixBatch.Name,
		Status:         string(radixBatch.Status),
	}
	if len(radixBatch.JobStatuses) == 1 {
		jobSummary.JobId = radixBatch.JobStatuses[0].JobId
	}
	return &jobSummary
}

func (eh EnvironmentHandler) getScheduledSingleJobSummaryList(singleJobBatches []*radixv1.RadixBatch, singleJobBatchStatuses []jobSchedulerV2Models.RadixBatch) []deploymentModels.ScheduledJobSummary {
	jobBatchStatusesMap := slice.Reduce(singleJobBatchStatuses, make(map[string]*jobSchedulerV2Models.RadixBatch), func(acc map[string]*jobSchedulerV2Models.RadixBatch, radixBatchStatus jobSchedulerV2Models.RadixBatch) map[string]*jobSchedulerV2Models.RadixBatch {
		acc[radixBatchStatus.Name] = &radixBatchStatus
		return acc
	})
	var summaries []deploymentModels.ScheduledJobSummary
	for _, singleJobBatch := range singleJobBatches {
		radixBatchStatus := jobBatchStatusesMap[singleJobBatch.Name]
		summaries = append(summaries, eh.getScheduledJobSummary(singleJobBatch, singleJobBatch.Spec.Jobs[0], radixBatchStatus, nil))
	}
	return summaries
}

func (eh EnvironmentHandler) getScheduledJobSummaries(radixBatch *radixv1.RadixBatch, radixBatchStatus *jobSchedulerV2Models.RadixBatch) (summaries []deploymentModels.ScheduledJobSummary) {
	for _, job := range radixBatch.Spec.Jobs {
		summaries = append(summaries, eh.getScheduledJobSummary(radixBatch, job, radixBatchStatus, nil))
	}
	return
}

func (eh EnvironmentHandler) getScheduledJobSummary(radixBatch *radixv1.RadixBatch, radixBatchJob radixv1.RadixBatchJob, radixBatchStatus *jobSchedulerV2Models.RadixBatch, jobComponent *radixv1.RadixDeployJobComponent) deploymentModels.ScheduledJobSummary {
	var batchName string
	if radixBatch.GetLabels()[kube.RadixBatchTypeLabel] == string(kube.RadixBatchTypeBatch) {
		batchName = radixBatch.GetName()
	}

	summary := deploymentModels.ScheduledJobSummary{
		Name:           fmt.Sprintf("%s-%s", radixBatch.GetName(), radixBatchJob.Name),
		DeploymentName: radixBatch.Spec.RadixDeploymentJobRef.Name,
		BatchName:      batchName,
		JobId:          radixBatchJob.JobId,
		ReplicaList:    getReplicaSummariesForJob(radixBatch, radixBatchJob),
		Status:         radixv1.RadixBatchJobApiStatusWaiting,
	}

	if jobComponent != nil {
		summary.TimeLimitSeconds = jobComponent.TimeLimitSeconds
		if radixBatchJob.TimeLimitSeconds != nil {
			summary.TimeLimitSeconds = radixBatchJob.TimeLimitSeconds
		}

		if jobComponent.BackoffLimit != nil {
			summary.BackoffLimit = *jobComponent.BackoffLimit
		}
		if radixBatchJob.BackoffLimit != nil {
			summary.BackoffLimit = *radixBatchJob.BackoffLimit
		}

		if jobComponent.Node != (radixv1.RadixNode{}) {
			summary.Node = (*deploymentModels.Node)(&jobComponent.Node)
		}
		if radixBatchJob.Node != nil {
			summary.Node = (*deploymentModels.Node)(radixBatchJob.Node)
		}

		if radixBatchJob.Resources != nil {
			summary.Resources = deploymentModels.ConvertRadixResourceRequirements(*radixBatchJob.Resources)
		} else if len(jobComponent.Resources.Requests) > 0 || len(jobComponent.Resources.Limits) > 0 {
			summary.Resources = deploymentModels.ConvertRadixResourceRequirements(jobComponent.Resources)
		}
	}
	if radixBatchStatus != nil {
		if jobStatus, ok := slice.FindFirst(radixBatchStatus.JobStatuses, func(jobStatus jobSchedulerV2Models.RadixBatchJobStatus) bool {
			return jobStatus.Name == radixBatchJob.Name
		}); ok {
			summary.Status = string(jobStatus.Status)
			summary.Created = jobStatus.CreationTime
			summary.Started = jobStatus.Started
			summary.Ended = jobStatus.Ended
			summary.Message = jobStatus.Message
			summary.FailedCount = jobStatus.Failed
			summary.Restart = jobStatus.Restart
		} else {
			summary.Status = radixv1.RadixBatchJobApiStatusWaiting
		}
	}
	return summary
}

func getReplicaSummariesForJob(radixBatch *radixv1.RadixBatch, radixBatchJob radixv1.RadixBatchJob) []deploymentModels.ReplicaSummary {
	if jobStatus, ok := slice.FindFirst(radixBatch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
		return jobStatus.Name == radixBatchJob.Name
	}); ok {
		return slice.Reduce(jobStatus.RadixBatchJobPodStatuses, make([]deploymentModels.ReplicaSummary, 0),
			func(acc []deploymentModels.ReplicaSummary, jobPodStatus radixv1.RadixBatchJobPodStatus) []deploymentModels.ReplicaSummary {
				return append(acc, getReplicaSummaryByJobPodStatus(radixBatchJob, jobPodStatus))
			})
	}
	return nil
}

func getReplicaSummaryByJobPodStatus(radixBatchJob radixv1.RadixBatchJob, jobPodStatus radixv1.RadixBatchJobPodStatus) deploymentModels.ReplicaSummary {
	summary := deploymentModels.ReplicaSummary{
		Name:          jobPodStatus.Name,
		Created:       radixutils.FormatTimestamp(jobPodStatus.CreationTime.Time),
		RestartCount:  jobPodStatus.RestartCount,
		Image:         jobPodStatus.Image,
		ImageId:       jobPodStatus.ImageID,
		PodIndex:      jobPodStatus.PodIndex,
		Reason:        jobPodStatus.Reason,
		StatusMessage: jobPodStatus.Message,
		ExitCode:      jobPodStatus.ExitCode,
		Status:        deploymentModels.ReplicaStatus{Status: string(jobPodStatus.Phase)},
	}
	if jobPodStatus.StartTime != nil {
		summary.StartTime = radixutils.FormatTimestamp(jobPodStatus.StartTime.Time)
	}
	if jobPodStatus.EndTime != nil {
		summary.EndTime = radixutils.FormatTimestamp(jobPodStatus.EndTime.Time)
	}
	if radixBatchJob.Resources != nil {
		summary.Resources = pointers.Ptr(deploymentModels.ConvertRadixResourceRequirements(*radixBatchJob.Resources))
	}
	return summary
}

func batchNotFoundError(batchName string) error {
	return radixhttp.NotFoundError(fmt.Sprintf("batch %s not found", batchName))
}

func jobNotFoundError(jobName string) error {
	return radixhttp.NotFoundError(fmt.Sprintf("job %s not found", jobName))
}

func parseBatchAndJobNameFromScheduledJobName(scheduleJobName string) (batchName, batchJobName string, ok bool) {
	scheduleJobNameParts := strings.Split(scheduleJobName, "-")
	if len(scheduleJobNameParts) < 2 {
		return
	}
	batchName = strings.Join(scheduleJobNameParts[:len(scheduleJobNameParts)-1], "-")
	batchJobName = scheduleJobNameParts[len(scheduleJobNameParts)-1]
	ok = true
	return
}

func (eh EnvironmentHandler) getBatchJob(ctx context.Context, appName string, envName string, jobComponentName string, jobName string) (*radixv1.RadixBatch, string, error) {
	batchName, batchJobName, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return nil, "", jobNotFoundError(jobName)
	}
	radixBatch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, "")
	if err != nil {
		return nil, "", err
	}
	return radixBatch, batchJobName, err
}

func (eh EnvironmentHandler) getRadixJobDeployComponent(ctx context.Context, appName, envName, jobComponentName, deploymentName string) (*radixv1.RadixDeployJobComponent, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	radixDeployment, err := eh.accounts.UserAccount.RadixClient.RadixV1().RadixDeployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	rdJob, _ := slice.FindFirst(radixDeployment.Spec.Jobs, func(job radixv1.RadixDeployJobComponent) bool { return job.Name == jobComponentName })
	return &rdJob, nil
}

func (eh EnvironmentHandler) getActiveDeploymentJobComponent(ctx context.Context, appName string, envName string, jobComponentName string, err error) (*radixv1.RadixDeployJobComponent, error) {
	activeRd, err := eh.getActiveRadixDeployment(ctx, appName, envName)
	if err != nil {
		return nil, err
	}
	jobComponent, err := eh.getRadixJobDeployComponent(ctx, appName, envName, jobComponentName, activeRd.GetName())
	if err != nil {
		return nil, err
	}
	return jobComponent, nil
}

func findJobInRadixBatch(radixBatch *radixv1.RadixBatch, jobName string) (*radixv1.RadixBatchJob, error) {
	if job, ok := slice.FindFirst(radixBatch.Spec.Jobs, func(job radixv1.RadixBatchJob) bool { return job.Name == jobName }); ok {
		return &job, nil
	}
	return nil, jobNotFoundError(jobName)
}
