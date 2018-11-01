package models

import (
	"net/http"

	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

// RadixHandlerFunc Pattern for handler functions
type RadixHandlerFunc func(kubernetes.Interface, radixclient.Interface, http.ResponseWriter, *http.Request)

// RadixWatcherFunc Pattern for watcher functions
type RadixWatcherFunc func(kubernetes.Interface, radixclient.Interface, string, chan []byte, chan struct{})

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
	Path        string
	Method      string
	HandlerFunc RadixHandlerFunc
	WatcherFunc RadixWatcherFunc
}

// Subscriptions Holder of all subscriptions
type Subscriptions []Subscription

// Subscription Holds information on stream handler function
type Subscription struct {
	SubcribeCommand    string
	UnsubscribeCommand string
	DataType           string
	HandlerFunc        RadixWatcherFunc
}
