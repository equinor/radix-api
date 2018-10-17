package pods

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
)

// HandleGetPods handler for GetPods
func HandleGetPods(client kubernetes.Interface, appName string, envName string) ([]Pod, error) {
	podList, err := client.CoreV1().Pods(getNamespaceForApplicationEnvironment(appName, envName)).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pods := make([]Pod, len(podList.Items))
	for i, pod := range podList.Items {
		pods[i] = Pod{Name: pod.Name}
	}

	return pods, nil
}

// TODO : Separate out into library functions
func getNamespaceForApplicationEnvironment(appName, environment string) string {
	return fmt.Sprintf("%s-%s", appName, environment)
}
