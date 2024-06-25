package kubequery

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetRadixBatchesForJobComponent(ctx context.Context, client radixclient.Interface, appName, envName, jobComponentName string, batchType kube.RadixBatchType) ([]radixv1.RadixBatch, error) {
	namespace := operatorutils.GetEnvironmentNamespace(appName, envName)
	selector := radixlabels.Merge(
		radixlabels.ForApplicationName(appName),
		radixlabels.ForComponentName(jobComponentName),
		radixlabels.ForBatchType(batchType),
	)

	batches, err := client.RadixV1().RadixBatches(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}

	return batches.Items, nil
}
