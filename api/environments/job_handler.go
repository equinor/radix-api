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
	"github.com/equinor/radix-api/api/utils/predicate"
	radixhttp "github.com/equinor/radix-common/net/http"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	jobSchedulerBatch "github.com/equinor/radix-job-scheduler/pkg/batch"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetBatches Get batches
func (eh EnvironmentHandler) GetBatches(ctx context.Context, appName, envName, jobComponentName string) ([]deploymentModels.ScheduledBatchSummary, error) {
	radixBatches, err := kubequery.GetRadixBatches(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, err
	}
	radixDeploymentsMap, err := kubequery.GetRadixDeploymentsMapForEnvironment(ctx, eh.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return nil, err
	}
	activeDeployJobComponent, err := getActiveDeployJobComponent(appName, envName, jobComponentName, radixDeploymentsMap)
	if err != nil {
		return nil, err
	}
	radixBatchStatuses := jobSchedulerBatch.GetRadixBatchStatuses(radixBatches, activeDeployJobComponent)
	batchSummaryList := models.GetScheduledBatchSummaryList(radixBatches, radixBatchStatuses, radixDeploymentsMap, jobComponentName)
	sort.SliceStable(batchSummaryList, func(i, j int) bool {
		return utils.IsBefore(&batchSummaryList[j], &batchSummaryList[i])
	})
	return batchSummaryList, nil
}

// GetJobs Get jobs
func (eh EnvironmentHandler) GetJobs(ctx context.Context, appName, envName, jobComponentName string) ([]deploymentModels.ScheduledJobSummary, error) {
	singleJobBatches, err := kubequery.GetRadixBatches(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, kube.RadixBatchTypeJob)
	if err != nil {
		return nil, err
	}
	radixDeploymentsMap, err := kubequery.GetRadixDeploymentsMapForEnvironment(ctx, eh.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return nil, err
	}
	activeRadixDeployJobComponent, err := getActiveDeployJobComponent(appName, envName, jobComponentName, radixDeploymentsMap)
	if err != nil {
		return nil, err
	}
	radixBatchStatuses := jobSchedulerBatch.GetRadixBatchStatuses(singleJobBatches, activeRadixDeployJobComponent)
	jobSummaryList := models.GetScheduledSingleJobSummaryList(singleJobBatches, radixBatchStatuses, radixDeploymentsMap, jobComponentName)
	sort.SliceStable(jobSummaryList, func(i, j int) bool {
		return utils.IsBefore(&jobSummaryList[j], &jobSummaryList[i])
	})
	return jobSummaryList, nil
}

// GetBatch Gets batch by name
func (eh EnvironmentHandler) GetBatch(ctx context.Context, appName, envName, jobComponentName, batchName string) (*deploymentModels.ScheduledBatchSummary, error) {
	radixBatch, err := kubequery.GetRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, err
	}
	radixDeploymentsMap, err := kubequery.GetRadixDeploymentsMapForEnvironment(ctx, eh.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return nil, err
	}
	activeDeployJobComponent, err := getActiveDeployJobComponent(appName, envName, jobComponentName, radixDeploymentsMap)
	if err != nil {
		return nil, err
	}
	radixBatchStatus := jobSchedulerBatch.GetRadixBatchStatus(radixBatch, activeDeployJobComponent)
	batchDeployJobComponent := models.GetBatchDeployJobComponent(radixBatch, jobComponentName, radixDeploymentsMap)
	batchSummary := models.GetScheduledBatchSummary(radixBatch, &radixBatchStatus, batchDeployJobComponent)
	return &batchSummary, nil
}

// GetJob Gets job by name
func (eh EnvironmentHandler) GetJob(ctx context.Context, appName, envName, jobComponentName, jobName string) (*deploymentModels.ScheduledJobSummary, error) {
	batchName, batchJobName, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return nil, jobNotFoundError(jobName)
	}
	radixBatch, err := kubequery.GetRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, batchName, "")
	if err != nil {
		return nil, err
	}
	radixBatchJob, err := findJobInRadixBatch(radixBatch, batchJobName)
	if err != nil {
		return nil, jobNotFoundError(batchJobName)
	}
	radixDeploymentsMap, err := kubequery.GetRadixDeploymentsMapForEnvironment(ctx, eh.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return nil, err
	}
	activeDeployJobComponent, err := getActiveDeployJobComponent(appName, envName, jobComponentName, radixDeploymentsMap)
	if err != nil {
		return nil, err
	}
	batchDeployJobComponent := getDeployJobComponent(radixDeploymentsMap, radixBatch.Spec.RadixDeploymentJobRef.Name, jobComponentName)
	if err != nil && !kubeerrors.IsNotFound(err) {
		return nil, err
	}
	radixBatchStatus := jobSchedulerBatch.GetRadixBatchStatus(radixBatch, activeDeployJobComponent)
	return pointers.Ptr(models.GetScheduledJobSummary(radixBatch, radixBatchJob, &radixBatchStatus, batchDeployJobComponent)), nil
}

// GetJobPayload Gets job payload
func (eh EnvironmentHandler) GetJobPayload(ctx context.Context, appName, envName, jobComponentName, jobName string) (io.ReadCloser, error) {
	batchName, batchJobName, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return nil, jobNotFoundError(jobName)
	}
	radixBatch, err := kubequery.GetRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, batchName, "")
	if err != nil {
		return nil, err
	}
	radixBatchJobs := slice.FindAll(radixBatch.Spec.Jobs, func(job radixv1.RadixBatchJob) bool { return job.Name == batchJobName })
	if len(radixBatchJobs) == 0 {
		return nil, jobNotFoundError(jobName)
	}
	radixBatchJob := radixBatchJobs[0]
	if radixBatchJob.PayloadSecretRef == nil {
		return io.NopCloser(&bytes.Buffer{}), nil
	}
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	secret, err := eh.accounts.ServiceAccount.Client.CoreV1().Secrets(namespace).Get(ctx, radixBatchJob.PayloadSecretRef.Name, metav1.GetOptions{})
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return nil, environmentModels.ScheduledJobPayloadNotFoundError(appName, jobName)
		}
		return nil, err
	}
	payload, ok := secret.Data[radixBatchJob.PayloadSecretRef.Key]
	if !ok {
		return nil, environmentModels.ScheduledJobPayloadNotFoundError(appName, jobName)
	}
	return io.NopCloser(bytes.NewReader(payload)), nil
}

// RestartBatch Restart a scheduled or stopped batch
func (eh EnvironmentHandler) RestartBatch(ctx context.Context, appName, envName, jobComponentName, batchName string) error {
	radixBatch, err := kubequery.GetRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return err
	}
	return jobSchedulerBatch.RestartRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, radixBatch)
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
	return jobSchedulerBatch.RestartRadixBatchJob(ctx, eh.accounts.UserAccount.RadixClient, radixBatch, batchJobName)
}

// CopyBatch Copy batch by name
func (eh EnvironmentHandler) CopyBatch(ctx context.Context, appName, envName, jobComponentName, batchName string, scheduledBatchRequest environmentModels.ScheduledBatchRequest) (*deploymentModels.ScheduledBatchSummary, error) {
	radixBatch, err := kubequery.GetRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, err
	}
	radixDeploymentsMap, err := kubequery.GetRadixDeploymentsMapForEnvironment(ctx, eh.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return nil, err
	}
	activeDeployJobComponent, err := getActiveDeployJobComponent(appName, envName, jobComponentName, radixDeploymentsMap)
	if err != nil {
		return nil, err
	}
	batchDeployJobComponent := getDeployJobComponent(radixDeploymentsMap, scheduledBatchRequest.DeploymentName, jobComponentName)
	if err != nil {
		return nil, err
	}
	batchStatus, err := jobSchedulerBatch.CopyRadixBatchOrJob(ctx, eh.accounts.UserAccount.RadixClient, radixBatch, "", activeDeployJobComponent, scheduledBatchRequest.DeploymentName)
	if err != nil {
		return nil, err
	}
	summary := models.GetScheduledBatchSummary(radixBatch, batchStatus, batchDeployJobComponent)
	return &summary, nil
}

// CopyJob Copy job by name
func (eh EnvironmentHandler) CopyJob(ctx context.Context, appName, envName, jobComponentName, jobName string, scheduledJobRequest environmentModels.ScheduledJobRequest) (*deploymentModels.ScheduledJobSummary, error) {
	radixBatch, batchJobName, err := eh.getBatchJob(ctx, appName, envName, jobComponentName, jobName)
	if err != nil {
		return nil, err
	}
	radixDeploymentsMap, err := kubequery.GetRadixDeploymentsMapForEnvironment(ctx, eh.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return nil, err
	}
	activeDeployJobComponent, err := getActiveDeployJobComponent(appName, envName, jobComponentName, radixDeploymentsMap)
	if err != nil {
		return nil, err
	}
	batchDeployJobComponent := getDeployJobComponent(radixDeploymentsMap, scheduledJobRequest.DeploymentName, jobComponentName)
	if err != nil {
		return nil, err
	}
	radixBatchStatus, err := jobSchedulerBatch.CopyRadixBatchOrJob(ctx, eh.accounts.UserAccount.RadixClient, radixBatch, batchJobName, activeDeployJobComponent, scheduledJobRequest.DeploymentName)
	if err != nil {
		return nil, err
	}
	radixBatchJob, ok := slice.FindFirst(radixBatch.Spec.Jobs, func(job radixv1.RadixBatchJob) bool { return job.Name == batchJobName })
	if !ok {
		return nil, jobNotFoundError(jobName)
	}
	return pointers.Ptr(models.GetScheduledJobSummary(radixBatch, &radixBatchJob, radixBatchStatus, batchDeployJobComponent)), nil
}

// StopBatch Stop batch by name
func (eh EnvironmentHandler) StopBatch(ctx context.Context, appName, envName, jobComponentName, batchName string) error {
	radixBatch, err := kubequery.GetRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return err
	}
	return jobSchedulerBatch.StopRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, radixBatch)
}

// StopJob Stop job by name
func (eh EnvironmentHandler) StopJob(ctx context.Context, appName, envName, jobComponentName, jobName string) error {
	radixBatch, batchJobName, err := eh.getBatchJob(ctx, appName, envName, jobComponentName, jobName)
	if err != nil {
		return err
	}
	if _, err := findJobInRadixBatch(radixBatch, batchJobName); err != nil {
		return err
	}
	return jobSchedulerBatch.StopRadixBatchJob(ctx, eh.accounts.UserAccount.RadixClient, radixBatch, batchJobName)
}

// DeleteBatch Delete batch by name
func (eh EnvironmentHandler) DeleteBatch(ctx context.Context, appName, envName, jobComponentName, batchName string) error {
	radixBatch, err := kubequery.GetRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return err
	}
	return jobSchedulerBatch.DeleteRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, radixBatch)
}

// DeleteJob Delete a job by name
func (eh EnvironmentHandler) DeleteJob(ctx context.Context, appName, envName, jobComponentName, jobName string) error {
	batchName, _, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return jobNotFoundError(jobName)
	}
	radixBatch, err := kubequery.GetRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeJob)
	if err != nil {
		return err
	}
	return jobSchedulerBatch.DeleteRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, radixBatch)
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
	radixBatch, err := kubequery.GetRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, batchName, "")
	if err != nil {
		return nil, "", err
	}
	return radixBatch, batchJobName, err
}

func getDeployJobComponent(radixDeploymentsMap map[string]radixv1.RadixDeployment, radixDeploymentName, jobComponentName string) *radixv1.RadixDeployJobComponent {
	radixDeployment, ok := radixDeploymentsMap[radixDeploymentName]
	if !ok {
		return nil
	}
	return getDeployJobComponentFromRadixDeployment(&radixDeployment, jobComponentName)
}

func getDeployJobComponentFromRadixDeployment(radixDeployment *radixv1.RadixDeployment, jobComponentName string) *radixv1.RadixDeployJobComponent {
	deployJobComponent, _ := slice.FindFirst(radixDeployment.Spec.Jobs, func(job radixv1.RadixDeployJobComponent) bool { return job.Name == jobComponentName })
	return &deployJobComponent
}

func getActiveDeployJobComponent(appName string, envName string, jobComponentName string, radixDeploymentMap map[string]radixv1.RadixDeployment) (*radixv1.RadixDeployJobComponent, error) {
	activeRd, err := getActiveRadixDeployment(appName, envName, radixDeploymentMap)
	if err != nil {
		return nil, err
	}
	return getDeployJobComponentFromRadixDeployment(activeRd, jobComponentName), nil
}

func getActiveRadixDeployment(appName string, envName string, radixDeploymentMap map[string]radixv1.RadixDeployment) (*radixv1.RadixDeployment, error) {
	for _, radixDeployment := range radixDeploymentMap {
		if predicate.IsActiveRadixDeployment(radixDeployment) {
			return &radixDeployment, nil
		}
	}
	return nil, fmt.Errorf("no active deployment found for the app %s, environment %s", appName, envName)
}

func findJobInRadixBatch(radixBatch *radixv1.RadixBatch, jobName string) (*radixv1.RadixBatchJob, error) {
	if job, ok := slice.FindFirst(radixBatch.Spec.Jobs, func(job radixv1.RadixBatchJob) bool { return job.Name == jobName }); ok {
		return &job, nil
	}
	return nil, jobNotFoundError(jobName)
}
