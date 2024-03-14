package router

import (
	"fmt"
	"net/http"
	"os"

	"github.com/equinor/radix-api/api/defaults"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/models"
	"github.com/equinor/radix-api/swaggerui"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"github.com/rs/zerolog/log"
	"github.com/urfave/negroni/v3"
)

const (
	apiVersionRoute                 = "/api/v1"
	admissionControllerRootPath     = "/admissioncontrollers"
	buildstatusControllerRootPath   = "/buildstatus"
	healthControllerPath            = "/health/"
	radixDNSZoneEnvironmentVariable = "RADIX_DNS_ZONE"
	swaggerUIPath                   = "/swaggerui"
)

// NewServer Constructor function
func NewServer(clusterName string, kubeUtil utils.KubeUtil, controllers ...models.Controller) http.Handler {
	router := mux.NewRouter().StrictSlash(true)

	initializeSwaggerUI(router)
	initializeAPIServer(kubeUtil, router, controllers)
	initializeHealthEndpoint(router)

	serveMux := http.NewServeMux()
	serveMux.Handle(healthControllerPath, negroni.New(
		negroni.Wrap(router),
	))

	serveMux.Handle("/api/", negroni.New(
		negroni.Wrap(router),
	))

	// TODO: We should maybe have oauth to stop any non-radix user from being able to see the API
	serveMux.Handle("/swaggerui/", negroni.New(
		negroni.Wrap(router),
	))

	serveMux.Handle("/metrics", negroni.New(
		negroni.Wrap(promhttp.Handler()),
	))

	rec := negroni.NewRecovery()
	rec.PrintStack = false

	n := negroni.New(
		rec,
		setZerologLogger(zerologLoggerWithRequestId),
		zerologRequestLogger(),
	)
	n.UseHandler(serveMux)

	useOutClusterClient := kubeUtil.IsUseOutClusterClient()
	return getCORSHandler(clusterName, n, useOutClusterClient)
}

func getCORSHandler(clusterName string, handler http.Handler, useOutClusterClient bool) http.Handler {
	radixDNSZone := os.Getenv(defaults.RadixDNSZoneEnvironmentVariable)

	corsOptions := cors.Options{
		AllowedOrigins: []string{
			"http://localhost:3000",
			"http://localhost:3001",
			"http://127.0.0.1:3000",
			"http://localhost:8000",
			"http://localhost:8086", // For swaggerui testing
			// TODO: We should consider:
			// 1. "https://*.radix.equinor.com"
			// 2. Keep cors rules in ingresses
			fmt.Sprintf("https://console.%s", radixDNSZone),
			getHostName("web", "radix-web-console-qa", clusterName, radixDNSZone),
			getHostName("web", "radix-web-console-prod", clusterName, radixDNSZone),
			getHostName("web", "radix-web-console-dev", clusterName, radixDNSZone),
			// Due to active-cluster
			getActiveClusterHostName("web", "radix-web-console-qa", radixDNSZone),
			getActiveClusterHostName("web", "radix-web-console-prod", radixDNSZone),
			getActiveClusterHostName("web", "radix-web-console-dev", radixDNSZone),
		},
		AllowCredentials: true,
		MaxAge:           600,
		AllowedHeaders:   []string{"Accept", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization"},
		AllowedMethods:   []string{"GET", "PUT", "POST", "OPTIONS", "DELETE", "PATCH"},
	}

	if !useOutClusterClient {
		// debugging mode
		corsOptions.Debug = true
		corsLogger := log.Logger.With().Str("pkg", "cors-middleware").Logger()
		corsOptions.Logger = &corsLogger
		// necessary header to allow ajax requests directly from radix-web-console app in browser
		corsOptions.AllowedHeaders = append(corsOptions.AllowedHeaders, "X-Requested-With")
	}

	c := cors.New(corsOptions)

	return c.Handler(handler)
}

func getActiveClusterHostName(componentName, namespace, radixDNSZone string) string {
	return fmt.Sprintf("https://%s-%s.%s", componentName, namespace, radixDNSZone)
}

func getHostName(componentName, namespace, clustername, radixDNSZone string) string {
	return fmt.Sprintf("https://%s-%s.%s.%s", componentName, namespace, clustername, radixDNSZone)
}

func initializeAPIServer(kubeUtil utils.KubeUtil, router *mux.Router, controllers []models.Controller) {
	for _, controller := range controllers {
		for _, route := range controller.GetRoutes() {
			addHandlerRoute(kubeUtil, router, route)
		}
	}
}

func initializeSwaggerUI(router *mux.Router) {
	swaggerFsHandler := http.FileServer(http.FS(swaggerui.FS()))
	swaggerui := http.StripPrefix(swaggerUIPath, swaggerFsHandler)
	router.PathPrefix(swaggerUIPath).Handler(swaggerui)
}

func initializeHealthEndpoint(router *mux.Router) {
	router.HandleFunc(healthControllerPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods("GET")
}

func addHandlerRoute(kubeUtil utils.KubeUtil, router *mux.Router, route models.Route) {
	path := apiVersionRoute + route.Path
	router.HandleFunc(path,
		utils.NewRadixMiddleware(kubeUtil, path, route.Method, route.AllowUnauthenticatedUsers, route.KubeApiConfig.QPS, route.KubeApiConfig.Burst, route.HandlerFunc).Handle).Methods(route.Method)
}
