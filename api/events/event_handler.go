package events

import (
	"context"
	"fmt"
	"regexp"

	eventModels "github.com/equinor/radix-api/api/events/models"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	k8v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// EventHandler defines methods for interacting with Kubernetes events
type EventHandler interface {
	GetEvents(ctx context.Context, appName, envName string) ([]*eventModels.Event, error)
	GetComponentEvents(ctx context.Context, appName, envName, componentName string) ([]*eventModels.Event, error)
	GetPodEvents(ctx context.Context, appName, envName, componentName, podName string) ([]*eventModels.Event, error)
}

// NamespaceFunc defines a function that returns a namespace
// Used as argument in GetEvents to filter events by namespace
type NamespaceFunc func() string

type eventHandler struct {
	kubeClient kubernetes.Interface
}

// Init creates a new EventHandler
func Init(kubeClient kubernetes.Interface) EventHandler {
	return &eventHandler{kubeClient: kubeClient}
}

// GetEvents return events for a namespace defined by a namespace
func (eh *eventHandler) GetEvents(ctx context.Context, appName, envName string) ([]*eventModels.Event, error) {
	return eh.getEvents(ctx, appName, envName, "", "")
}

// GetComponentEvents return events for a namespace defined by a namespace for a specific component
func (eh *eventHandler) GetComponentEvents(ctx context.Context, appName, envName, componentName string) ([]*eventModels.Event, error) {
	return eh.getEvents(ctx, appName, envName, componentName, "")
}

// GetPodEvents return events for a namespace defined by a namespace for a specific pod of a component
func (eh *eventHandler) GetPodEvents(ctx context.Context, appName, envName, componentName, podName string) ([]*eventModels.Event, error) {
	return eh.getEvents(ctx, appName, envName, componentName, podName)
}

func (eh *eventHandler) getEvents(ctx context.Context, appName, envName, componentName, podName string) ([]*eventModels.Event, error) {
	namespace := utils.GetEnvironmentNamespace(appName, envName)
	k8sEvents, err := eh.kubeClient.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	events := make([]*eventModels.Event, 0)
	for _, ev := range k8sEvents.Items {
		if len(podName) > 0 && !eventIsRelatedToPod(ev, componentName, podName) {
			continue
		}
		if len(componentName) > 0 && !eventIsRelatedToComponent(ev, componentName) {
			continue
		}
		builder := eventModels.NewEventBuilder().WithKubernetesEvent(ev)
		buildObjectState(ctx, builder, ev, eh.kubeClient)
		event := builder.Build()
		events = append(events, event)
	}
	return events, nil
}

func eventIsRelatedToComponent(ev k8v1.Event, componentName string) bool {
	if matchingToDeployment(ev, componentName) || matchingToReplicaSet(ev, componentName, "") {
		return true
	}
	podNameRegex, err := regexp.Compile(fmt.Sprintf("^%s-[a-z0-9]{9,10}-[a-z0-9]{5}$", componentName))
	if err != nil {
		return false
	}
	if ev.InvolvedObject.Kind == "Pod" && podNameRegex.MatchString(ev.InvolvedObject.Name) {
		return true
	}
	return false
}

func matchingToDeployment(ev k8v1.Event, componentName string) bool {
	return ev.InvolvedObject.Kind == "Deployment" && ev.InvolvedObject.Name == componentName
}

func matchingToReplicaSet(ev k8v1.Event, componentName, podName string) bool {
	if ev.InvolvedObject.Kind != "ReplicaSet" {
		return false
	}
	if replicaSetNameRegex, err := regexp.Compile(fmt.Sprintf("^%s-[a-z0-9]{9,10}$", componentName)); err != nil || !replicaSetNameRegex.MatchString(ev.InvolvedObject.Name) {
		return false
	}
	if len(podName) == 0 || len(ev.Message) == 0 {
		return true
	}
	podNameRegex, err := regexp.Compile(fmt.Sprintf(`^[\w\s]*:\s%s$`, podName))
	if err != nil {
		return false
	}
	return podNameRegex.MatchString(ev.Message)
}

func eventIsRelatedToPod(ev k8v1.Event, componentName, podName string) bool {
	if ev.InvolvedObject.Kind == "Pod" && ev.InvolvedObject.Name == podName {
		return true
	}
	return matchingToDeployment(ev, componentName) || matchingToReplicaSet(ev, componentName, podName)
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
