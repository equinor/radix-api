package environments

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/equinor/radix-api/api/deployments"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/models"
	"github.com/gorilla/mux"
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
			Path:        rootPath + "/environments/{envName}",
			Method:      "POST",
			HandlerFunc: CreateEnvironment,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}",
			Method:      "DELETE",
			HandlerFunc: DeleteEnvironment,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/secrets/{secretName}",
			Method:      "PUT",
			HandlerFunc: ChangeEnvironmentComponentSecret,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/stop",
			Method:      "POST",
			HandlerFunc: StopComponent,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/start",
			Method:      "POST",
			HandlerFunc: StartComponent,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/restart",
			Method:      "POST",
			HandlerFunc: RestartComponent,
		},
	}

	return routes
}

// GetApplicationEnvironmentDeployments Lists the application environment deployments
func GetApplicationEnvironmentDeployments(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
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

	deploymentHandler := deployments.Init(accounts)

	appEnvironmentDeployments, err := deploymentHandler.GetDeploymentsForApplicationEnvironment(appName, envName, useLatest)
	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, appEnvironmentDeployments)
}

// CreateEnvironment Creates a new environment
func CreateEnvironment(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName} environment createEnvironment
	// ---
	// summary: Creates application environment
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
	//     description: "Environment created ok"
	//   "401":
	//     description: "Unauthorized"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]

	// Need in cluster client in order to delete namespace using sufficient priviledges
	environmentHandler := Init(accounts)
	_, err := environmentHandler.CreateEnvironment(appName, envName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetEnvironment Get details for an application environment
func GetEnvironment(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
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
	//     description: "Successful get environment"
	//     schema:
	//        "$ref": "#/definitions/Environment"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]

	environmentHandler := Init(accounts)
	appEnvironment, err := environmentHandler.GetEnvironment(appName, envName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, appEnvironment)

}

// DeleteEnvironment Deletes environment
func DeleteEnvironment(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation DELETE /applications/{appName}/environments/{envName} environment deleteEnvironment
	// ---
	// summary: Deletes application environment
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
	//     description: "Environment deleted ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]

	// Need in cluster client in order to delete namespace using sufficient priviledges
	environmentHandler := Init(accounts)
	err := environmentHandler.DeleteEnvironment(appName, envName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetEnvironmentSummary Lists the environments for an application
func GetEnvironmentSummary(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments environment getEnvironmentSummary
	// ---
	// summary: Lists the environments for an application
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
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

	environmentHandler := Init(accounts)
	appEnvironments, err := environmentHandler.GetEnvironmentSummary(appName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, appEnvironments)
}

// ChangeEnvironmentComponentSecret Modifies an application environment component secret
func ChangeEnvironmentComponentSecret(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
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
	//     description: success
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

	environmentHandler := Init(accounts)

	_, err := environmentHandler.ChangeEnvironmentComponentSecret(appName, envName, componentName, secretName, secretParameters)
	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, "Success")
}

// StopComponent Stops job
func StopComponent(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/components/{componentName}/stop component stopComponent
	// ---
	// summary: Stops component
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: envName
	//   in: path
	//   description: Name of environment
	//   type: string
	//   required: true
	// - name: componentName
	//   in: path
	//   description: Name of component
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
	//     description: "Component stopped ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	componentName := mux.Vars(r)["componentName"]

	environmentHandler := Init(accounts)
	err := environmentHandler.StopComponent(appName, envName, componentName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, "Success")
}

// StartComponent Starts job
func StartComponent(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/components/{componentName}/start component startComponent
	// ---
	// summary: Start component
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: envName
	//   in: path
	//   description: Name of environment
	//   type: string
	//   required: true
	// - name: componentName
	//   in: path
	//   description: Name of component
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
	//     description: "Component started ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	componentName := mux.Vars(r)["componentName"]

	environmentHandler := Init(accounts)
	err := environmentHandler.StartComponent(appName, envName, componentName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, "Success")
}

// RestartComponent Restarts job
func RestartComponent(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/components/{componentName}/restart component restartComponent
	// ---
	// summary: Restart component
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: envName
	//   in: path
	//   description: Name of environment
	//   type: string
	//   required: true
	// - name: componentName
	//   in: path
	//   description: Name of component
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
	//     description: "Component started ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	componentName := mux.Vars(r)["componentName"]

	environmentHandler := Init(accounts)
	err := environmentHandler.RestartComponent(appName, envName, componentName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, "Success")
}
