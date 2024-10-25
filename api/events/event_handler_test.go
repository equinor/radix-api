package events

import (
	"context"
	"testing"

	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

const (
	uid1 = "d1bf3ab3-0693-4291-a559-96a12ace9f33"
	uid2 = "2dcc9cf7-086d-49a0-abe1-ea3610594eb2"
	uid3 = "0c43a075-d174-479e-96b1-70183d67464c"
	uid4 = "805a5ffe-a2ca-4d1e-a178-7988ab1f03ca"
)

func setupTest() (*kubefake.Clientset, *radixfake.Clientset) {
	kubeClient := kubefake.NewSimpleClientset()
	radixClient := radixfake.NewSimpleClientset()
	return kubeClient, radixClient
}

func Test_EventHandler_Init(t *testing.T) {
	kubeClient, radixClient := setupTest()
	eh := Init(kubeClient, radixClient).(*eventHandler)
	assert.NotNil(t, eh)
	assert.Equal(t, kubeClient, eh.kubeClient)
}

func Test_EventHandler_GetEventsForRadixApplication(t *testing.T) {
	appName, envName := "app1", "env1"
	envNamespace := operatorutils.GetEnvironmentNamespace(appName, envName)
	kubeClient, radixClient := setupTest()

	createRadixAppWithEnvironment(t, radixClient, appName, envName)
	createKubernetesEvent(t, kubeClient, envNamespace, "ev1", "Normal", "pod1", "Pod", uid1)
	createKubernetesEvent(t, kubeClient, envNamespace, "ev2", "Normal", "pod2", "Pod", uid2)
	createKubernetesEvent(t, kubeClient, "app2-env", "ev3", "Normal", "pod3", "Pod", uid3)

	eventHandler := Init(kubeClient, radixClient)
	events, err := eventHandler.GetEnvironmentEvents(context.Background(), appName, envName)
	assert.Nil(t, err)
	assert.Len(t, events, 2)
	assert.ElementsMatch(
		t,
		[]string{"pod1", "pod2"},
		[]string{events[0].InvolvedObjectName, events[1].InvolvedObjectName},
	)
}

func Test_EventHandler_NoEventsWhenThereIsNoRadixApplication(t *testing.T) {
	appName, envName := "app1", "env1"
	envNamespace := operatorutils.GetEnvironmentNamespace(appName, envName)
	kubeClient, radixClient := setupTest()

	createKubernetesEvent(t, kubeClient, envNamespace, "ev1", "Normal", "pod1", "Pod", uid1)

	eventHandler := Init(kubeClient, radixClient)
	events, err := eventHandler.GetEnvironmentEvents(context.Background(), appName, envName)
	assert.NotNil(t, err)
	assert.Len(t, events, 0)
}

func Test_EventHandler_NoEventsWhenThereIsNoRadixEnvironment(t *testing.T) {
	appName, envName := "app1", "env1"
	envNamespace := operatorutils.GetEnvironmentNamespace(appName, envName)
	kubeClient, radixClient := setupTest()

	createRadixApp(t, radixClient, appName, envName)
	createKubernetesEvent(t, kubeClient, envNamespace, "ev1", "Normal", "pod1", "Pod", uid1)

	eventHandler := Init(kubeClient, radixClient)
	events, err := eventHandler.GetEnvironmentEvents(context.Background(), appName, envName)
	assert.NotNil(t, err)
	assert.Len(t, events, 0)
}

func Test_EventHandler_GetEvents_PodState(t *testing.T) {
	appName, envName := "app1", "env1"
	envNamespace := operatorutils.GetEnvironmentNamespace(appName, envName)

	t.Run("ObjectState is nil for normal event type", func(t *testing.T) {
		kubeClient, radixClient := setupTest()
		createRadixAppWithEnvironment(t, radixClient, appName, envName)
		_, err := createKubernetesPod(kubeClient, "pod1", appName, envName, true, true, 0, uid1)
		createKubernetesEvent(t, kubeClient, envNamespace, "ev1", "Normal", "pod1", "Pod", uid1)
		require.NoError(t, err)
		eventHandler := Init(kubeClient, radixClient)
		events, _ := eventHandler.GetEnvironmentEvents(context.Background(), appName, envName)
		assert.Len(t, events, 1)
		assert.Nil(t, events[0].InvolvedObjectState)
	})

	t.Run("ObjectState has Pod state for warning event type", func(t *testing.T) {
		kubeClient, radixClient := setupTest()
		createRadixAppWithEnvironment(t, radixClient, appName, envName)
		_, err := createKubernetesPod(kubeClient, "pod1", appName, envName, true, false, 0, uid1)
		createKubernetesEvent(t, kubeClient, envNamespace, "ev1", "Warning", "pod1", "Pod", uid1)
		require.NoError(t, err)
		eventHandler := Init(kubeClient, radixClient)
		events, _ := eventHandler.GetEnvironmentEvents(context.Background(), appName, envName)
		assert.Len(t, events, 1)
		assert.NotNil(t, events[0].InvolvedObjectState)
		assert.NotNil(t, events[0].InvolvedObjectState.Pod)
	})

	t.Run("ObjectState is nil for warning event type when pod not exist", func(t *testing.T) {
		kubeClient, radixClient := setupTest()
		createRadixAppWithEnvironment(t, radixClient, appName, envName)
		createKubernetesEvent(t, kubeClient, envNamespace, "ev1", "Normal", "pod1", "Pod", uid1)
		eventHandler := Init(kubeClient, radixClient)
		events, _ := eventHandler.GetEnvironmentEvents(context.Background(), appName, envName)
		assert.Len(t, events, 1)
		assert.Nil(t, events[0].InvolvedObjectState)
	})
}

func createKubernetesEvent(t *testing.T, client *kubefake.Clientset, namespace, name, eventType, involvedObjectName, involvedObjectKind, uid string) {
	_, err := client.CoreV1().Events(namespace).CreateWithEventNamespace(&corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      involvedObjectKind,
			Name:      involvedObjectName,
			Namespace: namespace,
			UID:       types.UID(uid),
		},
		Type: eventType,
	})
	require.NoError(t, err)
}

func createKubernetesPod(client *kubefake.Clientset, name, appName, envName string, started, ready bool, restartCount int32, uid string) (*corev1.Pod, error) {
	namespace := operatorutils.GetEnvironmentNamespace(appName, envName)
	return client.CoreV1().Pods(namespace).Create(context.Background(),
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				UID:    types.UID(uid),
				Labels: radixlabels.ForApplicationName(appName),
			},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Started: &started, Ready: ready, RestartCount: restartCount},
				},
			},
		},
		metav1.CreateOptions{})
}

func createRadixAppWithEnvironment(t *testing.T, radixClient *radixfake.Clientset, appName string, envName string) {
	createRadixApp(t, radixClient, appName, envName)
	re := operatorutils.NewEnvironmentBuilder().WithAppName(appName).WithEnvironmentName(envName).BuildRE()
	_, err := radixClient.RadixV1().RadixEnvironments().Create(context.Background(), re, metav1.CreateOptions{})
	require.NoError(t, err)
}

func createRadixApp(t *testing.T, radixClient *radixfake.Clientset, appName string, envName string) {
	_, err := radixClient.RadixV1().RadixApplications(operatorutils.GetAppNamespace(appName)).Create(context.Background(), operatorutils.NewRadixApplicationBuilder().WithAppName(appName).WithEnvironment(envName, "").BuildRA(), metav1.CreateOptions{})
	require.NoError(t, err)
}
