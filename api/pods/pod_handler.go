package pods

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	podModels "github.com/statoil/radix-api/api/pods/models"
	"k8s.io/client-go/kubernetes"
)

// HandleGetPods handler for GetPods
func HandleGetPods(client kubernetes.Interface, appName string, envName string) ([]podModels.Pod, error) {
	podList, err := client.CoreV1().Pods(getNamespaceForApplicationEnvironment(appName, envName)).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pods := make([]podModels.Pod, len(podList.Items))
	for i, pod := range podList.Items {
		pods[i] = podModels.Pod{Name: pod.Name}
	}

	return pods, nil
}

// TODO : Separate out into library functions
func getNamespaceForApplicationEnvironment(appName, environment string) string {
	return fmt.Sprintf("%s-%s", appName, environment)
}
