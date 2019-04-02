package models

import (
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

type Account struct {
	Client      kubernetes.Interface
	RadixClient radixclient.Interface
}
