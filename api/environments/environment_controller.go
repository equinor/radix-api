package environments

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/statoil/radix-api/api/deployments"
	environmentModels "github.com/statoil/radix-api/api/environments/models"
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
		models.Route{
			Path:        rootPath + "/environments",
			Method:      "GET",
			HandlerFunc: GetEnvironmentSummary,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}",
			Method:      "GET",
			HandlerFunc: GetEnvironment,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/secrets/{secretName}",
			Method:      "PUT",
			HandlerFunc: ChangeEnvironmentComponentSecret,
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
	//           "$ref": "#/definitions/DeploymentSummary"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	latest := r.FormValue("latest")

	var err error
	var useLatest = false
	if strings.TrimSpace(latest) != "" {
		useLatest, err = strconv.ParseBool(r.FormValue("latest"))
		if err != nil {
			utils.ErrorResponse(w, r, err)
			return
		}
	}

	deploymentHandler := deployments.Init(client, radixclient)

	appEnvironmentDeployments, err := deploymentHandler.HandleGetDeployments(appName, envName, useLatest)
	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, appEnvironmentDeployments)
}

// GetEnvironment Get details for an application environment
func GetEnvironment(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName} environment getEnvironment
	// ---
	// summary: Get details for an application environment
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: envName
	//   in: path
	//   description: name of environment
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     description: "Successful operation"
	//     schema:
	//        type: "array"
	//        items:
	//           "$ref": "#/definitions/Environment"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]

	environmentHandler := Init(client, radixclient)
	appEnvironment, err := environmentHandler.HandleGetEnvironment(appName, envName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, appEnvironment)

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

	environmentHandler := Init(client, radixclient)
	appEnvironments, err := environmentHandler.HandleGetEnvironmentSummary(appName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, appEnvironments)
}

// ChangeEnvironmentComponentSecret Modifies an application environment component secret
func ChangeEnvironmentComponentSecret(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation PUT /applications/{appName}/environments/{envName}/components/{componentName}/secrets/{secretName} environment changeEnvironmentComponentSecret
	// ---
	// summary: Update an application environment component secret
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: envName
	//   in: path
	//   description: environment of Radix application
	//   type: string
	//   required: true
	// - name: componentName
	//   in: path
	//   description: environment component of Radix application
	//   type: string
	//   required: true
	// - name: secretName
	//   in: path
	//   description: environment component secret name to be updated
	//   type: string
	//   required: true
	// - name: componentSecret
	//   in: body
	//   description: New secret value
	//   required: true
	//   schema:
	//       "$ref": "#/definitions/SecretParameters"
	// responses:
	//   "200":
	// 	   description: "Success"
	//   "400":
	//     description: "Invalid application"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	//   "409":
	//     description: "Conflict"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	componentName := mux.Vars(r)["componentName"]
	secretName := mux.Vars(r)["secretName"]

	var secretParameters environmentModels.SecretParameters
	if err := json.NewDecoder(r.Body).Decode(&secretParameters); err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	environmentHandler := Init(client, radixclient)

	_, err := environmentHandler.HandleChangeEnvironmentComponentSecret(appName, envName, componentName, secretName, secretParameters)
	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, "Success")
}
