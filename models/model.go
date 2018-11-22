package models

import (
	"net/http"

	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

// RadixHandlerFunc Pattern for handler functions
type RadixHandlerFunc func(kubernetes.Interface, radixclient.Interface, http.ResponseWriter, *http.Request)

// RadixStreamerFunc Pattern for watcher functions
type RadixStreamerFunc func(kubernetes.Interface, radixclient.Interface, string, []string, chan []byte, chan struct{})

// Controller Pattern of an rest/stream controller
type Controller interface {
	GetRoutes() Routes
	GetSubscriptions() Subscriptions
	UseInClusterConfig() bool
}

// DefaultController Default implementation
type DefaultController struct {
}

// UseInClusterConfig Default implementation
func (d *DefaultController) UseInClusterConfig() bool {
	return false
}

// Routes Holder of all routes
type Routes []Route

// Route Describe route
type Route struct {
	Path                   string
	Method                 string
	RunInClusterKubeClient bool // kube client will be run under radix-api service account, instead of using users access token
	HandlerFunc            RadixHandlerFunc
}

// Subscriptions Holder of all subscriptions
type Subscriptions []Subscription

// Subscription Holds information on stream handler function
type Subscription struct {
	Resource    string
	DataType    string
	HandlerFunc RadixStreamerFunc
}
