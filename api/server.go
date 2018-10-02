package api

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	socketio "github.com/googollee/go-socket.io"
	"github.com/gorilla/mux"
	"github.com/rakyll/statik/fs"
	"github.com/statoil/radix-api-go/api/job"
	"github.com/statoil/radix-api-go/api/platform"
	"github.com/statoil/radix-api-go/api/pod"
	"github.com/statoil/radix-api-go/api/utils"
	"github.com/statoil/radix-api-go/models"
	_ "github.com/statoil/radix-api-go/swaggerui" // statik files
	"github.com/urfave/negroni"

	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const apiVersionRoute = "/api/v1"

// Server Holds instance variables
type Server struct {
	Middleware *negroni.Negroni
}

// NewServer Constructor function
func NewServer() *Server {
	router := mux.NewRouter().StrictSlash(true)

	statikFS, err := fs.New()
	if err != nil {
		panic(err)
	}

	staticServer := http.FileServer(statikFS)
	sh := http.StripPrefix("/swaggerui/", staticServer)
	router.PathPrefix("/swaggerui/").Handler(sh)

	initializeSocketServer(router)
	addHandlerRoutes(router, platform.GetRoutes())
	addHandlerRoutes(router, job.GetRoutes())
	addHandlerRoutes(router, pod.GetRoutes())

	serveMux := http.NewServeMux()
	serveMux.Handle("/api/", negroni.New(
		negroni.HandlerFunc(utils.BearerTokenHeaderVerifyerMiddleware),
		negroni.Wrap(router),
	))

	serveMux.Handle("/socket.io/", negroni.New(
		negroni.HandlerFunc(utils.BearerTokenQueryVerifyerMiddleware),
		negroni.Wrap(router),
	))

	// TODO: We should maybe have oauth to stop any non-radix user from beeing
	// able to see the API
	serveMux.Handle("/swaggerui/", negroni.New(
		negroni.Wrap(router),
	))

	n := negroni.Classic()
	n.UseHandler(serveMux)

	server := &Server{
		n,
	}

	return server
}

func addHandlerRoutes(router *mux.Router, routes models.Routes) {
	for _, route := range routes {
		router.HandleFunc(apiVersionRoute+route.Path, utils.NewRadixMiddleware(route.HandlerFunc, route.WatcherFunc).Handle).Methods(route.Method)
	}
}

func initializeSocketServer(router *mux.Router) {
	socketServer, _ := socketio.NewServer(nil)

	socketServer.On("connection", func(so socketio.Socket) {
		token := utils.GetTokenFromQuery(so.Request())
		client, radixclient := utils.GetKubernetesClient(token)

		// Make an extra check that the user has the correct access
		_, err := client.CoreV1().Namespaces().List(metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Socket connection refused, due to %v", err)

			// Refuse connection
			so.Disconnect()
		}

		disconnect := make(chan struct{})
		allSubscriptions := make(map[string]chan struct{})

		addSubscriptions(so, disconnect, allSubscriptions, client, radixclient, platform.GetSubscriptions())
		addSubscriptions(so, disconnect, allSubscriptions, client, radixclient, pod.GetSubscriptions())
		addSubscriptions(so, disconnect, allSubscriptions, client, radixclient, job.GetSubscriptions())

		so.On("disconnection", func() {
			if disconnect != nil {
				close(disconnect)
				disconnect = nil

				// close all open subscriptions
				for datatype, subscription := range allSubscriptions {
					close(subscription)
					subscription = nil
					delete(allSubscriptions, datatype)

					logrus.Infof("Unsubscribed from %s", datatype)
				}
			}
		})
	})

	router.Handle("/socket.io/", socketServer)
}

func addSubscriptions(so socketio.Socket, disconnect chan struct{}, allSubscriptions map[string]chan struct{}, client kubernetes.Interface, radixclient radixclient.Interface, subscriptions models.Subscriptions) {
	for _, sub := range subscriptions {
		var subscription chan struct{}

		so.On(sub.SubcribeCommand, func(so socketio.Socket, arg string) {
			// Allow only one subscription for now,
			// unsubscribe and resubscribe with new arguments
			if subscription != nil {
				close(subscription)
				subscription = nil
			}

			subscription = make(chan struct{})
			allSubscriptions[sub.DataType] = subscription

			data := make(chan []byte)
			go sub.HandlerFunc(client, radixclient, arg, data, subscription)
			go writeEventToSocket(so, sub.DataType, disconnect, data)

			logrus.Infof("Subscribing to %s", sub.DataType)
		})

		so.On(sub.UnsubscribeCommand, func() {
			// In case we call unsubscribe when we are not subscribed
			if subscription != nil {
				close(subscription)
				subscription = nil
				delete(allSubscriptions, sub.DataType)

				logrus.Infof("Unsubscribed from %s", sub.DataType)
			}
		})
	}
}

func writeEventToSocket(so socketio.Socket, event string, disconnect chan struct{}, data chan []byte) {
	for {
		select {
		case <-disconnect:
			return
		case <-data:
			for dataElement := range data {
				so.Emit(event, string(dataElement))
				logrus.Infof("Emitted data for %s", event)
			}
		}
	}
}
