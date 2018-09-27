package api

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	socketio "github.com/googollee/go-socket.io"
	"github.com/gorilla/mux"
	"github.com/statoil/radix-api-go/api/job"
	"github.com/statoil/radix-api-go/api/platform"
	"github.com/statoil/radix-api-go/api/pod"
	"github.com/statoil/radix-api-go/api/utils"
	"github.com/statoil/radix-api-go/models"
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

	initializeSocketServer(router)
	addHandlerRoutes(router, platform.GetRoutes())
	addHandlerRoutes(router, job.GetRoutes())
	addHandlerRoutes(router, pod.GetRoutes())

	withBearerTokenVerifyer := http.NewServeMux()
	withBearerTokenVerifyer.Handle("/", negroni.New(
		negroni.HandlerFunc(utils.BearerTokenVerifyerMiddleware),
		negroni.Wrap(router),
	))

	n := negroni.Classic()
	n.UseHandler(withBearerTokenVerifyer)

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
			// Refuse connection
			so.Disconnect()
		}

		disconnect := make(chan struct{})
		addSubscriptions(so, disconnect, client, radixclient, platform.GetSubscriptions())
		addSubscriptions(so, disconnect, client, radixclient, pod.GetSubscriptions())
		addSubscriptions(so, disconnect, client, radixclient, job.GetSubscriptions())

		so.On("disconnection", func() {
			close(disconnect)
		})
	})

	router.Handle("/socket.io/", socketServer)
}

func addSubscriptions(so socketio.Socket, disconnect chan struct{}, client kubernetes.Interface, radixclient radixclient.Interface, subscriptions models.Subscriptions) {
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
