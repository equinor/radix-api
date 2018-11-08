package router

import (
	"fmt"
	"net/http"

	log "github.com/Sirupsen/logrus"
	socketio "github.com/googollee/go-socket.io"
	"github.com/gorilla/mux"
	"github.com/rakyll/statik/fs"
	"github.com/rs/cors"
	"github.com/statoil/radix-api/api/utils"
	"github.com/statoil/radix-api/models"
	_ "github.com/statoil/radix-api/swaggerui" // statik files
	"github.com/urfave/negroni"

	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const apiVersionRoute = "/api/v1"
const admissionControllerRootPath = "/admissioncontrollers"

// Server Holds instance variables
type Server struct {
	Middleware  *negroni.Negroni
	clusterName string
	controllers []models.Controller
}

// NewServer Constructor function
func NewServer(clusterName string, kubeUtil utils.KubeUtil, controllers ...models.Controller) http.Handler {
	router := mux.NewRouter().StrictSlash(true)

	statikFS, err := fs.New()
	if err != nil {
		panic(err)
	}

	staticServer := http.FileServer(statikFS)
	sh := http.StripPrefix("/swaggerui/", staticServer)
	router.PathPrefix("/swaggerui/").Handler(sh)

	initializeSocketServer(kubeUtil, router)

	for _, controller := range controllers {
		if controller.UseInClusterConfig() {
			addHandlerRoutesInClusterKubeClient(kubeUtil, router, controller.GetRoutes(), "")
		} else {
			addHandlerRoutes(kubeUtil, router, controller.GetRoutes())
		}
	}

	serveMux := http.NewServeMux()
	serveMux.Handle(fmt.Sprintf("%s/", admissionControllerRootPath), negroni.New(
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
		controllers,
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
			"https://console.dev.radix.equinor.com",
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

func addHandlerRoutes(kubeUtil utils.KubeUtil, router *mux.Router, routes models.Routes) {
	for _, route := range routes {
		router.HandleFunc(apiVersionRoute+route.Path, utils.NewRadixMiddleware(kubeUtil, route.HandlerFunc, route.WatcherFunc).Handle).Methods(route.Method)
	}
}

// routes which should be run under radix-api service account, instead of using incomming access token
func addHandlerRoutesInClusterKubeClient(kubeUtil utils.KubeUtil, router *mux.Router, routes models.Routes, rootURL string) {
	for _, route := range routes {
		router.HandleFunc(rootURL+route.Path,
			func(w http.ResponseWriter, r *http.Request) {
				client, radixclient := kubeUtil.GetInClusterKubernetesClient()
				route.HandlerFunc(client, radixclient, w, r)
			}).Methods(route.Method)
	}
}

func initializeSocketServer(kubeUtil utils.KubeUtil, router *mux.Router, controllers ...models.Controller) {
	socketServer, _ := socketio.NewServer(nil)

	socketServer.On("connection", func(so socketio.Socket) {
		token := utils.GetTokenFromQuery(so.Request())
		client, radixclient := kubeUtil.GetOutClusterKubernetesClient(token)

		// Make an extra check that the user has the correct access
		_, err := client.CoreV1().Namespaces().List(metav1.ListOptions{})
		if err != nil {
			log.Errorf("Socket connection refused, due to %v", err)

			// Refuse connection
			so.Disconnect()
		}

		disconnect := make(chan struct{})
		allResourceSubscriptions := make(map[models.Subscription]chan struct{})

		for _, controller := range controllers {
			for _, sub := range controller.GetSubscriptions() {
				resourceSubscriptionChannel := make(chan struct{})
				allResourceSubscriptions[sub] = resourceSubscriptionChannel
			}
		}

		addSubscriptions(so, disconnect, allResourceSubscriptions, client, radixclient)

		so.On("disconnection", func() {
			if disconnect != nil {
				close(disconnect)
				disconnect = nil

				// close all open subscriptions
				for sub, subscriptionChannel := range allResourceSubscriptions {
					close(subscriptionChannel)
					subscriptionChannel = nil
					delete(allResourceSubscriptions, sub)

					log.Infof("Unsubscribed from %s", sub)
				}
			}
		})
	})

	router.Handle("/socket.io/", socketServer)
}

func addSubscriptions(so socketio.Socket, disconnect chan struct{}, allResourceSubscriptions map[models.Subscription]chan struct{}, client kubernetes.Interface, radixclient radixclient.Interface) {
	const subscribeCommand = "watch"
	const unSubscribeCommand = "unwatch"

	so.On(subscribeCommand, func(so socketio.Socket, resource string) {
		log.Infof("%s", resource)

		/*	for sub, channel := range allResourceSubscriptions {
			data := make(chan []byte)
			go sub.HandlerFunc(client, radixclient, arg, data, subscription)
			go writeEventToSocket(so, sub.DataType, disconnect, data)

			log.Infof("Subscribing to %s", sub.DataType)

		}*/
	})

	so.On(unSubscribeCommand, func(so socketio.Socket, resource string) {
		log.Infof("%s", resource)

		/*
			// In case we call unsubscribe when we are not subscribed
			if subscription != nil {
				close(subscription)
				subscription = nil
				delete(allSubscriptions, sub.DataType)

				log.Infof("Unsubscribed from %s", sub.DataType)
			}*/
	})

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
