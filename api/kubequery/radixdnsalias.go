package kubequery

import (
	"context"

	"github.com/equinor/radix-api/api/utils/labelselector"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetRadixDNSAliases returns all RadixDNSAliases for the specified application.
func GetRadixDNSAliases(ctx context.Context, client radixclient.Interface, appName string) ([]radixv1.RadixDNSAlias, error) {
	res, err := client.RadixV1().RadixDNSAliases().List(ctx, metav1.ListOptions{LabelSelector: labelselector.ForApplication(appName).String()})
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}
