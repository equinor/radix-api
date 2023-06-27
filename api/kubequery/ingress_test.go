package kubequery

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func Test_GetIngressesForEnvironments(t *testing.T) {
	matched1 := networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "matched1", Namespace: "app1-env1", Labels: labels.ForApplicationName("app1")}}
	matched2 := networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "matched2", Namespace: "app1-env1", Labels: labels.ForApplicationName("app1")}}
	matched3 := networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "matched3", Namespace: "app1-env2", Labels: labels.ForApplicationName("app1")}}
	unmatched1 := networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "unmatched1", Namespace: "app1-env1", Labels: labels.ForApplicationName("app2")}}
	unmatched2 := networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "unmatched2", Namespace: "app1-env3", Labels: labels.ForApplicationName("app1")}}
	unmatched3 := networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "unmatched3", Namespace: "app1-env1"}}
	client := kubefake.NewSimpleClientset(&matched1, &matched2, &matched3, &unmatched1, &unmatched2, &unmatched3)
	expected := []networkingv1.Ingress{matched1, matched2, matched3}
	actual, err := GetIngressesForEnvironments(context.Background(), client, "app1", []string{"env1", "env2"}, 10)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, actual)
}
