package models

import (
	"net/http"

	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

// Clients Holds both in cluster clients and out of cluster clients
type Clients struct {
	// Clients running with authorization of the API server
	InClusterClient      kubernetes.Interface
	InClusterRadixClient radixclient.Interface

	// Clients using the authorization of the user
	OutClusterClient      kubernetes.Interface
	OutClusterRadixClient radixclient.Interface
}

// RadixHandlerFunc Pattern for handler functions
type RadixHandlerFunc func(Clients, http.ResponseWriter, *http.Request)

// RadixStreamerFunc Pattern for watcher functions
type RadixStreamerFunc func(Clients, string, []string, chan []byte, chan struct{})

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

// Subscriptions Holder of all subscriptions
type Subscriptions []Subscription

// Subscription Holds information on stream handler function
type Subscription struct {
	Resource    string
	DataType    string
	HandlerFunc RadixStreamerFunc
}
