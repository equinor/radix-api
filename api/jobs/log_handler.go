package jobs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/pods"
	"github.com/equinor/radix-api/api/utils/tekton"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetApplicationJobLogs Gets logs for a job of an application
func (jh JobHandler) GetApplicationJobLogs(appName, jobName string, sinceTime *time.Time) ([]jobModels.StepLog, error) {
	job, err := jh.userAccount.RadixClient.RadixV1().RadixJobs(crdUtils.GetAppNamespace(appName)).Get(context.TODO(), jobName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil, jobModels.PipelineNotFoundError(appName, jobName)
	}
	if err != nil {
		return nil, err
	}

	steps := jobModels.GetJobStepsFromRadixJob(job)

	var logs []jobModels.StepLog
	for _, step := range steps {
		stepLog := getStepLog(jh.userAccount.Client, appName, step, sinceTime)
		logs = append(logs, stepLog)
	}
	return logs, nil
}

// GetTektonPipelineRunTaskLogs Get logs of a pipeline run task for a pipeline job
func (jh JobHandler) GetTektonPipelineRunTaskLogs(appName, jobName, pipelineRunName, taskName, stepName string, sinceTime *time.Time) (io.ReadCloser, error) {
	pipelineRun, err := tekton.GetPipelineRun(jh.userAccount.TektonClient, appName, jobName, pipelineRunName)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, jobModels.PipelineRunNotFoundError(appName, jobName, pipelineRunName)
		}
		return nil, err
	}
	podName, containerName, err := jh.getTaskPodAndContainerName(pipelineRun, taskName, stepName)
	if err != nil {
		return nil, err
	}
	podHandler := pods.Init(jh.userAccount.Client)
	if err != nil {
		return nil, err
	}
	return podHandler.HandleGetAppPodLog(appName, podName, containerName, sinceTime)
}

func (jh JobHandler) getTaskPodAndContainerName(pipelineRun *v1beta1.PipelineRun, taskRealName, stepName string) (string, string, error) {
	taskRealNameToNameMap := tekton.GetTaskRealNameToNameMap(pipelineRun)
	taskName, ok := taskRealNameToNameMap[taskRealName]
	if !ok {
		return "", "", fmt.Errorf("task %s is not executed", taskRealName)
	}

	var podName, containerName string
	for _, taskRun := range pipelineRun.Status.PipelineRunStatusFields.TaskRuns {
		if !strings.EqualFold(taskRun.PipelineTaskName, taskName) || taskRun.Status == nil {
			continue
		}
		podName = taskRun.Status.PodName
		for _, step := range taskRun.Status.TaskRunStatusFields.Steps {
			if !strings.EqualFold(step.Name, stepName) {
				continue
			}
			containerName = step.ContainerName
			break
		}
		break
	}
	if len(podName) == 0 || len(containerName) == 0 {
		return "", "", fmt.Errorf("missing task %s or step %s", taskName, stepName)
	}
	return podName, containerName, nil
}

func getStepLog(client kubernetes.Interface, appName string, step jobModels.Step, sinceTime *time.Time) jobModels.StepLog {
	var buildLog string
	podHandler := pods.Init(client)
	logReader, err := podHandler.HandleGetAppPodLog(appName, step.PodName, step.Name, sinceTime)

	if err != nil {
		log.Warnf("Failed to get build logs. %v", err)
		buildLog = fmt.Sprintf("%v", err)
	} else {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(logReader)
		buildLog = buf.String()
	}

	return jobModels.StepLog{
		Name:    step.Name,
		Log:     buildLog,
		PodName: step.PodName,
	}
}
