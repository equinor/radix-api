package events

import (
	"context"
	"fmt"
	"regexp"

	eventModels "github.com/equinor/radix-api/api/events/models"
	"github.com/equinor/radix-api/api/kubequery"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sTypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const (
	k8sKindDeployment = "Deployment"
	k8sKindReplicaSet = "ReplicaSet"
	k8sKindPod        = "Pod"
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
	environmentComponentsPodMap, err := eh.getEnvironmentComponentsPodMap(ctx, appName, envName, err)
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
		event := eh.buildEvent(ev, environmentComponentsPodMap)
		events = append(events, event)
	}
	return events, nil
}

func (eh *eventHandler) getEnvironmentComponentsPodMap(ctx context.Context, appName string, envName string, err error) (map[k8sTypes.UID]*corev1.Pod, error) {
	componentPodList, err := kubequery.GetPodsForEnvironmentComponents(ctx, eh.kubeClient, appName, envName)
	if err != nil {
		return nil, err
	}
	podMap := slice.Reduce(componentPodList, make(map[k8sTypes.UID]*corev1.Pod), func(acc map[k8sTypes.UID]*corev1.Pod, pod corev1.Pod) map[k8sTypes.UID]*corev1.Pod {
		acc[pod.GetUID()] = &pod
		return acc
	})
	return podMap, nil
}

func (eh *eventHandler) buildEvent(ev corev1.Event, podMap map[k8sTypes.UID]*corev1.Pod) *eventModels.Event {
	builder := eventModels.NewEventBuilder().WithKubernetesEvent(ev)
	if ev.Type != "Normal" {
		if objectState := getObjectState(ev, podMap); objectState != nil {
			builder.WithInvolvedObjectState(objectState)
		}
	}
	return builder.Build()
}

func eventIsRelatedToComponent(ev corev1.Event, componentName string) bool {
	if matchingToDeployment(ev, componentName) || matchingToReplicaSet(ev, componentName, "") {
		return true
	}
	podNameRegex, err := regexp.Compile(fmt.Sprintf("^%s-[a-z0-9]{9,10}-[a-z0-9]{5}$", componentName))
	if err != nil {
		return false
	}
	if ev.InvolvedObject.Kind == k8sKindPod && podNameRegex.MatchString(ev.InvolvedObject.Name) {
		return true
	}
	return false
}

func matchingToDeployment(ev corev1.Event, componentName string) bool {
	return ev.InvolvedObject.Kind == k8sKindDeployment && ev.InvolvedObject.Name == componentName
}

func matchingToReplicaSet(ev corev1.Event, componentName, podName string) bool {
	if ev.InvolvedObject.Kind != k8sKindReplicaSet {
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

func eventIsRelatedToPod(ev corev1.Event, componentName, podName string) bool {
	if ev.InvolvedObject.Kind == k8sKindPod && ev.InvolvedObject.Name == podName {
		return true
	}
	return matchingToDeployment(ev, componentName) || matchingToReplicaSet(ev, componentName, podName)
}

func getObjectState(ev corev1.Event, podMap map[k8sTypes.UID]*corev1.Pod) *eventModels.ObjectState {
	builder := eventModels.NewObjectStateBuilder()
	obj := ev.InvolvedObject

	switch obj.Kind {
	case k8sKindPod:
		if pod, ok := podMap[ev.InvolvedObject.UID]; ok {
			state := getPodState(pod)
			builder.WithPodState(state)
			return builder.Build()
		}
	}
	return nil
}

func getPodState(pod *corev1.Pod) *eventModels.PodState {
	return eventModels.NewPodStateBuilder().
		WithPod(pod).
		Build()
}
