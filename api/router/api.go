package router

import (
	"net/http"

	"github.com/equinor/radix-api/api/middleware/auth"
	"github.com/equinor/radix-api/api/middleware/cors"
	"github.com/equinor/radix-api/api/middleware/logger"
	"github.com/equinor/radix-api/api/middleware/recovery"
	"github.com/equinor/radix-api/api/utils"
	token "github.com/equinor/radix-api/api/utils/authn"
	"github.com/equinor/radix-api/models"
	"github.com/equinor/radix-api/swaggerui"
	"github.com/gorilla/mux"
	"github.com/urfave/negroni/v3"
)

const (
	apiVersionRoute = "/api/v1"
)

// NewAPIHandler Constructor function
func NewAPIHandler(clusterName string, validator token.ValidatorInterface, radixDNSZone string, kubeUtil utils.KubeUtil, controllers ...models.Controller) http.Handler {
	serveMux := http.NewServeMux()
	serveMux.Handle("/health/", createHealthHandler())
	serveMux.Handle("/swaggerui/", createSwaggerHandler())
	serveMux.Handle("/api/", createApiRouter(kubeUtil, controllers))

	n := negroni.New(
		recovery.CreateMiddleware(),
		cors.CreateMiddleware(clusterName, radixDNSZone),
		logger.CreateZerologRequestIdMiddleware(),
		logger.CreateZerologRequestDetailsMiddleware(),
		auth.CreateAuthenticationMiddleware(validator),
		logger.CreateZerologRequestLoggerMiddleware(),
	)
	n.UseHandler(serveMux)

	return n
}
func createApiRouter(kubeUtil utils.KubeUtil, controllers []models.Controller) *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	for _, controller := range controllers {
		for _, route := range controller.GetRoutes() {
			path := apiVersionRoute + route.Path
			handler := utils.NewRadixMiddleware(
				kubeUtil,
				path,
				route.Method,
				route.AllowUnauthenticatedUsers,
				route.KubeApiConfig.QPS,
				route.KubeApiConfig.Burst,
				route.HandlerFunc,
			)

			n := negroni.New()
			if !route.AllowUnauthenticatedUsers {
				n.Use(auth.CreateAuthorizeRequiredMiddleware())
			}
			n.UseHandler(handler)
			router.Handle(path, n).Methods(route.Method)
		}
	}
	return router
}

func createSwaggerHandler() http.Handler {
	swaggerFsHandler := http.FileServer(http.FS(swaggerui.FS()))
	swaggerui := http.StripPrefix("swaggerui", swaggerFsHandler)

	return swaggerui
}

func createHealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}
