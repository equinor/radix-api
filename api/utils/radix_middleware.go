package utils

import (
	"errors"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-api/models"
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
	log.Info("Handle request")
	token, err := getBearerTokenFromHeader(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	watch, _ := isWatch(r)
	if watch && handler.watch == nil {
		WriteError(w, r, errors.New("Watch is not supported for this type"))
		return
	}

	client, radixclient := GetKubernetesClient(token)

	if watch {
		// Sending data as server side events
		data := make(chan []byte)
		subscription := make(chan struct{})

		// Set it running - listening and broadcasting events
		go handler.watch(client, radixclient, "", data, subscription)
		serveSSE(w, r, data, subscription)

	} else {
		handler.next(client, radixclient, w, r)
	}
}

// BearerTokenHeaderVerifyerMiddleware Will verify that the request has a bearer token in header
func BearerTokenHeaderVerifyerMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	_, err := getBearerTokenFromHeader(r)

	if err != nil {
		WriteError(w, r, err)
		return
	}

	next(w, r)
}

// BearerTokenQueryVerifyerMiddleware Will verify that the request has a bearer token as query variable
func BearerTokenQueryVerifyerMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	// For socket connections it should be in the query
	jwtToken := GetTokenFromQuery(r)
	if jwtToken == "" {
		WriteError(w, r, errors.New("Authentication token is required"))
		return
	}

	next(w, r)
}

// ServeSSE Serves server side events
func serveSSE(w http.ResponseWriter, r *http.Request, data chan []byte, subscription chan struct{}) {
	flusher, ok := w.(http.Flusher)

	if !ok {
		WriteError(w, r, errors.New("Streaming unsupported"))
		return
	}

	// Set the headers related to event streaming.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Listen to connection close and un-register messageChan
	notify := w.(http.CloseNotifier).CloseNotify()

	go func() {
		<-notify
		close(subscription)
	}()

	for {
		select {
		case <-subscription:
			return
		default:
			// Write to the ResponseWriter
			// Server Sent Events compatible
			w.Write(<-data)

			// Flush the data immediatly instead of buffering it for later.
			flusher.Flush()
		}
	}

}
