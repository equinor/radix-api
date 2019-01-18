package pods

import (
	"bytes"

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
func (ph PodHandler) HandleGetAppPodLog(appName, podName, containerName string) (string, error) {
	appNs := crdUtils.GetAppNamespace(appName)
	return ph.getPodLog(appNs, podName, containerName)
}

// HandleGetEnvironmentPodLog Get logs from pod in environment
func (ph PodHandler) HandleGetEnvironmentPodLog(appName, envName, podName, containerName string) (string, error) {
	envNs := crdUtils.GetEnvironmentNamespace(appName, envName)
	return ph.getPodLog(envNs, podName, containerName)
}

func (ph PodHandler) getPodLog(namespace, podName, containerName string) (string, error) {
	pod, err := ph.client.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	req := getPodLogRequest(ph.client, pod, containerName, false)
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
