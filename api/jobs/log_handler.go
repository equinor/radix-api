package jobs

import (
	"bytes"
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	jobModels "github.com/statoil/radix-api/api/jobs/models"
	crdUtils "github.com/statoil/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func PipelineNotFoundError(appName, jobName string) error {
	return fmt.Errorf("Job %s not found for app %s", jobName, appName)
}

// HandleGetApplicationJobLogs Gets logs for an job of an application
func HandleGetApplicationJobLogs(client kubernetes.Interface, appName, jobName string) ([]jobModels.PipelineStep, error) {
	steps := []jobModels.PipelineStep{}
	pipelinePod, err := getPipelinePod(client, appName, jobName)
	if err != nil {
		return steps, err
	}

	pipelineStep := getPipelineStepLog(client, pipelinePod)

	pods, err := getBuildPods(client, pipelinePod)
	if err != nil {
		// use clone from pipeline step
		cloneStep := getInitCloneStepLog(client, pipelinePod)
		steps = append(steps, cloneStep, pipelineStep, jobModels.PipelineStep{
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
			buildStep := getBuildStep(client, buildPod, initContainer.Name, 1)
			steps = append(steps, buildStep)
		}

		for _, container := range buildPod.Spec.Containers {
			buildStep := getBuildStep(client, buildPod, container.Name, 3)
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

func getInitCloneStepLog(client kubernetes.Interface, pipelinePod *corev1.Pod) jobModels.PipelineStep {
	cloneLog, err := handleGetPodLog(client, pipelinePod, "clone")
	if err != nil {
		cloneLog = "error: log not found"
	}
	return jobModels.PipelineStep{
		Name:    "clone",
		Log:     cloneLog,
		PodName: fmt.Sprintf("%s %s", pipelinePod.GetName(), "clone"),
		Sort:    1,
	}
}

func getPipelineStepLog(client kubernetes.Interface, pipelinePod *corev1.Pod) jobModels.PipelineStep {
	podLog, err := handleGetPodLog(client, pipelinePod, "")
	if err != nil {
		podLog = "error: pipeline log not found"
	}
	return jobModels.PipelineStep{
		Name:    "pipeline",
		Log:     podLog,
		PodName: pipelinePod.GetName(),
		Sort:    2,
	}
}

func getBuildStep(client kubernetes.Interface, buildPod corev1.Pod, containerName string, sort int32) jobModels.PipelineStep {
	buildLog, err := handleGetPodLog(client, &buildPod, containerName)
	if err != nil {
		log.Warnf("Failed to get build logs. %v", err)
		buildLog = fmt.Sprintf("%v", err)
	}
	return jobModels.PipelineStep{
		Name:    containerName,
		Log:     buildLog,
		PodName: buildPod.GetName(),
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

func handleGetPodLog(client kubernetes.Interface, pod *corev1.Pod, containerName string) (string, error) {
	req := getPodLogRequest(client, pod, containerName, false)

	readCloser, err := req.Stream()
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(readCloser)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func getPodLogRequest(client kubernetes.Interface, pod *corev1.Pod, containerName string, follow bool) *rest.Request {
	podLogOption := corev1.PodLogOptions{
		Follow: follow,
	}
	if containerName != "" {
		podLogOption.Container = containerName
	}

	req := client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOption)
	return req
}
