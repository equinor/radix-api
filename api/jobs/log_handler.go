package jobs

import (
	"fmt"
	"strings"

	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/pods"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// HandleGetApplicationJobLogs Gets logs for an job of an application
func (jh JobHandler) HandleGetApplicationJobLogs(appName, jobName string) ([]jobModels.StepLog, error) {
	job, err := jh.userAccount.RadixClient.RadixV1().RadixJobs(crdUtils.GetAppNamespace(appName)).Get(jobName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil, jobModels.PipelineNotFoundError(appName, jobName)
	}
	if err != nil {
		return nil, err
	}

	steps := jobModels.GetJobStepsFromRadixJob(job)

	logs := []jobModels.StepLog{}
	for _, step := range steps {
		log := getStepLog(jh.userAccount.Client, appName, step)
		logs = append(logs, log)
	}
	return logs, nil
}

func getStepLog(client kubernetes.Interface, appName string, step jobModels.Step) jobModels.StepLog {
	podHandler := pods.Init(client)
	containerLog, err := podHandler.HandleGetAppContainerPodLog(appName, step.PodName, step.Name)
	if err != nil {
		log.Warnf("Failed to get container logs. %v", err)
		containerLog = fmt.Sprintf("%v", err)
	}

	podLog, err := podHandler.HandleGetAppPodLog(appName, step.PodName)
	if err != nil {
		log.Warnf("Failed to get pod logs. %v", err)
		podLog = fmt.Sprintf("%v", err)
	}

	if !strings.EqualFold(containerLog, podLog) {
		containerLog = fmt.Sprintf("%s\n%s", containerLog, podLog)
	}

	return jobModels.StepLog{
		Name:    step.Name,
		Log:     containerLog,
		PodName: step.PodName,
	}
}
