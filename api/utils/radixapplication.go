package utils

import (
	"context"
	secretsstorevclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
)

// CreateApplicationConfig creates an application config based on input
func CreateApplicationConfig(client kubernetes.Interface, radixClient radixclient.Interface, secretProviderClient secretsstorevclient.Interface, appName string) (*applicationconfig.ApplicationConfig, error) {
	radixApp, err := radixClient.RadixV1().RadixApplications(crdUtils.GetAppNamespace(appName)).Get(context.TODO(), appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	registration, err := radixClient.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	kubeUtils, _ := kube.New(client, radixClient)
	kubeUtils.WithSecretsProvider(secretProviderClient)
	return applicationconfig.NewApplicationConfig(client, kubeUtils, radixClient, registration, radixApp)
}
