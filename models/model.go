package models

import (
	"net/http"

	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

// RadixHandlerFunc Pattern for handler functions
type RadixHandlerFunc func(kubernetes.Interface, radixclient.Interface, http.ResponseWriter, *http.Request)

// Handler Pattern of an rest/stream handler
type Handler interface {
	GetRoutes() Routes
	GetSubscriptions() Subscriptions
}

// Routes Holder of all routes
type Routes []Route

// Route Describe route
type Route struct {
	Path        string
	Method      string
	HandlerFunc RadixHandlerFunc
}

// StreamHandlerFunc Is an adapter to allow the use of
// ordinary stream functions as handlers
type StreamHandlerFunc func(kubernetes.Interface, radixclient.Interface, string, chan []byte, chan struct{})

// Subscriptions Holder of all subscriptions
type Subscriptions []Subscription

// Subscription Holds information on stream handler function
type Subscription struct {
	SubcribeCommand    string
	UnsubscribeCommand string
	DataType           string
	HandlerFunc        StreamHandlerFunc
}
