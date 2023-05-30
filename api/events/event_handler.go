package events

import (
	"context"

	eventModels "github.com/equinor/radix-api/api/events/models"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	k8sObjectUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	k8v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// EventHandler defines methods for interacting with Kubernetes events
type EventHandler interface {
	GetEvents(ctx context.Context, namespaceFunc NamespaceFunc) ([]*eventModels.Event, error)
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

type eventHandler struct {
	kubeClient kubernetes.Interface
}

// Init creates a new EventHandler
func Init(kubeClient kubernetes.Interface) EventHandler {
	return &eventHandler{kubeClient: kubeClient}
}

// GetEvents return events for a namespace defined by a NamespaceFunc function
func (eh *eventHandler) GetEvents(ctx context.Context, namespaceFunc NamespaceFunc) ([]*eventModels.Event, error) {
	namespace := namespaceFunc()
	return eh.getEvents(ctx, namespace)
}

func (eh *eventHandler) getEvents(ctx context.Context, namespace string) ([]*eventModels.Event, error) {
	k8sEvents, err := eh.kubeClient.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	events := make([]*eventModels.Event, 0)
	for _, ev := range k8sEvents.Items {
		builder := eventModels.NewEventBuilder().WithKubernetesEvent(ev)
		buildObjectState(ctx, builder, ev, eh.kubeClient)
		event := builder.Build()
		events = append(events, event)
	}

	return events, nil
}

func buildObjectState(ctx context.Context, builder eventModels.EventBuilder, event k8v1.Event, kubeClient kubernetes.Interface) {
	if event.Type == "Normal" {
		return
	}

	if objectState := getObjectState(ctx, event, kubeClient); objectState != nil {
		builder.WithInvolvedObjectState(objectState)
	}
}

func getObjectState(ctx context.Context, event k8v1.Event, kubeClient kubernetes.Interface) *eventModels.ObjectState {
	builder := eventModels.NewObjectStateBuilder()
	build := false
	obj := event.InvolvedObject

	switch obj.Kind {
	case "Pod":
		if pod, err := kubeClient.CoreV1().Pods(obj.Namespace).Get(ctx, obj.Name, metav1.GetOptions{}); err == nil {
			state := getPodState(pod)
			builder.WithPodState(state)
			build = true
		}
	}

	if !build {
		return nil
	}

	return builder.Build()
}

func getPodState(pod *k8v1.Pod) *eventModels.PodState {
	return eventModels.NewPodStateBuilder().
		WithPod(pod).
		Build()
}
