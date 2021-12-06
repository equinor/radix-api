package models

import (
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	secretProviderClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
)

// Account Holds kubernetes account sessions
type Account struct {
	Client               kubernetes.Interface
	RadixClient          radixclient.Interface
	SecretProviderClient secretProviderClient.Interface
}
