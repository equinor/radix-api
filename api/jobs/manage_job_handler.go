package jobs

import (
	"context"
	"fmt"

	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/kubequery"
	"github.com/equinor/radix-common/utils/slice"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	jobConditionsInValidForJobStop = []v1.RadixJobCondition{v1.JobFailed, v1.JobStopped, v1.JobStoppedNoChanges}
	jobConditionsValidForJobRerun  = []v1.RadixJobCondition{v1.JobFailed, v1.JobStopped}
)

// StopJob Stops an application job
func (jh JobHandler) StopJob(ctx context.Context, appName, jobName string) error {
	log.Infof("Stopping the job: %s, %s", jobName, appName)
	radixJob, err := jh.getPipelineJob(ctx, appName, jobName)
	if err != nil {
		return err
	}
	if radixJob.Spec.Stop {
		return jobModels.JobAlreadyRequestedToStopError(appName, jobName)
	}
	if slice.Any(jobConditionsInValidForJobStop, func(condition v1.RadixJobCondition) bool { return condition == radixJob.Status.Condition }) {
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
	radixJob, err := jh.getPipelineJob(ctx, appName, jobName)
	if err != nil {
		return err
	}
	if !slice.Any(jobConditionsValidForJobRerun, func(condition v1.RadixJobCondition) bool { return condition == radixJob.Status.Condition }) {
		return jobModels.JobHasInvalidConditionToRerunError(appName, jobName, radixJob.Status.Condition)
	}

	copiedRadixJob := jh.buildPipelineJobRerunFrom(radixJob)
	_, err = jh.createPipelineJob(ctx, appName, copiedRadixJob)
	if err != nil {
		return fmt.Errorf("failed to create a job %s to rerun: %v", radixJob.GetName(), err)
	}

	log.Infof("reran the job %s as a new job %s in the application %s", radixJob.GetName(), copiedRadixJob.GetName(), appName)
	return nil
}

func (jh JobHandler) buildPipelineJobRerunFrom(srcRadixJob *v1.RadixJob) *v1.RadixJob {
	destJobName, imageTag := getUniqueJobName(workerImage)
	destRadixJob := v1.RadixJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:        destJobName,
			Labels:      srcRadixJob.Labels,
			Annotations: srcRadixJob.Annotations,
		},
		Spec: srcRadixJob.Spec,
	}
	if destRadixJob.ObjectMeta.Annotations == nil {
		destRadixJob.ObjectMeta.Annotations = make(map[string]string)
	}
	destRadixJob.ObjectMeta.Annotations[jobModels.RadixPipelineJobRerunAnnotation] = srcRadixJob.GetName()
	if len(destRadixJob.Spec.Build.ImageTag) > 0 {
		destRadixJob.Spec.Build.ImageTag = imageTag
	}
	destRadixJob.Spec.Stop = false
	triggeredBy, err := jh.getTriggeredBy("")
	if err != nil {
		log.Warnf("failed to get triggeredBy: %v", err)
	}
	destRadixJob.Spec.TriggeredBy = triggeredBy
	return &destRadixJob
}

func (jh JobHandler) getPipelineJob(ctx context.Context, appName string, jobName string) (*v1.RadixJob, error) {
	job, err := kubequery.GetRadixJob(ctx, jh.serviceAccount.RadixClient, appName, jobName)
	if err == nil {
		return job, nil
	}
	if errors.IsNotFound(err) {
		return nil, jobModels.PipelineNotFoundError(appName, jobName)
	}
	return nil, err
}
