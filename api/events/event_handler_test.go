package events

import (
	"fmt"
	"testing"

	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	k8sObjectUtils "github.com/equinor/radix-operator/pkg/apis/utils"
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
	expected := k8sObjectUtils.GetEnvironmentNamespace(appName, envName)
	ra := builders.NewRadixApplicationBuilder().WithAppName(appName).BuildRA()
	actual := RadixEnvironmentNamespace(ra, envName)()
	assert.Equal(t, expected, actual)
}

func Test_EventHandler_GetEventsForRadixApplication(t *testing.T) {
	appName, envName := "app", "env"
	appNamespace := k8sObjectUtils.GetEnvironmentNamespace(appName, envName)
	kubeClient := kubefake.NewSimpleClientset()
	createKubernetesEvent(kubeClient, appNamespace, "ev1", "pod1")
	createKubernetesEvent(kubeClient, appNamespace, "ev2", "pod2")
	createKubernetesEvent(kubeClient, "app2-env", "ev3", "pod3")
	e, err := kubeClient.CoreV1().Events("").List(metav1.ListOptions{})
	fmt.Println(e)

	ra := builders.NewRadixApplicationBuilder().WithAppName(appName).BuildRA()
	eventHandler := Init(kubeClient)
	events, err := eventHandler.GetEvents(RadixEnvironmentNamespace(ra, envName))
	assert.Nil(t, err)
	assert.Len(t, events, 2)
	assert.ElementsMatch(
		t,
		[]string{"pod1", "pod2"},
		[]string{events[0].InvolvedObjectName, events[1].InvolvedObjectName},
	)
}

func createKubernetesEvent(client *kubefake.Clientset, ns, name, involvedObjectName string) {
	client.CoreV1().Events(ns).CreateWithEventNamespace(&v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		InvolvedObject: v1.ObjectReference{
			Name: involvedObjectName,
		},
	})
}
