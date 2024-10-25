package events

import (
	"context"
	"testing"

	eventModels "github.com/equinor/radix-api/api/events/models"
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
	ev1               = "ev1"
	ev2               = "ev2"
	ev3               = "ev3"
	uid1              = "d1bf3ab3-0693-4291-a559-96a12ace9f33"
	uid2              = "2dcc9cf7-086d-49a0-abe1-ea3610594eb2"
	uid3              = "0c43a075-d174-479e-96b1-70183d67464c"
	deploy1           = "server1"
	deploy2           = "server2"
	deploy3           = "server3"
	replicaSetServer1 = "server2-795977897d"
	replicaSetServer2 = "server2-795977897d"
	replicaSetServer3 = "server3-5c97b4c698"
	podServer1        = "server1-5bf67cf976-9v333"
	podServer2        = "server2-795977897d-m2sw8"
	podServer3        = "server3-5c97b4c698-6x4cg"
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
	createKubernetesEvent(t, kubeClient, envNamespace, ev1, k8sEventTypeNormal, podServer1, k8sKindPod, uid1)
	createKubernetesEvent(t, kubeClient, envNamespace, ev2, k8sEventTypeNormal, podServer2, k8sKindPod, uid2)
	createKubernetesEvent(t, kubeClient, "app2-env", ev3, k8sEventTypeNormal, podServer3, k8sKindPod, uid3)

	eventHandler := Init(kubeClient, radixClient)
	events, err := eventHandler.GetEnvironmentEvents(context.Background(), appName, envName)
	assert.Nil(t, err)
	assert.Len(t, events, 2)
	assert.ElementsMatch(
		t,
		[]string{podServer1, podServer2},
		[]string{events[0].InvolvedObjectName, events[1].InvolvedObjectName},
	)
}

func Test_EventHandler_NoEventsWhenThereIsNoRadixApplication(t *testing.T) {
	appName, envName := "app1", "env1"
	envNamespace := operatorutils.GetEnvironmentNamespace(appName, envName)
	kubeClient, radixClient := setupTest()

	createKubernetesEvent(t, kubeClient, envNamespace, ev1, k8sEventTypeNormal, podServer1, k8sKindPod, uid1)

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
	createKubernetesEvent(t, kubeClient, envNamespace, ev1, k8sEventTypeNormal, podServer1, k8sKindPod, uid1)

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
		_, err := createKubernetesPod(kubeClient, podServer1, appName, envName, true, true, 0, uid1)
		createKubernetesEvent(t, kubeClient, envNamespace, ev1, k8sEventTypeNormal, podServer1, k8sKindPod, uid1)
		require.NoError(t, err)
		eventHandler := Init(kubeClient, radixClient)
		events, _ := eventHandler.GetEnvironmentEvents(context.Background(), appName, envName)
		assert.Len(t, events, 1)
		assert.Nil(t, events[0].InvolvedObjectState)
	})

	t.Run("ObjectState has Pod state for warning event type", func(t *testing.T) {
		kubeClient, radixClient := setupTest()
		createRadixAppWithEnvironment(t, radixClient, appName, envName)
		_, err := createKubernetesPod(kubeClient, podServer1, appName, envName, true, false, 0, uid1)
		createKubernetesEvent(t, kubeClient, envNamespace, ev1, k8sEventTypeWarning, podServer1, k8sKindPod, uid1)
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
		createKubernetesEvent(t, kubeClient, envNamespace, ev1, k8sEventTypeNormal, podServer1, k8sKindPod, uid1)
		eventHandler := Init(kubeClient, radixClient)
		events, _ := eventHandler.GetEnvironmentEvents(context.Background(), appName, envName)
		assert.Len(t, events, 1)
		assert.Nil(t, events[0].InvolvedObjectState)
	})
}

type scenario struct {
	name               string
	existingEventProps []eventProps
	expectedEvents     []eventModels.Event
}

type eventProps struct {
	name       string
	namespace  string
	eventType  string
	objectName string
	objectUid  string
	objectKind string
}

func Test_EventHandler_GetEnvironmentEvents(t *testing.T) {
	appName, envName := "app1", "env1"
	envNamespace := operatorutils.GetEnvironmentNamespace(appName, envName)

	scenarios := []scenario{
		{name: "NoEvents"},
		{
			name: "Pod events",
			existingEventProps: []eventProps{
				{name: ev1, namespace: envNamespace, eventType: k8sEventTypeNormal, objectName: podServer1, objectKind: k8sKindPod, objectUid: uid1},
				{name: ev2, namespace: envNamespace, eventType: k8sEventTypeNormal, objectName: podServer2, objectKind: k8sKindPod, objectUid: uid2},
				{name: ev3, namespace: "app2-env", eventType: k8sEventTypeNormal, objectName: podServer3, objectKind: k8sKindPod, objectUid: uid3},
			},
			expectedEvents: []eventModels.Event{
				{InvolvedObjectName: podServer1, InvolvedObjectKind: k8sKindPod, InvolvedObjectNamespace: envNamespace},
				{InvolvedObjectName: podServer2, InvolvedObjectKind: k8sKindPod, InvolvedObjectNamespace: envNamespace},
			},
		},
		{
			name: "Deploy events",
			existingEventProps: []eventProps{
				{name: ev1, namespace: envNamespace, eventType: k8sEventTypeNormal, objectName: deploy1, objectKind: k8sKindDeployment, objectUid: uid1},
				{name: ev2, namespace: envNamespace, eventType: k8sEventTypeNormal, objectName: deploy2, objectKind: k8sKindDeployment, objectUid: uid2},
				{name: ev3, namespace: "app2-env", eventType: k8sEventTypeNormal, objectName: deploy3, objectKind: k8sKindDeployment, objectUid: uid3},
			},
			expectedEvents: []eventModels.Event{
				{InvolvedObjectName: deploy1, InvolvedObjectKind: k8sKindDeployment, InvolvedObjectNamespace: envNamespace},
				{InvolvedObjectName: deploy2, InvolvedObjectKind: k8sKindDeployment, InvolvedObjectNamespace: envNamespace},
			},
		},
	}
	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			kubeClient, radixClient := setupTest()
			createRadixAppWithEnvironment(t, radixClient, appName, envName)
			for _, evProps := range ts.existingEventProps {
				createKubernetesEvent(t, kubeClient, evProps.namespace, evProps.name, evProps.eventType, evProps.objectName, evProps.objectKind, evProps.objectUid)
			}

			eventHandler := Init(kubeClient, radixClient)
			events, err := eventHandler.GetEnvironmentEvents(context.Background(), appName, envName)
			assert.Nil(t, err)
			if assert.Len(t, events, len(ts.expectedEvents)) {
				for i := 0; i < len(ts.expectedEvents); i++ {
					assert.Equal(t, ts.expectedEvents[i].InvolvedObjectName, events[i].InvolvedObjectName)
					assert.Equal(t, ts.expectedEvents[i].InvolvedObjectKind, events[i].InvolvedObjectKind)
					assert.Equal(t, ts.expectedEvents[i].InvolvedObjectNamespace, events[i].InvolvedObjectNamespace)
				}
			}
		})
	}
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
