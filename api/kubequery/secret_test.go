package kubequery

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func Test_GetSecretsForEnvironment(t *testing.T) {
	matched1 := corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "matched1", Namespace: "app1-env1"}}
	matched2 := corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "matched2", Namespace: "app1-env1"}}
	unmatched := corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "unmatched", Namespace: "app2-env1"}}
	client := kubefake.NewSimpleClientset(&matched1, &matched2, &unmatched)
	expected := []corev1.Secret{matched1, matched2}
	actual, err := GetSecretsForEnvironment(context.Background(), client, "app1", "env1")
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, actual)
}
