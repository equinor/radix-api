package kubequery

import (
	"context"

	"github.com/equinor/radix-api/api/utils/labelselector"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetRadixEnvironments(ctx context.Context, client radixclient.Interface, appName string) ([]radixv1.RadixEnvironment, error) {
	res, err := client.RadixV1().RadixEnvironments().List(ctx, v1.ListOptions{LabelSelector: labelselector.ForApplication(appName).String()})
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}
