package kubequery

import (
	"context"
	"testing"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_GetRadixDNSAliases(t *testing.T) {
	matched1 := radixv1.RadixDNSAlias{
		ObjectMeta: metav1.ObjectMeta{Name: "matched1"},
		Spec: radixv1.RadixDNSAliasSpec{
			AppName: "app1", Environment: "env1", Component: "comp1",
		},
		Status: radixv1.RadixDNSAliasStatus{
			Condition: "Success",
			Message:   "",
		},
	}
	matched2 := radixv1.RadixDNSAlias{ObjectMeta: metav1.ObjectMeta{Name: "matched2"}, Spec: radixv1.RadixDNSAliasSpec{
		AppName: "app1", Environment: "env1", Component: "comp1",
	},
		Status: radixv1.RadixDNSAliasStatus{
			Condition: "Failed",
			Message:   "Some error",
		},
	}
	unmatched := radixv1.RadixDNSAlias{ObjectMeta: metav1.ObjectMeta{Name: "unmatched"}, Spec: radixv1.RadixDNSAliasSpec{
		AppName: "app2", Environment: "env1", Component: "comp1",
	}}
	client := radixfake.NewSimpleClientset(&matched1, &matched2, &unmatched)
	expected := []applicationModels.DNSAlias{
		{
			URL:             "matched1.test.radix.equinor.com",
			EnvironmentName: "env1",
			ComponentName:   "comp1",
			Status: applicationModels.DNSAliasStatus{
				Condition: "Success",
				Message:   "",
			},
		}, {
			URL:             "matched2.test.radix.equinor.com",
			EnvironmentName: "env1",
			ComponentName:   "comp2",
			Status: applicationModels.DNSAliasStatus{
				Condition: "Failed",
				Message:   "Some error",
			},
		}}
	ra := &radixv1.RadixApplication{ObjectMeta: metav1.ObjectMeta{Name: "app1"},
		Spec: radixv1.RadixApplicationSpec{
			DNSAlias: []radixv1.DNSAlias{
				{Alias: "matched1", Environment: "env1", Component: "comp1"},
				{Alias: "matched2", Environment: "env1", Component: "comp2"},
			}},
	}
	actual := GetDNSAliases(context.Background(), client, ra, "test.radix.equinor.com")
	require.Len(t, actual, 2, "unexpected amount of actual DNS aliases")
	assert.ElementsMatch(t, expected, actual)
}
