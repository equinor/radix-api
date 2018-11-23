package environments

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/statoil/radix-api/api/deployments"
	"github.com/statoil/radix-api/api/utils"
	"github.com/statoil/radix-api/models"
	"k8s.io/client-go/kubernetes"

	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
)

const rootPath = "/applications/{appName}"

type environmentController struct {
	*models.DefaultController
}

// NewEnvironmentController Constructor
func NewEnvironmentController() models.Controller {
	return &environmentController{}
}

// GetRoutes List the supported routes of this handler
func (ec *environmentController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/environments/{envName}/deployments",
			Method:      "GET",
			HandlerFunc: GetApplicationEnvironmentDeployments,
		},
	}

	return routes
}

// GetSubscriptions Lists subscriptions this handler offers
func (ec *environmentController) GetSubscriptions() models.Subscriptions {
	subscriptions := models.Subscriptions{}

	return subscriptions
}

// GetApplicationEnvironmentDeployments Lists the application environment deployments
func GetApplicationEnvironmentDeployments(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/deployments environment getApplicationEnvironmentDeployments
	// ---
	// summary: Lists the application environment deployments
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: envName
	//   in: path
	//   description: environment of Radix application
	//   type: string
	//   required: true
	// - name: latest
	//   in: query
	//   description: indicator to allow only listing latest
	//   type: boolean
	//   required: false
	// responses:
	//   "200":
	//     description: "Successful operation"
	//     schema:
	//        type: "array"
	//        items:
	//           "$ref": "#/definitions/Secret"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	latest := r.FormValue("latest")

	deploy := deployments.Init(client, radixclient)

	var err error
	var useLatest = false
	if strings.TrimSpace(latest) != "" {
		useLatest, err = strconv.ParseBool(r.FormValue("latest"))
		if err != nil {
			utils.ErrorResponse(w, r, err)
			return
		}
	}

	appEnvironmentDeployments, err := deploy.HandleGetDeployments(appName, envName, useLatest)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, appEnvironmentDeployments)
}

// GetEnvironmentSummary Lists the environments for an application
func GetEnvironmentSummary(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments environment getEnvironmentSummary
	// ---
	// summary: Lists the environments for an application
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     description: "Successful operation"
	//     schema:
	//        type: "array"
	//        items:
	//           "$ref": "#/definitions/EnvironmentSummary"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	var err error
	appEnvironments := appName

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, appEnvironments)
}
