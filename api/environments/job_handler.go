package environments

import (
	"context"
	"fmt"
	"io"
	"sort"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-api/api/utils"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	k8sJobNameLabel = "job-name" // A label that k8s automatically adds to a Pod created by a Job
)

// GetJobs Get jobs
func (eh EnvironmentHandler) GetJobs(appName, envName, jobComponentName string) ([]deploymentModels.ScheduledJobSummary, error) {
	var jobs []deploymentModels.ScheduledJobSummary

	// Backward compatibility: Get list of jobs not handled by RadixBatch
	// TODO: Remove when there are no legacy jobs left
	jh := legacyJobHandler{accounts: eh.accounts}
	legacyJobs, err := jh.GetJobs(appName, envName, jobComponentName)
	if err != nil {
		return nil, err
	}
	jobs = append(jobs, legacyJobs...)

	return jobs, nil
}

// GetJob Gets job by name
func (eh EnvironmentHandler) GetJob(appName, envName, jobComponentName, jobName string) (*deploymentModels.ScheduledJobSummary, error) {

	// Backward compatibility: Get job not handled by RadixBatch
	// TODO: Remove when there are no legacy jobs left
	jh := legacyJobHandler{accounts: eh.accounts}
	return jh.GetJob(appName, envName, jobComponentName, jobName)
}

// GetBatches Get batches
func (eh EnvironmentHandler) GetBatches(appName, envName, jobComponentName string) ([]deploymentModels.ScheduledBatchSummary, error) {

	radixBatches, err := eh.getRadixBatches(appName, envName, jobComponentName, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, err
	}
	summaries := eh.getScheduledBatchSummaryList(radixBatches)

	// Backward compatibility: Get list of batches not handled by RadixBatch
	// TODO: Remove when there are no legacy jobs left
	jh := legacyJobHandler{accounts: eh.accounts}
	legacyBatches, err := jh.GetBatches(appName, envName, jobComponentName)
	if err != nil {
		return nil, err
	}
	summaries = append(summaries, legacyBatches...)

	sort.Slice(summaries, func(i, j int) bool {
		return utils.IsBefore(&summaries[j], &summaries[i])
	})

	return summaries, nil
}

func (eh EnvironmentHandler) getRadixBatches(appName, envName, jobComponentName string, batchType kube.RadixBatchType) ([]radixv1.RadixBatch, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	selector := labels.Merge(
		labels.ForApplicationName(appName),
		labels.ForComponentName(jobComponentName),
		labels.ForBatchType(batchType),
	)

	batches, err := eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}

	return batches.Items, nil
}

// GetBatch Gets batch by name
func (eh EnvironmentHandler) GetBatch(appName, envName, jobComponentName, batchName string) (*deploymentModels.ScheduledBatchSummary, error) {

	// Backward compatibility: Get batch not handled by RadixBatch
	// TODO: Remove when there are no legacy jobs left
	jh := legacyJobHandler{accounts: eh.accounts}
	return jh.GetBatch(appName, envName, jobComponentName, batchName)
}

// GetJobPayload Gets job payload
func (eh EnvironmentHandler) GetJobPayload(appName, envName, jobComponentName, jobName string) (io.ReadCloser, error) {
	// Backward compatibility: Get batch not handled by RadixBatch
	// TODO: Remove when there are no legacy jobs left
	jh := legacyJobHandler{accounts: eh.accounts}
	return jh.GetJobPayload(appName, envName, jobComponentName, jobName)
}

func (eh EnvironmentHandler) getScheduledBatchSummaryList(batches []radixv1.RadixBatch) (summaries []deploymentModels.ScheduledBatchSummary) {
	for _, batch := range batches {
		summaries = append(summaries, eh.getScheduledBatchSummary(batch))
	}

	return summaries
}

func (eh EnvironmentHandler) getScheduledBatchSummary(batch radixv1.RadixBatch) deploymentModels.ScheduledBatchSummary {
	return deploymentModels.ScheduledBatchSummary{
		Name:          batch.Name,
		Status:        string(batch.Status.Condition.Type),
		TotalJobCount: len(batch.Spec.Jobs),
		Created:       radixutils.FormatTimestamp(batch.GetCreationTimestamp().Time),
		Started:       radixutils.FormatTime(batch.Status.Condition.ActiveTime),
		Ended:         radixutils.FormatTime(batch.Status.Condition.CompletionTime),
	}
}

// func (eh EnvironmentHandler) getScheduledJobSummary(batch radixv1.RadixBatch, jobName string) *deploymentModels.ScheduledJobSummary {
// 	creationTimestamp := job.GetCreationTimestamp()
// 	batchName := job.ObjectMeta.Labels[kube.RadixBatchNameLabel]
// 	summary := deploymentModels.ScheduledJobSummary{
// 		Name:      job.Name,
// 		Created:   radixutils.FormatTimestamp(creationTimestamp.Time),
// 		Started:   radixutils.FormatTime(job.Status.StartTime),
// 		BatchName: batchName,
// 		JobId:     "", // TODO: was job.ObjectMeta.Labels[kube.RadixJobIdLabel],
// 	}
// 	summary.TimeLimitSeconds = job.Spec.Template.Spec.ActiveDeadlineSeconds
// 	jobPods := jobPodsMap[job.Name]
// 	if len(jobPods) > 0 {
// 		summary.ReplicaList = h.getReplicaSummariesForPods(jobPods)
// 	}
// 	summary.Resources = h.getJobResourceRequirements(job, jobPods)
// 	summary.BackoffLimit = h.getJobBackoffLimit(job)
// 	jobStatus := GetJobStatusFromJob(h.accounts.UserAccount.Client, job, jobPodsMap[job.Name])
// 	summary.Status = jobStatus.Status
// 	summary.Message = jobStatus.Message
// 	summary.Ended = jobStatus.Ended
// 	return &summary
// }

func jobNotFoundError(jobName string) error {
	return radixhttp.NotFoundError(fmt.Sprintf("job %s not found", jobName))
}
