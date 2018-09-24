package pod

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
)

// HandleGetPods handler for GetPods
func HandleGetPods(client kubernetes.Interface) ([]Pod, error) {
	podList, err := client.CoreV1().Pods(corev1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pods := make([]Pod, len(podList.Items))
	for i, pod := range podList.Items {
		pods[i] = Pod{Name: pod.Name}
	}

	return pods, nil
}
