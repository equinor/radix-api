package utils

import (
	"log"
	"net/http"

	"github.com/pkg/errors"
)

// Broker holds open client connections for server side events,
// listens for incoming events on its Notifier channel
// and broadcast event data to all registered connections
type Broker struct {

	// Events are pushed to this channel by the main events-gathering routine
	Notifier chan []byte

	// Subscription
	Subscription chan struct{}
}

// ServeSSE Serves server side events
func (broker *Broker) ServeSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)

	if !ok {
		WriteError(w, r, http.StatusBadRequest, errors.New("Streaming unsupported"))
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
		close(broker.Subscription)
	}()

	for {
		select {
		case <-broker.Subscription:
			log.Print("Removed client")
			return
		default:
			// Write to the ResponseWriter
			// Server Sent Events compatible
			w.Write(<-broker.Notifier)

			// Flush the data immediatly instead of buffering it for later.
			flusher.Flush()
		}
	}

}
