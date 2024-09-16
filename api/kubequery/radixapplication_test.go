package kubequery

import (
	"context"
	"testing"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_GetRadixApplication(t *testing.T) {
	matched := radixv1.RadixApplication{ObjectMeta: metav1.ObjectMeta{Name: "app1", Namespace: "app1-app"}}
	unmatched := radixv1.RadixApplication{ObjectMeta: metav1.ObjectMeta{Name: "app2", Namespace: "app2-any"}}
	client := radixfake.NewSimpleClientset(&matched, &unmatched)

	// Get existing RA
	actual, err := GetRadixApplication(context.Background(), client, "app1")
	require.NoError(t, err)
	assert.Equal(t, &matched, actual)

	// Get non-existing RA (wrong namespace)
	_, err = GetRadixApplication(context.Background(), client, "app2")
	assert.True(t, errors.IsNotFound(err))
}
