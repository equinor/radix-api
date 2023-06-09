package kubequery

import (
	"context"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetRadixApplication(ctx context.Context, client radixclient.Interface, appName string) (*radixv1.RadixApplication, error) {
	ns := operatorUtils.GetAppNamespace(appName)
	return client.RadixV1().RadixApplications(ns).Get(ctx, appName, metav1.GetOptions{})
}
