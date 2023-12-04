package kubequery

import (
	"context"
	"testing"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_GetRadixDNSAliases(t *testing.T) {
	matched1 := radixv1.RadixDNSAlias{ObjectMeta: metav1.ObjectMeta{Name: "matched1", Labels: labels.ForApplicationName("app1")}}
	matched2 := radixv1.RadixDNSAlias{ObjectMeta: metav1.ObjectMeta{Name: "matched2", Labels: labels.ForApplicationName("app1")}}
	unmatched1 := radixv1.RadixDNSAlias{ObjectMeta: metav1.ObjectMeta{Name: "unmatched1", Labels: labels.ForApplicationName("app2")}}
	client := radixfake.NewSimpleClientset(&matched1, &matched2, &unmatched1)
	expected := []radixv1.RadixDNSAlias{matched1, matched2}
	actual, err := GetRadixDNSAliases(context.Background(), client, "app1")
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, actual)
}
