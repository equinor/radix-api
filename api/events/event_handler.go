package events

import (
	eventModels "github.com/equinor/radix-api/api/events/models"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	k8sObjectUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// EventHandler defines methods for interacting with Kubernetes events
type EventHandler interface {
	GetEvents(namespaceFunc NamespaceFunc) ([]*eventModels.Event, error)
}

// NamespaceFunc defines a function that returns a namespace
// Used as argument in GetEvents to filter events by namespace
type NamespaceFunc func() string

// RadixEnvironmentNamespace builds a namespace from a RadixApplication and environment name
func RadixEnvironmentNamespace(ra *v1.RadixApplication, envName string) NamespaceFunc {
	return func() string {
		return k8sObjectUtils.GetEnvironmentNamespace(ra.Name, envName)
	}
}

func NamespaceString(namespace string) NamespaceFunc {
	return func() string {
		return namespace
	}
}

type eventHandler struct {
	kubeClient kubernetes.Interface
}

// Init creates a new EventHandler
func Init(kubeClient kubernetes.Interface) EventHandler {
	return &eventHandler{kubeClient: kubeClient}
}

// GetEvents return events for a namespace defined by a NamespaceFunc function
func (eh *eventHandler) GetEvents(namespaceFunc NamespaceFunc) ([]*eventModels.Event, error) {
	namespace := namespaceFunc()
	return eh.getEvents(namespace)
}

func (eh *eventHandler) getEvents(namespace string) ([]*eventModels.Event, error) {
	k8sEvents, err := eh.kubeClient.CoreV1().Events(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	events := make([]*eventModels.Event, 0)
	for _, ev := range k8sEvents.Items {
		event := eventModels.NewEventBuilder().WithKubernetesEvent(ev).BuildEvent()
		events = append(events, event)
	}

	return events, nil
}
