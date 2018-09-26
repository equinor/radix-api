package utils

import (
	"errors"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/statoil/radix-api-go/models"
)

// RadixMiddleware The middleware beween router and radix handler functions
type RadixMiddleware struct {
	next  models.RadixHandlerFunc
	watch models.RadixWatcherFunc
}

// NewRadixMiddleware Constructor for radix middleware
func NewRadixMiddleware(next models.RadixHandlerFunc, watch models.RadixWatcherFunc) *RadixMiddleware {
	handler := &RadixMiddleware{
		next,
		watch,
	}

	return handler
}

// Handle Wraps radix handler methods
func (handler *RadixMiddleware) Handle(w http.ResponseWriter, r *http.Request) {
	logrus.Info("Handle request")
	token, err := getBearerTokenFromHeader(r)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	watch, _ := isWatch(r)
	if watch && handler.watch == nil {
		WriteError(w, r, http.StatusBadRequest, errors.New("Watch is not supported for this type"))
		return
	}

	client, radixclient := GetKubernetesClient(token)

	if watch {
		// Sending data as server side events
		data := make(chan []byte)
		subscription := make(chan struct{})

		broker := &Broker{
			Notifier:     data,
			Subscription: subscription,
		}

		// Set it running - listening and broadcasting events
		go handler.watch(client, radixclient, "", data, subscription)
		broker.ServeSSE(w, r)

	} else {
		handler.next(client, radixclient, w, r)
	}
}

// BearerTokenVerifyerMiddleware Will verify that the request has a bearer token
func BearerTokenVerifyerMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	_, err := getBearerTokenFromHeader(r)

	if err != nil {
		WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	next(w, r)
}
