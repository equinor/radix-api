package environments

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"

	deploymentmodels "github.com/equinor/radix-api/api/deployments/models"
	environmentmodels "github.com/equinor/radix-api/api/environments/models"
	"github.com/equinor/radix-api/api/kubequery"
	"github.com/equinor/radix-api/api/models"
	"github.com/equinor/radix-api/api/utils"
	radixhttp "github.com/equinor/radix-common/net/http"
	"github.com/equinor/radix-common/utils/slice"
	jobschedulerbatch "github.com/equinor/radix-job-scheduler/pkg/batch"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetJobs Get jobs
func (eh EnvironmentHandler) GetJobs(ctx context.Context, appName, envName, jobComponentName string) ([]deploymentmodels.ScheduledJobSummary, error) {
	radixBatches, err := kubequery.GetRadixBatches(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, kube.RadixBatchTypeJob)
	if err != nil {
		return nil, err
	}
	envRdList, err := kubequery.GetRadixDeploymentsForEnvironment(ctx, eh.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return nil, err
	}
	batchSummaries := slice.Map(radixBatches, func(rb radixv1.RadixBatch) deploymentmodels.ScheduledBatchSummary {
		return *models.BuildScheduledBatchSummary(&rb, envRdList)
	})
	jobSummaries := slices.Concat(slice.Map(batchSummaries, func(v deploymentmodels.ScheduledBatchSummary) []deploymentmodels.ScheduledJobSummary {
		return v.JobList
	})...)
	sort.SliceStable(jobSummaries, func(i, j int) bool {
		return utils.IsBefore(&jobSummaries[j], &jobSummaries[i])
	})
	return jobSummaries, nil
}

// GetJob Gets job by name
func (eh EnvironmentHandler) GetJob(ctx context.Context, appName, envName, jobComponentName, jobName string) (*deploymentmodels.ScheduledJobSummary, error) {
	batchName, batchJobName, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return nil, jobNotFoundError(jobName)
	}
	radixBatch, err := kubequery.GetRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, batchName, "")
	if err != nil {
		return nil, err
	}
	if !slice.Any(radixBatch.Spec.Jobs, isRadixBatchJobWithName(batchJobName)) {
		return nil, jobNotFoundError(batchJobName)
	}
	envRdList, err := kubequery.GetRadixDeploymentsForEnvironment(ctx, eh.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return nil, err
	}
	batchSummary := models.BuildScheduledBatchSummary(radixBatch, envRdList)
	jobSummary, ok := slice.FindFirst(batchSummary.JobList, isScheduledJobSummaryWithName(jobName))
	if !ok {
		return nil, radixhttp.UnexpectedError("Internal Error", errors.New("failed to find job in ScheduleBatchSummary"))
	}
	return &jobSummary, nil
}

// GetBatches Get batches
func (eh EnvironmentHandler) GetBatches(ctx context.Context, appName, envName, jobComponentName string) ([]deploymentmodels.ScheduledBatchSummary, error) {
	radixBatches, err := kubequery.GetRadixBatches(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, err
	}
	envRdList, err := kubequery.GetRadixDeploymentsForEnvironment(ctx, eh.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return nil, err
	}
	batchSummaryList := slice.Map(radixBatches, func(rb radixv1.RadixBatch) deploymentmodels.ScheduledBatchSummary {
		return *models.BuildScheduledBatchSummary(&rb, envRdList)
	})
	sort.SliceStable(batchSummaryList, func(i, j int) bool {
		return utils.IsBefore(&batchSummaryList[j], &batchSummaryList[i])
	})
	return batchSummaryList, nil
}

// GetBatch Gets batch by name
func (eh EnvironmentHandler) GetBatch(ctx context.Context, appName, envName, jobComponentName, batchName string) (*deploymentmodels.ScheduledBatchSummary, error) {
	radixBatch, err := kubequery.GetRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, err
	}
	envRdList, err := kubequery.GetRadixDeploymentsForEnvironment(ctx, eh.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return nil, err
	}
	batchSummary := models.BuildScheduledBatchSummary(radixBatch, envRdList)
	return batchSummary, nil
}

// RestartBatch Restart a scheduled or stopped batch
func (eh EnvironmentHandler) RestartBatch(ctx context.Context, appName, envName, jobComponentName, batchName string) error {
	radixBatch, err := kubequery.GetRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return err
	}
	return jobschedulerbatch.RestartRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, radixBatch)
}

// RestartJob Start running or stopped job by name
func (eh EnvironmentHandler) RestartJob(ctx context.Context, appName, envName, jobComponentName, jobName string) error {
	batchName, batchJobName, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return jobNotFoundError(jobName)
	}
	radixBatch, err := kubequery.GetRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, batchName, "")
	if err != nil {
		return err
	}
	if !slice.Any(radixBatch.Spec.Jobs, isRadixBatchJobWithName(batchJobName)) {
		return jobNotFoundError(batchJobName)
	}
	return jobschedulerbatch.RestartRadixBatchJob(ctx, eh.accounts.UserAccount.RadixClient, radixBatch, batchJobName)
}

// CopyBatch Copy batch by name
func (eh EnvironmentHandler) CopyBatch(ctx context.Context, appName, envName, jobComponentName, batchName string, scheduledBatchRequest environmentmodels.ScheduledBatchRequest) (*deploymentmodels.ScheduledBatchSummary, error) {
	radixBatch, err := kubequery.GetRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, err
	}
	envRdList, err := kubequery.GetRadixDeploymentsForEnvironment(ctx, eh.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return nil, err
	}
	newRadixBatch, err := jobschedulerbatch.CopyRadixBatchOrJob(ctx, eh.accounts.UserAccount.RadixClient, radixBatch, "", scheduledBatchRequest.DeploymentName)
	if err != nil {
		return nil, err
	}
	summary := models.BuildScheduledBatchSummary(newRadixBatch, envRdList)
	return summary, nil
}

// CopyJob Copy job by name
func (eh EnvironmentHandler) CopyJob(ctx context.Context, appName, envName, jobComponentName, jobName string, scheduledJobRequest environmentmodels.ScheduledJobRequest) (*deploymentmodels.ScheduledJobSummary, error) {
	batchName, batchJobName, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return nil, jobNotFoundError(jobName)
	}
	radixBatch, err := kubequery.GetRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, batchName, "")
	if err != nil {
		return nil, err
	}
	if !slice.Any(radixBatch.Spec.Jobs, isRadixBatchJobWithName(batchJobName)) {
		return nil, jobNotFoundError(batchJobName)
	}
	envRdList, err := kubequery.GetRadixDeploymentsForEnvironment(ctx, eh.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return nil, err
	}
	newRadixBatch, err := jobschedulerbatch.CopyRadixBatchOrJob(ctx, eh.accounts.UserAccount.RadixClient, radixBatch, batchJobName, scheduledJobRequest.DeploymentName)
	if err != nil {
		return nil, err
	}
	batchSummary := models.BuildScheduledBatchSummary(newRadixBatch, envRdList)
	jobSummary, ok := slice.FindFirst(batchSummary.JobList, isScheduledJobSummaryWithName(jobName))
	if !ok {
		return nil, radixhttp.UnexpectedError("Internal Error", errors.New("failed to find job in ScheduleBatchSummary"))
	}
	return &jobSummary, nil
}

// StopBatch Stop batch by name
func (eh EnvironmentHandler) StopBatch(ctx context.Context, appName, envName, jobComponentName, batchName string) error {
	radixBatch, err := kubequery.GetRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return err
	}
	return jobschedulerbatch.StopRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, radixBatch)
}

// StopJob Stop job by name
func (eh EnvironmentHandler) StopJob(ctx context.Context, appName, envName, jobComponentName, jobName string) error {
	batchName, batchJobName, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return jobNotFoundError(jobName)
	}
	radixBatch, err := kubequery.GetRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, batchName, "")
	if err != nil {
		return err
	}
	if !slice.Any(radixBatch.Spec.Jobs, isRadixBatchJobWithName(batchJobName)) {
		return jobNotFoundError(batchJobName)
	}
	return jobschedulerbatch.StopRadixBatchJob(ctx, eh.accounts.UserAccount.RadixClient, radixBatch, batchJobName)
}

// DeleteBatch Delete batch by name
func (eh EnvironmentHandler) DeleteBatch(ctx context.Context, appName, envName, jobComponentName, batchName string) error {
	radixBatch, err := kubequery.GetRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return err
	}
	return jobschedulerbatch.DeleteRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, radixBatch)
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
	return jobschedulerbatch.DeleteRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, radixBatch)
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
	radixBatchJob, ok := slice.FindFirst(radixBatch.Spec.Jobs, isRadixBatchJobWithName(batchJobName))
	if !ok {
		return nil, jobNotFoundError(jobName)
	}
	if radixBatchJob.PayloadSecretRef == nil {
		return io.NopCloser(&bytes.Buffer{}), nil
	}
	namespace := operatorutils.GetEnvironmentNamespace(appName, envName)
	secret, err := eh.accounts.ServiceAccount.Client.CoreV1().Secrets(namespace).Get(ctx, radixBatchJob.PayloadSecretRef.Name, metav1.GetOptions{})
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return nil, environmentmodels.ScheduledJobPayloadNotFoundError(appName, jobName)
		}
		return nil, err
	}
	payload, ok := secret.Data[radixBatchJob.PayloadSecretRef.Key]
	if !ok {
		return nil, environmentmodels.ScheduledJobPayloadNotFoundError(appName, jobName)
	}
	return io.NopCloser(bytes.NewReader(payload)), nil
}

func jobNotFoundError(jobName string) error {
	return radixhttp.NotFoundError(fmt.Sprintf("job %s not found", jobName))
}

func parseBatchAndJobNameFromScheduledJobName(scheduleJobName string) (string, string, bool) {
	scheduleJobNameParts := strings.Split(scheduleJobName, "-")
	if len(scheduleJobNameParts) < 2 {
		return "", "", false
	}
	batchName := strings.Join(scheduleJobNameParts[:len(scheduleJobNameParts)-1], "-")
	batchJobName := scheduleJobNameParts[len(scheduleJobNameParts)-1]
	return batchName, batchJobName, true
}

func isScheduledJobSummaryWithName(jobName string) func(j deploymentmodels.ScheduledJobSummary) bool {
	return func(j deploymentmodels.ScheduledJobSummary) bool {
		return j.Name == jobName
	}
}

func isRadixBatchJobWithName(batchJobName string) func(j radixv1.RadixBatchJob) bool {
	return func(j radixv1.RadixBatchJob) bool {
		return j.Name == batchJobName
	}
}
