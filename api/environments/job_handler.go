package environments

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	"github.com/equinor/radix-api/api/kubequery"
	apimodels "github.com/equinor/radix-api/api/models"
	"github.com/equinor/radix-api/api/utils"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	jobsSchedulerModels "github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetJobs Get jobs
func (eh EnvironmentHandler) GetJobs(ctx context.Context, appName, envName, jobComponentName string) ([]deploymentModels.ScheduledJobSummary, error) {
	rdList, err := kubequery.GetRadixDeploymentsForEnvironment(ctx, eh.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return nil, err
	}
	rbList, err := kubequery.GetRadixBatchesForJobComponent(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, kube.RadixBatchTypeJob)
	if err != nil {
		return nil, err
	}

	batches := apimodels.BuildScheduledBatchSummaries(rbList, rdList)

	var jobs []deploymentModels.ScheduledJobSummary
	for _, batch := range batches {
		jobs = append(jobs, batch.JobList...)
	}

	sort.SliceStable(jobs, func(i, j int) bool {
		return utils.IsBefore(&jobs[j], &jobs[i])
	})

	return jobs, nil
}

// GetJob Gets job by name
func (eh EnvironmentHandler) GetJob(ctx context.Context, appName, envName, jobComponentName, jobName string) (*deploymentModels.ScheduledJobSummary, error) {
	return eh.getJob(ctx, appName, envName, jobComponentName, jobName)
}

// GetBatches Get batches
func (eh EnvironmentHandler) GetBatches(ctx context.Context, appName, envName, jobComponentName string) ([]deploymentModels.ScheduledBatchSummary, error) {
	rdList, err := kubequery.GetRadixDeploymentsForEnvironment(ctx, eh.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return nil, err
	}
	rbList, err := kubequery.GetRadixBatchesForJobComponent(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, err
	}

	batches := apimodels.BuildScheduledBatchSummaries(rbList, rdList)
	sort.SliceStable(batches, func(i, j int) bool {
		return utils.IsBefore(&batches[j], &batches[i])
	})

	return batches, nil
}

// GetBatch Gets batch by name
func (eh EnvironmentHandler) GetBatch(ctx context.Context, appName, envName, jobComponentName, batchName string) (*deploymentModels.ScheduledBatchSummary, error) {
	return eh.getBatch(ctx, appName, envName, jobComponentName, batchName)
}

// StopJob Stop job by name
func (eh EnvironmentHandler) StopJob(ctx context.Context, appName, envName, jobComponentName, jobName string) error {
	batch, jobId, batchJobName, err := eh.getBatchJob(ctx, appName, envName, jobComponentName, jobName)
	if err != nil {
		return err
	}

	nonStoppableJob := slice.FindAll(batch.Status.JobStatuses, func(js radixv1.RadixBatchJobStatus) bool { return js.Name == batchJobName && !isBatchJobStoppable(js) })
	if len(nonStoppableJob) > 0 {
		return radixhttp.ValidationError(jobName, fmt.Sprintf("invalid job running state=%s", nonStoppableJob[0].Phase))
	}

	batch.Spec.Jobs[jobId].Stop = radixutils.BoolPtr(true)
	_, err = eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(batch.GetNamespace()).Update(ctx, batch, metav1.UpdateOptions{})
	return err
}

// RestartJob Start running or stopped job by name
func (eh EnvironmentHandler) RestartJob(ctx context.Context, appName, envName, jobComponentName, jobName string) error {
	batch, jobIdx, _, err := eh.getBatchJob(ctx, appName, envName, jobComponentName, jobName)
	if err != nil {
		return err
	}

	setRestartJobTimeout(batch, jobIdx, radixutils.FormatTimestamp(time.Now()))
	_, err = eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(batch.GetNamespace()).Update(ctx, batch, metav1.UpdateOptions{})
	return err
}

// RestartBatch Restart a scheduled or stopped batch
func (eh EnvironmentHandler) RestartBatch(ctx context.Context, appName, envName, jobComponentName, batchName string) error {
	batch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return err
	}

	restartTimestamp := radixutils.FormatTimestamp(time.Now())
	for jobIdx := 0; jobIdx < len(batch.Spec.Jobs); jobIdx++ {
		setRestartJobTimeout(batch, jobIdx, restartTimestamp)
	}
	_, err = eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(batch.GetNamespace()).Update(ctx, batch, metav1.UpdateOptions{})
	return err
}

// DeleteJob Delete job by name
func (eh EnvironmentHandler) DeleteJob(ctx context.Context, appName, envName, jobComponentName, jobName string) error {
	batchName, _, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return jobNotFoundError(jobName)
	}

	batch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeJob)
	if err != nil {
		return err
	}

	return eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(batch.GetNamespace()).Delete(ctx, batch.GetName(), metav1.DeleteOptions{})
}

// StopBatch Stop batch by name
func (eh EnvironmentHandler) StopBatch(ctx context.Context, appName, envName, jobComponentName, batchName string) error {
	batch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return err
	}

	if !isBatchStoppable(batch.Status.Condition) {
		return nil
	}

	nonStoppableJobs := slice.FindAll(batch.Status.JobStatuses, func(js radixv1.RadixBatchJobStatus) bool { return !isBatchJobStoppable(js) })
	var didChange bool
	for idx, job := range batch.Spec.Jobs {
		if slice.FindIndex(nonStoppableJobs, func(js radixv1.RadixBatchJobStatus) bool { return js.Name == job.Name }) == -1 {
			batch.Spec.Jobs[idx].Stop = radixutils.BoolPtr(true)
			didChange = true
		}
	}

	if !didChange {
		return nil
	}

	_, err = eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(batch.GetNamespace()).Update(ctx, batch, metav1.UpdateOptions{})
	return err
}

// DeleteBatch Delete batch by name
func (eh EnvironmentHandler) DeleteBatch(ctx context.Context, appName, envName, jobComponentName, batchName string) error {
	batch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return err
	}

	return eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(batch.GetNamespace()).Delete(ctx, batch.GetName(), metav1.DeleteOptions{})
}

// CopyBatch Copy batch by name
func (eh EnvironmentHandler) CopyBatch(ctx context.Context, appName, envName, jobComponentName, batchName string, scheduledBatchRequest environmentModels.ScheduledBatchRequest) (*deploymentModels.ScheduledBatchSummary, error) {
	deploymentName := scheduledBatchRequest.DeploymentName
	jobSchedulerBatchHandler := eh.jobSchedulerHandlerFactory.CreateJobSchedulerBatchHandlerForEnv(getJobSchedulerEnvFor(appName, envName, jobComponentName, deploymentName))
	batchStatus, err := jobSchedulerBatchHandler.CopyBatch(ctx, batchName, deploymentName)
	if err != nil {
		return nil, err
	}
	return eh.getBatch(ctx, appName, envName, jobComponentName, batchStatus.BatchName)
}

// CopyJob Copy job by name
func (eh EnvironmentHandler) CopyJob(ctx context.Context, appName, envName, jobComponentName, jobName string, scheduledJobRequest environmentModels.ScheduledJobRequest) (*deploymentModels.ScheduledJobSummary, error) {
	deploymentName := scheduledJobRequest.DeploymentName
	jobSchedulerJobHandler := eh.jobSchedulerHandlerFactory.CreateJobSchedulerJobHandlerForEnv(getJobSchedulerEnvFor(appName, envName, jobComponentName, deploymentName))
	jobStatus, err := jobSchedulerJobHandler.CopyJob(ctx, jobName, deploymentName)
	if err != nil {
		return nil, err
	}

	return eh.getJob(ctx, appName, envName, jobComponentName, jobStatus.Name)
}

// GetJobPayload Gets job payload
func (eh EnvironmentHandler) GetJobPayload(ctx context.Context, appName, envName, jobComponentName, jobName string) (io.ReadCloser, error) {
	return eh.getJobPayload(ctx, appName, envName, jobComponentName, jobName)
}

func (eh EnvironmentHandler) getJob(ctx context.Context, appName, envName, jobComponentName, jobName string) (*deploymentModels.ScheduledJobSummary, error) {
	batchName, _, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return nil, jobNotFoundError(jobName)
	}

	rb, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, "")
	if err != nil {
		return nil, err
	}

	rd, err := kubequery.GetRadixDeploymentByName(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, rb.Spec.RadixDeploymentJobRef.Name)
	if err != nil && !kubeerrors.IsNotFound(err) {
		return nil, err
	}

	batch := apimodels.BuildScheduledBatchSummary(rb, rd)
	job, found := slice.FindFirst(batch.JobList, func(j deploymentModels.ScheduledJobSummary) bool { return j.Name == jobName })
	if !found {
		return nil, jobNotFoundError(jobName)
	}

	return &job, nil
}

func (eh EnvironmentHandler) getBatch(ctx context.Context, appName, envName, jobComponentName, batchName string) (*deploymentModels.ScheduledBatchSummary, error) {
	rb, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, err
	}

	rd, err := kubequery.GetRadixDeploymentByName(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, rb.Spec.RadixDeploymentJobRef.Name)
	if err != nil && !kubeerrors.IsNotFound(err) {
		return nil, err
	}

	return apimodels.BuildScheduledBatchSummary(rb, rd), nil
}

func (eh EnvironmentHandler) getJobPayload(ctx context.Context, appName, envName, jobComponentName, jobName string) (io.ReadCloser, error) {
	batchName, batchJobName, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return nil, jobNotFoundError(jobName)
	}

	batch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, "")
	if err != nil {
		return nil, err
	}

	jobs := slice.FindAll(batch.Spec.Jobs, func(job radixv1.RadixBatchJob) bool { return job.Name == batchJobName })
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

	batch, err := eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(namespace).Get(ctx, batchName, metav1.GetOptions{})
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return nil, batchNotFoundError(batchName)
		}
		return nil, err
	}

	if !labelSelector.AsSelector().Matches(labels.Set(batch.GetLabels())) {
		return nil, batchNotFoundError(batchName)
	}

	return batch, nil
}

// check if batch can be stopped
func isBatchStoppable(condition radixv1.RadixBatchCondition) bool {
	return condition.Type == "" ||
		condition.Type == radixv1.BatchConditionTypeActive ||
		condition.Type == radixv1.BatchConditionTypeWaiting
}

// check if batch job can be stopped
func isBatchJobStoppable(status radixv1.RadixBatchJobStatus) bool {
	return status.Phase == "" ||
		status.Phase == radixv1.BatchJobPhaseActive ||
		status.Phase == radixv1.BatchJobPhaseWaiting ||
		status.Phase == radixv1.BatchJobPhaseRunning
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

func (eh EnvironmentHandler) getBatchJob(ctx context.Context, appName string, envName string, jobComponentName string, jobName string) (*radixv1.RadixBatch, int, string, error) {
	batchName, batchJobName, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return nil, 0, "", jobNotFoundError(jobName)
	}

	batch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, "")
	if err != nil {
		return nil, 0, "", err
	}

	idx := slice.FindIndex(batch.Spec.Jobs, func(job radixv1.RadixBatchJob) bool { return job.Name == batchJobName })
	if idx == -1 {
		return nil, 0, "", jobNotFoundError(jobName)
	}
	return batch, idx, batchJobName, err
}

func getJobSchedulerEnvFor(appName, envName, jobComponentName, deploymentName string) *jobsSchedulerModels.Env {
	return &jobsSchedulerModels.Env{
		RadixComponentName:                           jobComponentName,
		RadixDeploymentName:                          deploymentName,
		RadixDeploymentNamespace:                     operatorUtils.GetEnvironmentNamespace(appName, envName),
		RadixJobSchedulersPerEnvironmentHistoryLimit: 10,
	}
}

func setRestartJobTimeout(batch *radixv1.RadixBatch, jobIdx int, restartTimestamp string) {
	batch.Spec.Jobs[jobIdx].Stop = nil
	batch.Spec.Jobs[jobIdx].Restart = restartTimestamp
}
