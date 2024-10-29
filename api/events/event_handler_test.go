package events

import (
	"context"
	"testing"

	eventModels "github.com/equinor/radix-api/api/events/models"
	"github.com/equinor/radix-common/utils/pointers"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

const (
	appName           = "app1"
	envName           = "env1"
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
	ingressName1      = "ingress1"
	ingressHost1      = "ingress1.example.com"
	ingressName2      = "ingress2"
	ingressHost2      = "ingress2.example.com"
	ingressName3      = "ingress3"
	ingressHost3      = "ingress3.example.com"
	port8080          = int32(8080)
	port8090          = int32(8090)
	port9090          = int32(9090)
)

type scenario struct {
	name                     string
	existingEventProps       []eventProps
	expectedEvents           []eventModels.Event
	existingIngressRuleProps []ingressRuleProps
}

type eventProps struct {
	name       string
	namespace  string
	eventType  string
	objectName string
	objectUid  string
	objectKind string
}

type ingressRuleProps struct {
	name    string
	host    string
	service string
	port    int32
	appName string
	envName string
}

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

func Test_EventHandler_GetEnvironmentEvents(t *testing.T) {
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
		{
			name: "ReplicaSet events",
			existingEventProps: []eventProps{
				{name: ev1, namespace: envNamespace, eventType: k8sEventTypeNormal, objectName: replicaSetServer1, objectKind: k8sKindReplicaSet, objectUid: uid1},
				{name: ev2, namespace: envNamespace, eventType: k8sEventTypeNormal, objectName: replicaSetServer2, objectKind: k8sKindReplicaSet, objectUid: uid2},
				{name: ev3, namespace: "app2-env", eventType: k8sEventTypeNormal, objectName: replicaSetServer3, objectKind: k8sKindReplicaSet, objectUid: uid3},
			},
			expectedEvents: []eventModels.Event{
				{InvolvedObjectName: replicaSetServer1, InvolvedObjectKind: k8sKindReplicaSet, InvolvedObjectNamespace: envNamespace},
				{InvolvedObjectName: replicaSetServer2, InvolvedObjectKind: k8sKindReplicaSet, InvolvedObjectNamespace: envNamespace},
			},
		},
		{
			name: "Ingress events",
			existingEventProps: []eventProps{
				{name: ev1, namespace: envNamespace, eventType: k8sEventTypeNormal, objectName: ingressName1, objectKind: k8sKindIngress, objectUid: uid1},
				{name: ev2, namespace: envNamespace, eventType: k8sEventTypeNormal, objectName: ingressName2, objectKind: k8sKindIngress, objectUid: uid2},
				{name: ev3, namespace: "app2-env", eventType: k8sEventTypeNormal, objectName: ingressName3, objectKind: k8sKindIngress, objectUid: uid3},
			},
			expectedEvents: []eventModels.Event{
				{InvolvedObjectName: ingressName1, InvolvedObjectKind: k8sKindIngress, InvolvedObjectNamespace: envNamespace},
				{InvolvedObjectName: ingressName2, InvolvedObjectKind: k8sKindIngress, InvolvedObjectNamespace: envNamespace},
			},
		},
		{
			name: "Ingress events with rules",
			existingEventProps: []eventProps{
				{name: ev1, namespace: envNamespace, eventType: k8sEventTypeNormal, objectName: ingressName1, objectKind: k8sKindIngress, objectUid: uid1},
				{name: ev2, namespace: envNamespace, eventType: k8sEventTypeNormal, objectName: ingressName2, objectKind: k8sKindIngress, objectUid: uid2},
				{name: ev3, namespace: "app2-env", eventType: k8sEventTypeNormal, objectName: ingressName3, objectKind: k8sKindIngress, objectUid: uid3},
			},
			existingIngressRuleProps: []ingressRuleProps{
				{name: ingressName1, appName: appName, envName: envName, host: ingressHost1, service: deploy1, port: port8080},
				{name: ingressName2, appName: appName, envName: envName, host: ingressHost2, service: deploy2, port: port8090},
				{name: ingressName3, appName: "app2", envName: envName, host: ingressHost3, service: deploy3, port: port9090},
			},
			expectedEvents: []eventModels.Event{
				{InvolvedObjectName: ingressName1, InvolvedObjectKind: k8sKindIngress, InvolvedObjectNamespace: envNamespace},
				{InvolvedObjectName: ingressName2, InvolvedObjectKind: k8sKindIngress, InvolvedObjectNamespace: envNamespace},
			},
		},
	}
	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			eventHandler := setupTestEnvForHandler(t, appName, envName, ts)
			actualEvents, err := eventHandler.GetEnvironmentEvents(context.Background(), appName, envName)
			assert.Nil(t, err)
			assertEvents(t, ts.expectedEvents, actualEvents)
		})
	}
}

func assertEvents(t *testing.T, expectedEvents []eventModels.Event, actualEvents []*eventModels.Event) {
	if assert.Len(t, actualEvents, len(expectedEvents)) {
		for i := 0; i < len(expectedEvents); i++ {
			assert.Equal(t, expectedEvents[i].InvolvedObjectName, actualEvents[i].InvolvedObjectName)
			assert.Equal(t, expectedEvents[i].InvolvedObjectKind, actualEvents[i].InvolvedObjectKind)
			assert.Equal(t, expectedEvents[i].InvolvedObjectNamespace, actualEvents[i].InvolvedObjectNamespace)
		}
	}
}

func setupTestEnvForHandler(t *testing.T, appName string, envName string, ts scenario) EventHandler {
	kubeClient, radixClient := setupTest()
	createRadixAppWithEnvironment(t, radixClient, appName, envName)
	for _, evProps := range ts.existingEventProps {
		createKubernetesEvent(t, kubeClient, evProps.namespace, evProps.name, evProps.eventType, evProps.objectName, evProps.objectKind, evProps.objectUid)
	}
	for _, ingressRuleProp := range ts.existingIngressRuleProps {
		createIngressRule(t, kubeClient, ingressRuleProp)
	}
	eventHandler := Init(kubeClient, radixClient)
	return eventHandler
}

func createIngressRule(t *testing.T, kubeClient *kubefake.Clientset, props ingressRuleProps) {
	_, err := kubeClient.NetworkingV1().Ingresses(operatorutils.GetEnvironmentNamespace(props.appName, props.envName)).Create(context.Background(), &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: props.name,
			Labels: radixlabels.ForApplicationName(props.appName)},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{Host: props.host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: pointers.Ptr(networkingv1.PathTypeImplementationSpecific),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: props.service,
											Port: networkingv1.ServiceBackendPort{Number: props.port},
										},
									},
								},
							},
						},
					}},
			},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
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
