package models

import (
	"net/http"

	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

// NewAccounts creates a new Accounts struct
func NewAccounts(
	inClusterClient kubernetes.Interface,
	inClusterRadixClient radixclient.Interface,
	outClusterClient kubernetes.Interface,
	outClusterRadixClient radixclient.Interface) Accounts {

	return Accounts{
		UserAccount: Account{
			Client:      outClusterClient,
			RadixClient: outClusterRadixClient,
		},
		ServiceAccount: Account{
			Client:      inClusterClient,
			RadixClient: inClusterRadixClient,
		},
	}
}

// Accounts contains accounts for accessing k8s API.
type Accounts struct {
	UserAccount    Account
	ServiceAccount Account
}

// RadixHandlerFunc Pattern for handler functions
type RadixHandlerFunc func(Accounts, http.ResponseWriter, *http.Request)

// Controller Pattern of an rest/stream controller
type Controller interface {
	GetRoutes() Routes
}

// DefaultController Default implementation
type DefaultController struct {
}

// Routes Holder of all routes
type Routes []Route

// Route Describe route
type Route struct {
	Path        string
	Method      string
	HandlerFunc RadixHandlerFunc
}
