package jobs

import (
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	jobModels "github.com/statoil/radix-api/api/jobs/models"
	"github.com/statoil/radix-api/api/pods"
	crdUtils "github.com/statoil/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// PipelineNotFoundError Job not found
func PipelineNotFoundError(appName, jobName string) error {
	return fmt.Errorf("Job %s not found for app %s", jobName, appName)
}

// HandleGetApplicationJobLogs Gets logs for an job of an application
func (jh JobHandler) HandleGetApplicationJobLogs(appName, jobName string) ([]jobModels.StepLog, error) {
	steps := []jobModels.StepLog{}
	pipelinePod, err := getPipelinePod(jh.client, appName, jobName)
	if err != nil {
		return steps, err
	}

	pipelineStep := getPipelineStepLog(jh.client, appName, pipelinePod.GetName())

	pods, err := getBuildPods(jh.client, pipelinePod)
	if err != nil {
		// use clone from pipeline step
		cloneStep := getInitCloneStepLog(jh.client, appName, pipelinePod.GetName())
		steps = append(steps, cloneStep, pipelineStep, jobModels.StepLog{
			Name: "docker build",
			Log:  fmt.Sprintf("%v", err),
			Sort: 3,
		})
		return steps, nil
	}

	// use clone from build step
	steps = append(steps, pipelineStep)
	for _, buildPod := range pods.Items {
		for _, initContainer := range buildPod.Spec.InitContainers {
			buildStep := getBuildStepLog(jh.client, appName, buildPod.GetName(), initContainer.Name, 1)
			steps = append(steps, buildStep)
		}

		for _, container := range buildPod.Spec.Containers {
			buildStep := getBuildStepLog(jh.client, appName, buildPod.GetName(), container.Name, 3)
			steps = append(steps, buildStep)
		}
	}

	return steps, nil
}

func getPipelinePod(client kubernetes.Interface, appName, jobName string) (*corev1.Pod, error) {
	ns := crdUtils.GetAppNamespace(appName)
	pods, err := client.CoreV1().Pods(ns).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})

	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, PipelineNotFoundError(appName, jobName)
	}

	return &pods.Items[0], nil
}

func getBuildPods(client kubernetes.Interface, pipelinePod *corev1.Pod) (*corev1.PodList, error) {
	imageTag, err := getImageTag(pipelinePod.Spec.Containers[0].Args)
	if err != nil {
		log.Warnf("Error getting image tag: %v", err)
		return nil, fmt.Errorf("Error getting image tag: %v", err)
	}

	buildJobName := fmt.Sprintf("radix-builder-%s", imageTag)
	pods, err := client.CoreV1().Pods(pipelinePod.GetNamespace()).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", buildJobName),
	})
	if err != nil {
		log.Warnf("Error getting build pods. %v", err)
		return nil, fmt.Errorf("error: docker build logs not found")
	} else if len(pods.Items) <= 0 {
		return nil, fmt.Errorf("")
	}
	return pods, nil
}

func getInitCloneStepLog(client kubernetes.Interface, appName, podName string) jobModels.StepLog {
	podHandler := pods.Init(client)
	cloneLog, err := podHandler.HandleGetAppPodLog(appName, podName, "clone")

	if err != nil {
		cloneLog = "error: log not found"
	}
	return jobModels.StepLog{
		Name:    "clone",
		Log:     cloneLog,
		PodName: fmt.Sprintf("%s %s", podName, "clone"),
		Sort:    1,
	}
}

func getPipelineStepLog(client kubernetes.Interface, appName, podName string) jobModels.StepLog {
	podHandler := pods.Init(client)
	podLog, err := podHandler.HandleGetAppPodLog(appName, podName, "")
	if err != nil {
		podLog = "error: pipeline log not found"
	}
	return jobModels.StepLog{
		Name:    "job",
		Log:     podLog,
		PodName: podName,
		Sort:    2,
	}
}

func getBuildStepLog(client kubernetes.Interface, appName, podName, containerName string, sort int32) jobModels.StepLog {
	podHandler := pods.Init(client)
	buildLog, err := podHandler.HandleGetAppPodLog(appName, podName, containerName)
	if err != nil {
		log.Warnf("Failed to get build logs. %v", err)
		buildLog = fmt.Sprintf("%v", err)
	}
	return jobModels.StepLog{
		Name:    containerName,
		Log:     buildLog,
		PodName: podName,
		Sort:    sort,
	}
}

func getImageTag(args []string) (string, error) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "IMAGE_TAG=") {
			return strings.TrimPrefix(arg, "IMAGE_TAG="), nil
		}
	}
	return "", fmt.Errorf("IMAGE_TAG not found under args")
}
