package api

import (
	"fmt"
	"net/http"

	log "github.com/Sirupsen/logrus"
	socketio "github.com/googollee/go-socket.io"
	"github.com/gorilla/mux"
	"github.com/rakyll/statik/fs"
	"github.com/rs/cors"
	"github.com/statoil/radix-api/api/deployment"
	"github.com/statoil/radix-api/api/job"
	"github.com/statoil/radix-api/api/platform"
	"github.com/statoil/radix-api/api/pod"
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
	Middleware *negroni.Negroni
}

// NewServer Constructor function
func NewServer() http.Handler {
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
	addHandlerRoutes(router, deployment.GetRoutes())

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

	return getCORSHandler(server)
}

func getCORSHandler(apiRouter *Server) http.Handler {
	clusterName := getClusterName()
	c := cors.New(cors.Options{
		AllowedOrigins: []string{
			"http://localhost:3000", // For socket.io testing
			"http://localhost:3001", // For socket.io testing
			"http://localhost:8086", // For swaggerui testing
			// TODO: We should consider:
			// 1. "https://*.radix.equinor.com"
			// 2. Keep cors rules in ingresses
			getHostName("web", "radix-web-console-dev", clusterName),
			getHostName("web", "radix-web-console-prod", clusterName),
		},
		AllowCredentials: true, // Needed for sockets
		MaxAge:           600,
		AllowedHeaders:   []string{"Accept", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization"},
		AllowedMethods:   []string{"GET", "PUT", "POST", "OPTIONS", "DELETE"},
	})
	return c.Handler(apiRouter.Middleware)
}

func getClusterName() string {
	client, _ := utils.GetInClusterKubernetesClient()
	radixconfigmap, err := client.CoreV1().ConfigMaps("default").Get("radix-config", metav1.GetOptions{})
	if err != nil {
		panic(err)
	}

	clustername := radixconfigmap.Data["clustername"]
	return clustername
}

func getHostName(componentName, namespace, clustername string) string {
	return fmt.Sprintf("https://%s-%s.%s.dev.radix.equinor.com", componentName, namespace, clustername)
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

		addSubscriptions(so, disconnect, allSubscriptions, client, radixclient, deployment.GetSubscriptions())
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
