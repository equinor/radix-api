package pods

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/equinor/radix-api/api/utils/labelselector"
	sortUtils "github.com/equinor/radix-api/api/utils/sort"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
func (ph PodHandler) HandleGetAppPodLog(appName, podName, containerName string, sinceTime *time.Time, logLines *int64) (io.ReadCloser, error) {
	appNs := crdUtils.GetAppNamespace(appName)
	return ph.getPodLog(appNs, podName, containerName, sinceTime, logLines)
}

// HandleGetEnvironmentPodLog Get logs from pod in environment
func (ph PodHandler) HandleGetEnvironmentPodLog(appName, envName, podName, containerName string, sinceTime *time.Time, logLines *int64) (io.ReadCloser, error) {
	envNs := crdUtils.GetEnvironmentNamespace(appName, envName)
	return ph.getPodLog(envNs, podName, containerName, sinceTime, logLines)
}

// HandleGetEnvironmentScheduledJobLog Get logs from scheduled job in environment
func (ph PodHandler) HandleGetEnvironmentScheduledJobLog(appName, envName, scheduledJobName, containerName string, sinceTime *time.Time, logLines *int64) (io.ReadCloser, error) {
	envNs := crdUtils.GetEnvironmentNamespace(appName, envName)
	return ph.getScheduledJobLog(envNs, scheduledJobName, containerName, sinceTime, logLines)
}

// HandleGetEnvironmentAuxiliaryResourcePodLog Get logs from auxiliary resource pod in environment
func (ph PodHandler) HandleGetEnvironmentAuxiliaryResourcePodLog(appName, envName, componentName, auxType, podName string, sinceTime *time.Time, logLines *int64) (io.ReadCloser, error) {
	envNs := crdUtils.GetEnvironmentNamespace(appName, envName)
	pods, err := ph.client.CoreV1().Pods(envNs).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelselector.ForAuxiliaryResource(appName, componentName, auxType).String(),
		FieldSelector: getPodNameFieldSelector(podName),
	})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, PodNotFoundError(podName)
	}
	return ph.getPodLog(envNs, podName, "", sinceTime, logLines)
}

func (ph PodHandler) getPodLog(namespace, podName, containerName string, sinceTime *time.Time, logLines *int64) (io.ReadCloser, error) {
	pod, err := ph.client.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return ph.getPodLogFor(pod, containerName, sinceTime, logLines)
}

func (ph PodHandler) getScheduledJobLog(namespace, scheduledJobName, containerName string, sinceTime *time.Time, logLines *int64) (io.ReadCloser, error) {
	pods, err := ph.client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", scheduledJobName),
	})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, PodNotFoundError(scheduledJobName)
	}

	sortUtils.Pods(pods.Items, sortUtils.ByPodCreationTimestamp, sortUtils.Descending)
	pod := &pods.Items[0]
	return ph.getPodLogFor(pod, containerName, sinceTime, logLines)
}

func (ph PodHandler) getPodLogFor(pod *corev1.Pod, containerName string, sinceTime *time.Time, logLines *int64) (io.ReadCloser, error) {
	req := getPodLogRequest(ph.client, pod, containerName, false, sinceTime, logLines)
	return req.Stream(context.TODO())
}

func getPodLogRequest(client kubernetes.Interface, pod *corev1.Pod, containerName string, follow bool, sinceTime *time.Time, logLines *int64) *rest.Request {
	podLogOption := corev1.PodLogOptions{
		Follow:    follow,
		TailLines: logLines,
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

func getPodNameFieldSelector(podName string) string {
	return fmt.Sprintf("metadata.name=%s", podName)
}
