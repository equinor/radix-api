package jobs

import (
	"context"
	"fmt"

	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/kubequery"
	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	jobConditionsNotValidForJobStop = []radixv1.RadixJobCondition{radixv1.JobFailed, radixv1.JobStopped, radixv1.JobStoppedNoChanges}
	jobConditionsValidForJobRerun   = []radixv1.RadixJobCondition{radixv1.JobFailed, radixv1.JobStopped}
)

// StopJob Stops an application job
func (jh JobHandler) StopJob(ctx context.Context, appName, jobName string) error {
	log.Infof("Stopping the job: %s, %s", jobName, appName)
	radixJob, err := jh.getPipelineJobByName(ctx, appName, jobName)
	if err != nil {
		return err
	}
	if radixJob.Spec.Stop {
		return jobModels.JobAlreadyRequestedToStopError(appName, jobName)
	}
	if slice.Any(jobConditionsNotValidForJobStop, func(condition radixv1.RadixJobCondition) bool { return condition == radixJob.Status.Condition }) {
		return jobModels.JobHasInvalidConditionToStopError(appName, jobName, radixJob.Status.Condition)
	}

	radixJob.Spec.Stop = true

	_, err = jh.userAccount.RadixClient.RadixV1().RadixJobs(radixJob.GetNamespace()).Update(ctx, radixJob, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch job object: %v", err)
	}
	return nil
}

// RerunJob Reruns the pipeline job as a copy
func (jh JobHandler) RerunJob(ctx context.Context, appName, jobName string) error {
	log.Infof("Rerunning the job %s in the application %s", jobName, appName)
	radixJob, err := jh.getPipelineJobByName(ctx, appName, jobName)
	if err != nil {
		return err
	}
	if !slice.Any(jobConditionsValidForJobRerun, func(condition radixv1.RadixJobCondition) bool { return condition == radixJob.Status.Condition }) {
		return jobModels.JobHasInvalidConditionToRerunError(appName, jobName, radixJob.Status.Condition)
	}

	copiedRadixJob := jh.buildPipelineJobToRerunFrom(radixJob)
	_, err = jh.createPipelineJob(ctx, appName, copiedRadixJob)
	if err != nil {
		return fmt.Errorf("failed to create a job %s to rerun: %v", radixJob.GetName(), err)
	}

	log.Infof("reran the job %s as a new job %s in the application %s", radixJob.GetName(), copiedRadixJob.GetName(), appName)
	return nil
}

func (jh JobHandler) buildPipelineJobToRerunFrom(radixJob *radixv1.RadixJob) *radixv1.RadixJob {
	rerunJobName, imageTag := getUniqueJobName(workerImage)
	rerunRadixJob := radixv1.RadixJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:        rerunJobName,
			Labels:      radixJob.Labels,
			Annotations: radixJob.Annotations,
		},
		Spec: radixJob.Spec,
	}
	if rerunRadixJob.ObjectMeta.Annotations == nil {
		rerunRadixJob.ObjectMeta.Annotations = make(map[string]string)
	}
	rerunRadixJob.ObjectMeta.Annotations[jobModels.RadixPipelineJobRerunAnnotation] = radixJob.GetName()
	if len(rerunRadixJob.Spec.Build.ImageTag) > 0 {
		rerunRadixJob.Spec.Build.ImageTag = imageTag
	}
	rerunRadixJob.Spec.Stop = false
	triggeredBy, err := jh.getTriggeredBy("")
	if err != nil {
		log.Warnf("failed to get triggeredBy: %v", err)
	}
	rerunRadixJob.Spec.TriggeredBy = triggeredBy
	return &rerunRadixJob
}

func (jh JobHandler) getPipelineJobByName(ctx context.Context, appName string, jobName string) (*radixv1.RadixJob, error) {
	radixJob, err := kubequery.GetRadixJob(ctx, jh.userAccount.RadixClient, appName, jobName)
	if err == nil {
		return radixJob, nil
	}
	if errors.IsNotFound(err) {
		return nil, jobModels.PipelineNotFoundError(appName, jobName)
	}
	return nil, err
}
