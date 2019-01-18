package jobs

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/pods"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// HandleGetApplicationJobLogs Gets logs for an job of an application
func (jh JobHandler) HandleGetApplicationJobLogs(appName, jobName string) ([]jobModels.StepLog, error) {
	job, err := jh.client.BatchV1().Jobs(crdUtils.GetAppNamespace(appName)).Get(jobName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil, jobModels.PipelineNotFoundError(appName, jobName)
	}
	if err != nil {
		return nil, err
	}

	steps, err := jh.getJobSteps(appName, job)
	if err != nil {
		return nil, err
	}

	logs := []jobModels.StepLog{}
	for _, step := range steps {
		log := getStepLog(jh.client, appName, step)
		logs = append(logs, log)
	}
	return logs, nil
}

func getStepLog(client kubernetes.Interface, appName string, step jobModels.Step) jobModels.StepLog {
	podHandler := pods.Init(client)
	buildLog, err := podHandler.HandleGetAppPodLog(appName, step.PodName, step.Name)
	if err != nil {
		log.Warnf("Failed to get build logs. %v", err)
		buildLog = fmt.Sprintf("%v", err)
	}
	return jobModels.StepLog{
		Name:    step.Name,
		Log:     buildLog,
		PodName: step.PodName,
		Sort:    step.Sort,
	}
}
