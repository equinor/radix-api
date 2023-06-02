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
func (ph PodHandler) HandleGetAppPodLog(ctx context.Context, appName, podName, containerName string, sinceTime *time.Time, logLines *int64) (io.ReadCloser, error) {
	appNs := crdUtils.GetAppNamespace(appName)
	return ph.getPodLog(ctx, appNs, podName, containerName, sinceTime, logLines, false)
}

// HandleGetEnvironmentPodLog Get logs from pod in environment
func (ph PodHandler) HandleGetEnvironmentPodLog(ctx context.Context, appName, envName, podName, containerName string, sinceTime *time.Time, logLines *int64, previousLog bool) (io.ReadCloser, error) {
	envNs := crdUtils.GetEnvironmentNamespace(appName, envName)
	return ph.getPodLog(ctx, envNs, podName, containerName, sinceTime, logLines, previousLog)
}

// HandleGetEnvironmentScheduledJobLog Get logs from scheduled job in environment
func (ph PodHandler) HandleGetEnvironmentScheduledJobLog(ctx context.Context, appName, envName, scheduledJobName, containerName string, sinceTime *time.Time, logLines *int64) (io.ReadCloser, error) {
	envNs := crdUtils.GetEnvironmentNamespace(appName, envName)
	return ph.getScheduledJobLog(ctx, envNs, scheduledJobName, containerName, sinceTime, logLines)
}

// HandleGetEnvironmentAuxiliaryResourcePodLog Get logs from auxiliary resource pod in environment
func (ph PodHandler) HandleGetEnvironmentAuxiliaryResourcePodLog(ctx context.Context, appName, envName, componentName, auxType, podName string, sinceTime *time.Time, logLines *int64) (io.ReadCloser, error) {
	envNs := crdUtils.GetEnvironmentNamespace(appName, envName)
	pods, err := ph.client.CoreV1().Pods(envNs).List(ctx, metav1.ListOptions{
		LabelSelector: labelselector.ForAuxiliaryResource(appName, componentName, auxType).String(),
		FieldSelector: getPodNameFieldSelector(podName),
	})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, PodNotFoundError(podName)
	}
	return ph.getPodLog(ctx, envNs, podName, "", sinceTime, logLines, false)
}

func (ph PodHandler) getPodLog(ctx context.Context, namespace, podName, containerName string, sinceTime *time.Time, logLines *int64, previousLog bool) (io.ReadCloser, error) {
	pod, err := ph.client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return ph.getPodLogFor(ctx, pod, containerName, sinceTime, logLines, previousLog)
}

func (ph PodHandler) getScheduledJobLog(ctx context.Context, namespace, scheduledJobName, containerName string, sinceTime *time.Time, logLines *int64) (io.ReadCloser, error) {
	pods, err := ph.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
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
	return ph.getPodLogFor(ctx, pod, containerName, sinceTime, logLines, false)
}

func (ph PodHandler) getPodLogFor(ctx context.Context, pod *corev1.Pod, containerName string, sinceTime *time.Time, logLines *int64, previousLog bool) (io.ReadCloser, error) {
	req := getPodLogRequest(ph.client, pod, containerName, false, sinceTime, logLines, previousLog)
	return req.Stream(ctx)
}

func getPodLogRequest(client kubernetes.Interface, pod *corev1.Pod, containerName string, follow bool, sinceTime *time.Time, logLines *int64, previousLog bool) *rest.Request {
	podLogOption := corev1.PodLogOptions{
		Follow:    follow,
		TailLines: logLines,
		Previous:  previousLog,
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
