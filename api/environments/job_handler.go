package environments

import (
	"bytes"
	"context"
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
	"github.com/equinor/radix-api/api/utils/predicate"
	radixhttp "github.com/equinor/radix-common/net/http"
	"github.com/equinor/radix-common/utils/pointers"
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
	batchSummaries := models.BuildScheduledBatchSummaryList(radixBatches, envRdList)

	jobSummaries := slices.Concat(slice.Map(batchSummaries, func(v deploymentmodels.ScheduledBatchSummary) []deploymentmodels.ScheduledJobSummary {
		return v.JobList
	})...)
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
	radixBatchJob, err := findJobInRadixBatch(radixBatch, batchJobName)
	if err != nil {
		return nil, jobNotFoundError(batchJobName)
	}

	envRdList, err := kubequery.GetRadixDeploymentsForEnvironment(ctx, eh.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return nil, err
	}
	var activeRd, batchRd *radixv1.RadixDeployment
	if rd, ok := slice.FindFirst(envRdList, predicate.IsActiveRadixDeployment); ok {
		activeRd = &rd
	}
	if rd, ok := slice.FindFirst(envRdList, predicate.IsRadixDeploymentForRadixBatch(radixBatch)); ok {
		batchRd = &rd
	}

	jobSummary := models.BuildScheduleJobSummary(radixBatch, radixBatchJob, batchRd, activeRd)
	return jobSummary, nil
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

	batchSummaryList := models.BuildScheduledBatchSummaryList(radixBatches, envRdList)
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
	var activeRd, batchRd *radixv1.RadixDeployment
	if rd, ok := slice.FindFirst(envRdList, predicate.IsActiveRadixDeployment); ok {
		activeRd = &rd
	}
	if rd, ok := slice.FindFirst(envRdList, predicate.IsRadixDeploymentForRadixBatch(radixBatch)); ok {
		batchRd = &rd
	}

	batchSummary := models.BuildScheduledBatchSummary(radixBatch, batchRd, activeRd)
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
	radixBatch, batchJobName, err := eh.getBatchJob(ctx, appName, envName, jobComponentName, jobName)
	if err != nil {
		return err
	}
	if _, err = findJobInRadixBatch(radixBatch, batchJobName); err != nil {
		return err
	}
	return jobschedulerbatch.RestartRadixBatchJob(ctx, eh.accounts.UserAccount.RadixClient, radixBatch, batchJobName)
}

// CopyBatch Copy batch by name
func (eh EnvironmentHandler) CopyBatch(ctx context.Context, appName, envName, jobComponentName, batchName string, scheduledBatchRequest environmentmodels.ScheduledBatchRequest) (*deploymentmodels.ScheduledBatchSummary, error) {
	radixBatch, err := kubequery.GetRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, err
	}
	_, activeDeployJobComponent, batchDeployJobComponent, err := eh.getDeploymentMapAndDeployJobComponents(ctx, appName, envName, jobComponentName, radixBatch)
	if err != nil {
		return nil, err
	}
	rb, err := jobschedulerbatch.CopyRadixBatchOrJob(ctx, eh.accounts.UserAccount.RadixClient, radixBatch, "", scheduledBatchRequest.DeploymentName)
	if err != nil {
		return nil, err
	}

	summary := models.BuildScheduledBatchSummary(rb, nil, nil)
	return summary, nil
}

// CopyJob Copy job by name
func (eh EnvironmentHandler) CopyJob(ctx context.Context, appName, envName, jobComponentName, jobName string, scheduledJobRequest environmentmodels.ScheduledJobRequest) (*deploymentmodels.ScheduledJobSummary, error) {
	radixBatch, batchJobName, err := eh.getBatchJob(ctx, appName, envName, jobComponentName, jobName)
	if err != nil {
		return nil, err
	}
	_, activeDeployJobComponent, batchDeployJobComponent, err := eh.getDeploymentMapAndDeployJobComponents(ctx, appName, envName, jobComponentName, radixBatch)
	if err != nil {
		return nil, err
	}
	radixBatchStatus, err := jobschedulerbatch.CopyRadixBatchOrJob(ctx, eh.accounts.UserAccount.RadixClient, radixBatch, batchJobName, activeDeployJobComponent, scheduledJobRequest.DeploymentName)
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
	return jobschedulerbatch.StopRadixBatch(ctx, eh.accounts.UserAccount.RadixClient, radixBatch)
}

// StopJob Stop job by name
func (eh EnvironmentHandler) StopJob(ctx context.Context, appName, envName, jobComponentName, jobName string) error {
	radixBatch, batchJobName, err := eh.getBatchJob(ctx, appName, envName, jobComponentName, jobName)
	if err != nil {
		return err
	}
	if _, err = findJobInRadixBatch(radixBatch, batchJobName); err != nil {
		return err
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
	radixBatchJobs := slice.FindAll(radixBatch.Spec.Jobs, func(job radixv1.RadixBatchJob) bool { return job.Name == batchJobName })
	if len(radixBatchJobs) == 0 {
		return nil, jobNotFoundError(jobName)
	}
	radixBatchJob := radixBatchJobs[0]
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

func findJobInRadixBatch(radixBatch *radixv1.RadixBatch, batchJobName string) (*radixv1.RadixBatchJob, error) {
	if job, ok := slice.FindFirst(radixBatch.Spec.Jobs, func(job radixv1.RadixBatchJob) bool { return job.Name == batchJobName }); ok {
		return &job, nil
	}
	return nil, jobNotFoundError(batchJobName)
}

func (eh EnvironmentHandler) getDeploymentMapAndDeployJobComponents(ctx context.Context, appName string, envName string, jobComponentName string, radixBatch *radixv1.RadixBatch) (map[string]radixv1.RadixDeployment, *radixv1.RadixDeployJobComponent, *radixv1.RadixDeployJobComponent, error) {
	radixDeploymentsMap, activeDeployJobComponent, err := eh.getDeploymentMapAndActiveDeployJobComponent(ctx, appName, envName, jobComponentName)
	if err != nil {
		return nil, nil, nil, err
	}
	batchDeployJobComponent := models.GetBatchDeployJobComponent(radixBatch.Spec.RadixDeploymentJobRef.Name, jobComponentName, radixDeploymentsMap)
	return radixDeploymentsMap, activeDeployJobComponent, batchDeployJobComponent, nil
}

func (eh EnvironmentHandler) getDeploymentMapAndActiveDeployJobComponent(ctx context.Context, appName string, envName string, jobComponentName string) (map[string]radixv1.RadixDeployment, *radixv1.RadixDeployJobComponent, error) {
	rdList, err := kubequery.GetRadixDeploymentsForEnvironment(ctx, eh.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return nil, nil, err
	}
	rdMap := slice.Reduce(rdList, make(map[string]radixv1.RadixDeployment), func(acc map[string]radixv1.RadixDeployment, rd radixv1.RadixDeployment) map[string]radixv1.RadixDeployment {
		acc[rd.Name] = rd
		return acc
	})
	activeRadixDeployJobComponent, err := getActiveDeployJobComponent(appName, envName, jobComponentName, rdMap)
	if err != nil {
		return nil, nil, err
	}
	return rdMap, activeRadixDeployJobComponent, nil
}
