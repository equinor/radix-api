package pods

import (
	"bytes"
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// PodHandler Instance variables
type PodHandler struct {
	client kubernetes.Interface
}

// Init Constructor
func Init(client kubernetes.Interface) PodHandler {
	return PodHandler{client}
}

// HandleGetAppPodLog Get logs from pod in app namespace
func (ph PodHandler) HandleGetAppPodLog(appName, podName, containerName string, sinceTime *time.Time) (string, error) {
	appNs := crdUtils.GetAppNamespace(appName)
	return ph.getPodLog(appNs, podName, containerName, sinceTime)
}

// HandleGetEnvironmentPodLog Get logs from pod in environment
func (ph PodHandler) HandleGetEnvironmentPodLog(appName, envName, podName, containerName string, sinceTime *time.Time) (string, error) {
	envNs := crdUtils.GetEnvironmentNamespace(appName, envName)
	return ph.getPodLog(envNs, podName, containerName, sinceTime)
}

// HandleGetEnvironmentScheduledJobLog Get logs from scheduled job in environment
func (ph PodHandler) HandleGetEnvironmentScheduledJobLog(appName, envName, scheduledJobName, containerName string, sinceTime *time.Time) (string, error) {
	envNs := crdUtils.GetEnvironmentNamespace(appName, envName)
	return ph.getScheduledJobLog(envNs, scheduledJobName, containerName, sinceTime)
}

func (ph PodHandler) getPodLog(namespace, podName, containerName string, sinceTime *time.Time) (string, error) {
	pod, err := ph.client.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return ph.getPodLogFor(pod, containerName, sinceTime, err)
}

func (ph PodHandler) getScheduledJobLog(namespace, scheduledJobName, containerName string, sinceTime *time.Time) (string, error) {
	pods, err := ph.client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", scheduledJobName),
	})
	if err != nil {
		return "", err
	}
	if len(pods.Items) <= 0 {
		return "", nil
	}

	pod := &pods.Items[0]
	return ph.getPodLogFor(pod, containerName, sinceTime, err)
}

func (ph PodHandler) getPodLogFor(pod *corev1.Pod, containerName string, sinceTime *time.Time, err error) (string, error) {
	req := getPodLogRequest(ph.client, pod, containerName, false, sinceTime)
	readCloser, err := req.Stream(context.TODO())
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

func getPodLogRequest(client kubernetes.Interface, pod *corev1.Pod, containerName string, follow bool, sinceTime *time.Time) *rest.Request {
	podLogOption := corev1.PodLogOptions{
		Follow: follow,
	}

	if sinceTime != nil {
		podLogOption.SinceTime = &metav1.Time{
			Time: *sinceTime,
		}
	}

	if containerName != "" {
		podLogOption.Container = containerName
	}

	return client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOption)
}
