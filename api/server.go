package api

import (
	"fmt"
	"net/http"

	log "github.com/Sirupsen/logrus"
	socketio "github.com/googollee/go-socket.io"
	"github.com/gorilla/mux"
	"github.com/rakyll/statik/fs"
	"github.com/rs/cors"
	"github.com/statoil/radix-api/api/admissioncontrollers"
	"github.com/statoil/radix-api/api/applications"
	"github.com/statoil/radix-api/api/deployments"
	"github.com/statoil/radix-api/api/jobs"
	"github.com/statoil/radix-api/api/pods"
	"github.com/statoil/radix-api/api/utils"
	"github.com/statoil/radix-api/models"
	_ "github.com/statoil/radix-api/swaggerui" // statik files
	"github.com/urfave/negroni"

	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const apiVersionRoute = "/api/v1"

// Server Holds instance variables
type Server struct {
	Middleware  *negroni.Negroni
	clusterName string
}

// NewServer Constructor function
func NewServer(clusterName string) http.Handler {
	router := mux.NewRouter().StrictSlash(true)

	statikFS, err := fs.New()
	if err != nil {
		panic(err)
	}

	staticServer := http.FileServer(statikFS)
	sh := http.StripPrefix("/swaggerui/", staticServer)
	router.PathPrefix("/swaggerui/").Handler(sh)

	initializeSocketServer(router)
	addHandlerRoutes(router, applications.GetRoutes())
	addHandlerRoutes(router, jobs.GetRoutes())
	addHandlerRoutes(router, pods.GetRoutes())
	addHandlerRoutes(router, deployments.GetRoutes())
	addHandlerRoutesInClusterKubeClient(router, admissioncontrollers.GetRoutes(), "")

	serveMux := http.NewServeMux()
	serveMux.Handle(fmt.Sprintf("%s/", admissioncontrollers.RootPath), negroni.New(
		negroni.Wrap(router),
	))

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
		clusterName,
	}

	return getCORSHandler(server)
}

func getCORSHandler(apiRouter *Server) http.Handler {
	c := cors.New(cors.Options{
		AllowedOrigins: []string{
			"http://localhost:3000", // For socket.io testing
			"http://localhost:3001", // For socket.io testing
			"http://localhost:8086", // For swaggerui testing
			// TODO: We should consider:
			// 1. "https://*.radix.equinor.com"
			// 2. Keep cors rules in ingresses
			getHostName("web", "radix-web-console-dev", apiRouter.clusterName),
			getHostName("web", "radix-web-console-prod", apiRouter.clusterName),
		},
		AllowCredentials: true, // Needed for sockets
		MaxAge:           600,
		AllowedHeaders:   []string{"Accept", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization"},
		AllowedMethods:   []string{"GET", "PUT", "POST", "OPTIONS", "DELETE"},
	})
	return c.Handler(apiRouter.Middleware)
}

func getHostName(componentName, namespace, clustername string) string {
	return fmt.Sprintf("https://%s-%s.%s.dev.radix.equinor.com", componentName, namespace, clustername)
}

func addHandlerRoutes(router *mux.Router, routes models.Routes) {
	for _, route := range routes {
		router.HandleFunc(apiVersionRoute+route.Path, utils.NewRadixMiddleware(route.HandlerFunc, route.WatcherFunc).Handle).Methods(route.Method)
	}
}

// routes which should be run under radix-api service account, instead of using incomming access token
func addHandlerRoutesInClusterKubeClient(router *mux.Router, routes models.Routes, rootURL string) {
	for _, route := range routes {
		router.HandleFunc(rootURL+route.Path,
			func(w http.ResponseWriter, r *http.Request) {
				client, radixclient := utils.GetInClusterKubernetesClient()
				route.HandlerFunc(client, radixclient, w, r)
			}).Methods(route.Method)
	}
}

func initializeSocketServer(router *mux.Router) {
	socketServer, _ := socketio.NewServer(nil)

	socketServer.On("connection", func(so socketio.Socket) {
		token := utils.GetTokenFromQuery(so.Request())
		client, radixclient := utils.GetOutClusterKubernetesClient(token)

		// Make an extra check that the user has the correct access
		_, err := client.CoreV1().Namespaces().List(metav1.ListOptions{})
		if err != nil {
			log.Errorf("Socket connection refused, due to %v", err)

			// Refuse connection
			so.Disconnect()
		}

		disconnect := make(chan struct{})
		allSubscriptions := make(map[string]chan struct{})

		addSubscriptions(so, disconnect, allSubscriptions, client, radixclient, deployments.GetSubscriptions())
		addSubscriptions(so, disconnect, allSubscriptions, client, radixclient, applications.GetSubscriptions())
		addSubscriptions(so, disconnect, allSubscriptions, client, radixclient, pods.GetSubscriptions())
		addSubscriptions(so, disconnect, allSubscriptions, client, radixclient, jobs.GetSubscriptions())

		so.On("disconnection", func() {
			if disconnect != nil {
				close(disconnect)
				disconnect = nil

				// close all open subscriptions
				for datatype, subscription := range allSubscriptions {
					close(subscription)
					subscription = nil
					delete(allSubscriptions, datatype)

					log.Infof("Unsubscribed from %s", datatype)
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

			log.Infof("Subscribing to %s", sub.DataType)
		})

		so.On(sub.UnsubscribeCommand, func() {
			// In case we call unsubscribe when we are not subscribed
			if subscription != nil {
				close(subscription)
				subscription = nil
				delete(allSubscriptions, sub.DataType)

				log.Infof("Unsubscribed from %s", sub.DataType)
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
				log.Infof("Emitted data for %s", event)
			}
		}
	}
}
