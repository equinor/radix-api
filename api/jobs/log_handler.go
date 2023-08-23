package jobs

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/pods"
	"github.com/equinor/radix-api/api/utils/tekton"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetTektonPipelineRunTaskStepLogs Get logs of a pipeline run task for a pipeline job
func (jh JobHandler) GetTektonPipelineRunTaskStepLogs(ctx context.Context, appName, jobName, pipelineRunName, taskName, stepName string, sinceTime *time.Time, logLines *int64) (io.ReadCloser, error) {
	taskRunsMap, err := tekton.GetTektonPipelineTaskRuns(ctx, jh.userAccount.TektonClient, appName, jobName, pipelineRunName)
	if err != nil {
		return nil, err
	}
	podName, containerName, err := jh.getTaskPodAndContainerName(taskRunsMap, taskName, stepName)
	if err != nil {
		return nil, err
	}
	podHandler := pods.Init(jh.userAccount.Client)
	if err != nil {
		return nil, err
	}
	return podHandler.HandleGetAppPodLog(ctx, appName, podName, containerName, sinceTime, logLines)
}

func (jh JobHandler) getTaskPodAndContainerName(taskRunsMap map[string]*pipelinev1.TaskRun, taskRealName, stepName string) (string, string, error) {
	var podName, containerName string
	if taskRun, ok := taskRunsMap[taskRealName]; ok {
		podName = taskRun.Status.PodName
		for _, step := range taskRun.Status.TaskRunStatusFields.Steps {
			if !strings.EqualFold(step.Name, stepName) {
				continue
			}
			return podName, step.Container, nil
		}
		return "", "", fmt.Errorf("missing step %s in the task %s", stepName, taskRealName)
	}
	if len(podName) == 0 || len(containerName) == 0 {
		return "", "", fmt.Errorf("missing task %s or step %s", taskRealName, stepName)
	}
	return podName, containerName, nil
}

// GetPipelineJobStepLogs Get logs of a pipeline job step
func (jh JobHandler) GetPipelineJobStepLogs(ctx context.Context, appName, jobName, stepName string, sinceTime *time.Time, logLines *int64) (io.ReadCloser, error) {
	job, err := jh.userAccount.RadixClient.RadixV1().RadixJobs(crdUtils.GetAppNamespace(appName)).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, jobModels.PipelineNotFoundError(appName, jobName)
		}
		return nil, err
	}
	stepPodName := getPodNameForStep(job, stepName)
	if len(stepPodName) == 0 {
		return nil, jobModels.PipelineStepNotFoundError(appName, jobName, stepName)
	}

	podHandler := pods.Init(jh.userAccount.Client)
	logReader, err := podHandler.HandleGetAppPodLog(ctx, appName, stepPodName, stepName, sinceTime, logLines)
	if err != nil {
		log.Warnf("Failed to get build logs. %v", err)
		return nil, err
	}
	return logReader, nil
}

func getPodNameForStep(job *radixv1.RadixJob, stepName string) string {
	for _, jobStep := range job.Status.Steps {
		if strings.EqualFold(jobStep.Name, stepName) {
			return jobStep.PodName
		}
	}
	return ""
}
