package deployments

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/models"
	"github.com/gorilla/mux"
)

const rootPath = "/applications/{appName}"

type deploymentController struct {
	*models.DefaultController
}

// NewDeploymentController Constructor
func NewDeploymentController() models.Controller {
	return &deploymentController{}
}

// GetRoutes List the supported routes of this handler
func (dc *deploymentController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/deployments",
			Method:      "GET",
			HandlerFunc: GetDeployments,
		},
		models.Route{
			Path:        rootPath + "/deployments/{deploymentName}",
			Method:      "GET",
			HandlerFunc: GetDeployment,
		},
		models.Route{
			Path:        rootPath + "/deployments/{deploymentName}/components/{componentName}/replicas/{podName}/logs",
			Method:      "GET",
			HandlerFunc: GetPodLog,
		},
		models.Route{
			Path:        rootPath + "/deployments/{deploymentName}/components",
			Method:      "GET",
			HandlerFunc: GetComponents,
		},
	}

	return routes
}

// GetDeployments Lists deployments
func GetDeployments(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/deployments application getDeployments
	// ---
	// summary: Lists the application deployments
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: environment
	//   in: query
	//   description: environment of Radix application
	//   type: string
	//   required: false
	// - name: latest
	//   in: query
	//   description: indicator to allow only listing latest
	//   type: boolean
	//   required: false
	// - name: Impersonate-User
	//   in: header
	//   description: Works only with custom setup of cluster. Allow impersonation of test users (Required if Impersonate-Group is set)
	//   type: string
	//   required: false
	// - name: Impersonate-Group
	//   in: header
	//   description: Works only with custom setup of cluster. Allow impersonation of test group (Required if Impersonate-User is set)
	//   type: string
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
	environment := r.FormValue("environment")
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

	deployHandler := Init(accounts)
	appDeployments, err := deployHandler.GetDeploymentsForApplicationEnvironment(appName, environment, useLatest)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, appDeployments)
}

// GetDeployment Get deployment details
func GetDeployment(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/deployments/{deploymentName} deployment getDeployment
	// ---
	// summary: Get deployment details
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: deploymentName
	//   in: path
	//   description: name of deployment
	//   type: string
	//   required: true
	// - name: Impersonate-User
	//   in: header
	//   description: Works only with custom setup of cluster. Allow impersonation of test users (Required if Impersonate-Group is set)
	//   type: string
	//   required: false
	// - name: Impersonate-Group
	//   in: header
	//   description: Works only with custom setup of cluster. Allow impersonation of test group (Required if Impersonate-User is set)
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     description: "Successful get deployment"
	//     schema:
	//        "$ref": "#/definitions/Deployment"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	deploymentName := mux.Vars(r)["deploymentName"]

	deployHandler := Init(accounts)
	appDeployment, err := deployHandler.GetDeploymentWithName(appName, deploymentName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, appDeployment)
}

// GetComponents for a deployment
func GetComponents(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/deployments/{deploymentName}/components component components
	// ---
	// summary: Get components for a deployment
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: deploymentName
	//   in: path
	//   description: Name of deployment
	//   type: string
	//   required: true
	// - name: Impersonate-User
	//   in: header
	//   description: Works only with custom setup of cluster. Allow impersonation of test users (Required if Impersonate-Group is set)
	//   type: string
	//   required: false
	// - name: Impersonate-Group
	//   in: header
	//   description: Works only with custom setup of cluster. Allow impersonation of test group (Required if Impersonate-User is set)
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     description: "pod log"
	//     schema:
	//        type: "array"
	//        items:
	//           "$ref": "#/definitions/Component"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	deploymentName := mux.Vars(r)["deploymentName"]

	deployHandler := Init(accounts)
	components, err := deployHandler.GetComponentsForDeploymentName(appName, deploymentName)
	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, components)
}

// GetPodLog Get logs of a single pod
func GetPodLog(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/deployments/{deploymentName}/components/{componentName}/replicas/{podName}/logs component log
	// ---
	// summary: Get logs from a deployed pod
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: deploymentName
	//   in: path
	//   description: Name of deployment
	//   type: string
	//   required: true
	// - name: componentName
	//   in: path
	//   description: Name of component
	//   type: string
	//   required: true
	// - name: podName
	//   in: path
	//   description: Name of pod
	//   type: string
	//   required: true
	// - name: Impersonate-User
	//   in: header
	//   description: Works only with custom setup of cluster. Allow impersonation of test users (Required if Impersonate-Group is set)
	//   type: string
	//   required: false
	// - name: Impersonate-Group
	//   in: header
	//   description: Works only with custom setup of cluster. Allow impersonation of test group (Required if Impersonate-User is set)
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     description: "pod log"
	//     schema:
	//        type: "string"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	// deploymentName := mux.Vars(r)["deploymentName"]
	// componentName := mux.Vars(r)["componentName"]
	podName := mux.Vars(r)["podName"]

	deployHandler := Init(accounts)
	log, err := deployHandler.GetLogs(appName, podName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.StringResponse(w, r, log)
}
