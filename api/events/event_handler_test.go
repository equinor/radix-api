package events

import (
	"context"
	"testing"

	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func Test_EventHandler_Init(t *testing.T) {
	kubeClient := kubefake.NewSimpleClientset()
	eh := Init(kubeClient).(*eventHandler)
	assert.NotNil(t, eh)
	assert.Equal(t, kubeClient, eh.kubeClient)
}

func Test_EventHandler_RadixEnvironmentNamespace(t *testing.T) {
	appName, envName := "app", "env"
	expected := operatorutils.GetEnvironmentNamespace(appName, envName)
	ra := operatorutils.NewRadixApplicationBuilder().WithAppName(appName).BuildRA()
	actual := RadixEnvironmentNamespace(ra, envName)()
	assert.Equal(t, expected, actual)
}

func Test_EventHandler_GetEventsForRadixApplication(t *testing.T) {
	appName, envName := "app", "env"
	appNamespace := operatorutils.GetEnvironmentNamespace(appName, envName)
	kubeClient := kubefake.NewSimpleClientset()

	createKubernetesEvent(t, kubeClient, appNamespace, "ev1", "Normal", "pod1", "Pod")
	createKubernetesEvent(t, kubeClient, appNamespace, "ev2", "Normal", "pod2", "Pod")
	createKubernetesEvent(t, kubeClient, "app2-env", "ev3", "Normal", "pod3", "Pod")

	ra := operatorutils.NewRadixApplicationBuilder().WithAppName(appName).BuildRA()
	eventHandler := Init(kubeClient)
	events, err := eventHandler.GetEvents(context.Background(), RadixEnvironmentNamespace(ra, envName))
	assert.Nil(t, err)
	assert.Len(t, events, 2)
	assert.ElementsMatch(
		t,
		[]string{"pod1", "pod2"},
		[]string{events[0].InvolvedObjectName, events[1].InvolvedObjectName},
	)
}

func Test_EventHandler_GetEvents_PodState(t *testing.T) {
	appName, envName := "app", "env"
	appNamespace := operatorutils.GetEnvironmentNamespace(appName, envName)
	ra := operatorutils.NewRadixApplicationBuilder().WithAppName(appName).BuildRA()

	t.Run("ObjectState is nil for normal event type", func(t *testing.T) {
		kubeClient := kubefake.NewSimpleClientset()
		createKubernetesEvent(t, kubeClient, appNamespace, "ev1", "Normal", "pod1", "Pod")
		createKubernetesPod(kubeClient, "pod1", appNamespace, true, true, 0)
		eventHandler := Init(kubeClient)
		events, _ := eventHandler.GetEvents(context.Background(), RadixEnvironmentNamespace(ra, envName))
		assert.Len(t, events, 1)
		assert.Nil(t, events[0].InvolvedObjectState)
	})

	t.Run("ObjectState has Pod state for warning event type", func(t *testing.T) {
		kubeClient := kubefake.NewSimpleClientset()
		createKubernetesEvent(t, kubeClient, appNamespace, "ev1", "Warning", "pod1", "Pod")
		createKubernetesPod(kubeClient, "pod1", appNamespace, true, false, 0)
		eventHandler := Init(kubeClient)
		events, _ := eventHandler.GetEvents(context.Background(), RadixEnvironmentNamespace(ra, envName))
		assert.Len(t, events, 1)
		assert.NotNil(t, events[0].InvolvedObjectState)
		assert.NotNil(t, events[0].InvolvedObjectState.Pod)
	})

	t.Run("ObjectState is nil for warning event type when pod not exist", func(t *testing.T) {
		kubeClient := kubefake.NewSimpleClientset()
		createKubernetesEvent(t, kubeClient, appNamespace, "ev1", "Normal", "pod1", "Pod")
		eventHandler := Init(kubeClient)
		events, _ := eventHandler.GetEvents(context.Background(), RadixEnvironmentNamespace(ra, envName))
		assert.Len(t, events, 1)
		assert.Nil(t, events[0].InvolvedObjectState)
	})
}

func createKubernetesEvent(t *testing.T, client *kubefake.Clientset, namespace,
	name, eventType, involvedObjectName, involvedObjectKind string) {
	_, err := client.CoreV1().Events(namespace).CreateWithEventNamespace(&v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		InvolvedObject: v1.ObjectReference{
			Kind:      involvedObjectKind,
			Name:      involvedObjectName,
			Namespace: namespace,
		},
		Type: eventType,
	})
	require.NoError(t, err)
}

func createKubernetesPod(client *kubefake.Clientset, name, namespace string,
	started, ready bool,
	restartCount int32) {
	client.CoreV1().Pods(namespace).Create(context.Background(),
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Status: v1.PodStatus{
				ContainerStatuses: []v1.ContainerStatus{
					{Started: &started, Ready: ready, RestartCount: restartCount},
				},
			},
		},
		metav1.CreateOptions{})
}
