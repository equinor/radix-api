package environments

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/equinor/radix-api/api/utils/logs"

	"github.com/equinor/radix-api/api/deployments"
	"github.com/equinor/radix-api/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
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
			Path:        rootPath + "/environments/{envName}/events",
			Method:      "GET",
			HandlerFunc: GetEnvironmentEvents,
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
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/aux/oauth/restart",
			Method:      "POST",
			HandlerFunc: RestartOAuthAuxiliaryResource,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/stop",
			Method:      "POST",
			HandlerFunc: StopEnvironment,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/start",
			Method:      "POST",
			HandlerFunc: StartEnvironment,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/restart",
			Method:      "POST",
			HandlerFunc: RestartEnvironment,
		},
		models.Route{
			Path:        rootPath + "/stop",
			Method:      "POST",
			HandlerFunc: StopApplication,
		},
		models.Route{
			Path:        rootPath + "/start",
			Method:      "POST",
			HandlerFunc: StartApplication,
		},
		models.Route{
			Path:        rootPath + "/restart",
			Method:      "POST",
			HandlerFunc: RestartApplication,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/replicas/{podName}/logs",
			Method:      "GET",
			HandlerFunc: GetPodLog,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/scheduledjobs/{scheduledJobName}/logs",
			Method:      "GET",
			HandlerFunc: GetScheduledJobLog,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/aux/oauth/replicas/{podName}/logs",
			Method:      "GET",
			HandlerFunc: GetOAuthAuxiliaryResourcePodLog,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/jobs",
			Method:      "GET",
			HandlerFunc: GetJobs,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/jobs/{jobName}",
			Method:      "GET",
			HandlerFunc: GetJob,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/jobs/{jobName}/stop",
			Method:      "POST",
			HandlerFunc: StopJob,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/jobs/{jobName}",
			Method:      "DELETE",
			HandlerFunc: DeleteJob,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/jobs/{jobName}/payload",
			Method:      "GET",
			HandlerFunc: GetJobPayload,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/batches",
			Method:      "GET",
			HandlerFunc: GetBatches,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/batches/{batchName}",
			Method:      "GET",
			HandlerFunc: GetBatch,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/batches/{batchName}/stop",
			Method:      "POST",
			HandlerFunc: StopBatch,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/jobcomponents/{jobComponentName}/batches/{batchName}",
			Method:      "DELETE",
			HandlerFunc: DeleteBatch,
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
	//   description: indicator to allow only listing the latest
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
			radixhttp.ErrorResponse(w, r, err)
			return
		}
	}

	deploymentHandler := deployments.Init(accounts)

	appEnvironmentDeployments, err := deploymentHandler.GetDeploymentsForApplicationEnvironment(appName, envName, useLatest)
	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, appEnvironmentDeployments)
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

	// Need in cluster client in order to delete namespace using sufficient privileges
	environmentHandler := Init(WithAccounts(accounts))
	_, err := environmentHandler.CreateEnvironment(appName, envName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
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

	environmentHandler := Init(WithAccounts(accounts))
	appEnvironment, err := environmentHandler.GetEnvironment(appName, envName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, appEnvironment)

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

	environmentHandler := Init(WithAccounts(accounts))
	err := environmentHandler.DeleteEnvironment(appName, envName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
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

	environmentHandler := Init(WithAccounts(accounts))
	appEnvironments, err := environmentHandler.GetEnvironmentSummary(appName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, appEnvironments)
}

// GetEnvironmentEvents Get events for an application environment
func GetEnvironmentEvents(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/events environment getEnvironmentEvents
	// ---
	// summary: Lists events for an application environment
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
	//     description: "Successful get environment events"
	//     schema:
	//        "$ref": "#/definitions/Event"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]

	environmentHandler := Init(WithAccounts(accounts))
	events, err := environmentHandler.GetEnvironmentEvents(appName, envName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, events)

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

	environmentHandler := Init(WithAccounts(accounts))
	err := environmentHandler.StopComponent(appName, envName, componentName, false)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, "Success")
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

	environmentHandler := Init(WithAccounts(accounts))
	err := environmentHandler.StartComponent(appName, envName, componentName, false)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, "Success")
}

// RestartComponent Restarts job
func RestartComponent(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/components/{componentName}/restart component restartComponent
	// ---
	// summary: |
	//   Restart a component
	//     - Stops running the component container
	//     - Pulls new image from image hub in radix configuration
	//     - Starts the container again using an up-to-date image
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

	environmentHandler := Init(WithAccounts(accounts))
	err := environmentHandler.RestartComponent(appName, envName, componentName, false)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, "Success")
}

// StopEnvironment  all components in the environment
func StopEnvironment(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/stop environment stopEnvironment
	// ---
	// summary: Stops all components in the environment
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
	//     description: "Environment stopped ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]

	environmentHandler := Init(WithAccounts(accounts))
	err := environmentHandler.StopEnvironment(appName, envName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, "Success")
}

// StartEnvironment Starts all components in the environment
func StartEnvironment(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/start environment startEnvironment
	// ---
	// summary: Start all components in the environment
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
	//     description: "Environment started ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]

	environmentHandler := Init(WithAccounts(accounts))
	err := environmentHandler.StartEnvironment(appName, envName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, "Success")
}

// RestartEnvironment Restarts all components in the environment
func RestartEnvironment(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/restart environment restartEnvironment
	// ---
	// summary: |
	//   Restart all components in the environment
	//     - Stops all running components in the environment
	//     - Pulls new images from image hub in radix configuration
	//     - Starts all components in the environment again using up-to-date image
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
	//     description: "Environment started ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]

	environmentHandler := Init(WithAccounts(accounts))
	err := environmentHandler.RestartEnvironment(appName, envName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, "Success")
}

// StopApplication  all components in all environments of the application
func StopApplication(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/stop application stopApplication
	// ---
	// summary: Stops all components in the environment
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
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
	//     description: "Application stopped ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	environmentHandler := Init(WithAccounts(accounts))
	err := environmentHandler.StopApplication(appName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, "Success")
}

// StartApplication Starts all components in all environments of the application
func StartApplication(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/start application startApplication
	// ---
	// summary: Start all components in all environments of the application
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
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
	//     description: "Application started ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	environmentHandler := Init(WithAccounts(accounts))
	err := environmentHandler.StartApplication(appName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, "Success")
}

// RestartApplication Restarts all components in all environments of the application
func RestartApplication(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/restart application restartApplication
	// ---
	// summary: |
	//   Restart all components in all environments of the application
	//     - Stops all running components in all environments of the application
	//     - Pulls new images from image hub in radix configuration
	//     - Starts all components in all environments of the application again using up-to-date image
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
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
	//     description: "Application started ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	environmentHandler := Init(WithAccounts(accounts))
	err := environmentHandler.RestartApplication(appName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, "Success")
}

// RestartOAuthAuxiliaryResource Restarts oauth auxiliary resource for a component
func RestartOAuthAuxiliaryResource(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/components/{componentName}/aux/oauth/restart component restartOAuthAuxiliaryResource
	// ---
	// summary: Restarts an auxiliary resource for a component
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
	//     description: "Auxiliary resource restarted ok"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "409":
	//     description: "Conflict"
	//   "404":
	//     description: "Not found"
	//   "500":
	//     description: "Internal server error"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	componentName := mux.Vars(r)["componentName"]

	environmentHandler := Init(WithAccounts(accounts))
	err := environmentHandler.RestartComponentAuxiliaryResource(appName, envName, componentName, defaults.OAuthProxyAuxiliaryComponentType)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, "Success")
}

// GetPodLog Get logs of a single pod
func GetPodLog(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/components/{componentName}/replicas/{podName}/logs component replicaLog
	// ---
	// summary: Get logs from a deployed pod
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
	// - name: podName
	//   in: path
	//   description: Name of pod
	//   type: string
	//   required: true
	// - name: sinceTime
	//   in: query
	//   description: Get log only from sinceTime (example 2020-03-18T07:20:41+00:00)
	//   type: string
	//   format: date-time
	//   required: false
	// - name: lines
	//   in: query
	//   description: Get log lines (example 1000)
	//   type: string
	//   format: number
	//   required: false
	// - name: file
	//   in: query
	//   description: Get log as a file if true
	//   type: string
	//   format: boolean
	//   required: false
	// - name: previous
	//   in: query
	//   description: Get previous container log if true
	//   type: string
	//   format: boolean
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
	//     description: "pod log"
	//     schema:
	//        type: "string"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	podName := mux.Vars(r)["podName"]

	since, asFile, logLines, err, previousLog := logs.GetLogParams(r)
	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	eh := Init(WithAccounts(accounts))
	log, err := eh.GetLogs(appName, envName, podName, &since, logLines, previousLog)
	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}
	defer log.Close()

	if asFile {
		fileName := fmt.Sprintf("%s.log", time.Now().Format("20060102150405"))
		radixhttp.ReaderFileResponse(w, log, fileName, "text/plain; charset=utf-8")
	} else {
		radixhttp.ReaderResponse(w, log, "text/plain; charset=utf-8")
	}
}

// GetScheduledJobLog Get log from a scheduled job
func GetScheduledJobLog(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/scheduledjobs/{scheduledJobName}/logs job jobLog
	// ---
	// summary: Get log from a scheduled job
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
	//   type: string
	//   required: true
	// - name: scheduledJobName
	//   in: path
	//   description: Name of scheduled job
	//   type: string
	//   required: true
	// - name: sinceTime
	//   in: query
	//   description: Get log only from sinceTime (example 2020-03-18T07:20:41+00:00)
	//   type: string
	//   format: date-time
	//   required: false
	// - name: lines
	//   in: query
	//   description: Get log lines (example 1000)
	//   type: string
	//   format: number
	//   required: false
	// - name: file
	//   in: query
	//   description: Get log as a file if true
	//   type: string
	//   format: boolean
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
	//     description: "scheduled job log"
	//     schema:
	//        type: "string"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	scheduledJobName := mux.Vars(r)["scheduledJobName"]

	since, asFile, logLines, err, _ := logs.GetLogParams(r)
	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	eh := Init(WithAccounts(accounts))
	log, err := eh.GetScheduledJobLogs(appName, envName, scheduledJobName, &since, logLines)
	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}
	defer log.Close()

	if asFile {
		fileName := fmt.Sprintf("%s.log", time.Now().Format("20060102150405"))
		radixhttp.ReaderFileResponse(w, log, fileName, "text/plain; charset=utf-8")
	} else {
		radixhttp.ReaderResponse(w, log, "text/plain; charset=utf-8")
	}
}

// GetJobs Get list of scheduled jobs
func GetJobs(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/jobs job getJobs
	// ---
	// summary: Get list of scheduled jobs
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
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
	//     description: "scheduled jobs"
	//     schema:
	//        type: array
	//        items:
	//          "$ref": "#/definitions/ScheduledJobSummary"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]

	eh := Init(WithAccounts(accounts))
	jobSummaries, err := eh.GetJobs(appName, envName, jobComponentName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, jobSummaries)
}

// GetJob Get a scheduled job
func GetJob(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/jobs/{jobName} job getJob
	// ---
	// summary: Get list of scheduled jobs
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: Name of job
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
	//     description: "scheduled job"
	//     schema:
	//        "$ref": "#/definitions/ScheduledJobSummary"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]
	jobName := mux.Vars(r)["jobName"]

	eh := Init(WithAccounts(accounts))
	jobSummary, err := eh.GetJob(appName, envName, jobComponentName, jobName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, jobSummary)
}

// StopJob Stop a scheduled job
func StopJob(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/jobs/{jobName}/stop job stopJob
	// ---
	// summary: Stop scheduled job
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: Name of job
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
	//   "204":
	//     description: "Success"
	//   "400":
	//     description: "Invalid job"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]
	jobName := mux.Vars(r)["jobName"]

	eh := Init(WithAccounts(accounts))
	err := eh.StopJob(appName, envName, jobComponentName, jobName)
	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteJob Delete a job
func DeleteJob(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation DELETE /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/jobs/{jobName} job deleteJob
	// ---
	// summary: Delete job
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: Name of job
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
	//   "204":
	//     description: "Success"
	//   "400":
	//     description: "Invalid job"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]
	jobName := mux.Vars(r)["jobName"]

	eh := Init(WithAccounts(accounts))
	err := eh.DeleteJob(appName, envName, jobComponentName, jobName)
	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetBatches Get list of scheduled batches
func GetBatches(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/batches job getBatches
	// ---
	// summary: Get list of scheduled batches
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
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
	//     description: "scheduled batches"
	//     schema:
	//        type: array
	//        items:
	//          "$ref": "#/definitions/ScheduledBatchSummary"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]

	eh := Init(WithAccounts(accounts))
	batchSummaries, err := eh.GetBatches(appName, envName, jobComponentName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, batchSummaries)
}

// GetBatch Get a scheduled batch
func GetBatch(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/batches/{batchName} job getBatch
	// ---
	// summary: Get list of scheduled batches
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
	//   type: string
	//   required: true
	// - name: batchName
	//   in: path
	//   description: Name of batch
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
	//     description: "scheduled batch"
	//     schema:
	//        "$ref": "#/definitions/ScheduledBatchSummary"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]
	batchName := mux.Vars(r)["batchName"]

	eh := Init(WithAccounts(accounts))
	jobSummary, err := eh.GetBatch(appName, envName, jobComponentName, batchName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, jobSummary)
}

// StopBatch Stop a scheduled batch
func StopBatch(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/batches/{batchName}/stop job stopBatch
	// ---
	// summary: Stop scheduled batch
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
	//   type: string
	//   required: true
	// - name: batchName
	//   in: path
	//   description: Name of batch
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
	//   "204":
	//     description: "Success"
	//   "400":
	//     description: "Invalid batch"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]
	batchName := mux.Vars(r)["batchName"]

	eh := Init(WithAccounts(accounts))
	err := eh.StopBatch(appName, envName, jobComponentName, batchName)
	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteBatch Delete a batch
func DeleteBatch(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation DELETE /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/batches/{batchName} job deleteBatch
	// ---
	// summary: Delete batch
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
	//   type: string
	//   required: true
	// - name: batchName
	//   in: path
	//   description: Name of batch
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
	//   "204":
	//     description: "Success"
	//   "400":
	//     description: "Invalid batch"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]
	batchName := mux.Vars(r)["batchName"]

	eh := Init(WithAccounts(accounts))
	err := eh.DeleteBatch(appName, envName, jobComponentName, batchName)
	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetOAuthAuxiliaryResourcePodLog Get log for a single auxiliary resource pod
func GetOAuthAuxiliaryResourcePodLog(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/components/{componentName}/aux/oauth/replicas/{podName}/logs component getOAuthPodLog
	// ---
	// summary: Get logs for an oauth auxiliary resource pod
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
	// - name: podName
	//   in: path
	//   description: Name of pod
	//   type: string
	//   required: true
	// - name: sinceTime
	//   in: query
	//   description: Get log only from sinceTime (example 2020-03-18T07:20:41+00:00)
	//   type: string
	//   format: date-time
	//   required: false
	// - name: lines
	//   in: query
	//   description: Get log lines (example 1000)
	//   type: string
	//   format: number
	//   required: false
	// - name: file
	//   in: query
	//   description: Get log as a file if true
	//   type: string
	//   format: boolean
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
	//     description: "pod log"
	//     schema:
	//        type: "string"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"
	//   "500":
	//     description: "Internal server error"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	componentName := mux.Vars(r)["componentName"]
	podName := mux.Vars(r)["podName"]

	since, asFile, logLines, err, _ := logs.GetLogParams(r)
	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	eh := Init(WithAccounts(accounts))
	log, err := eh.GetAuxiliaryResourcePodLog(appName, envName, componentName, defaults.OAuthProxyAuxiliaryComponentType, podName, &since, logLines)
	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}
	defer log.Close()

	if asFile {
		fileName := fmt.Sprintf("%s.log", time.Now().Format("20060102150405"))
		radixhttp.ReaderFileResponse(w, log, fileName, "text/plain; charset=utf-8")
	} else {
		radixhttp.ReaderResponse(w, log, "text/plain; charset=utf-8")
	}
}

// GetJobPayload Get a scheduled job payload
func GetJobPayload(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/jobcomponents/{jobComponentName}/jobs/{jobName}/payload job getJobPayload
	// ---
	// summary: Get payload of a scheduled job
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
	// - name: jobComponentName
	//   in: path
	//   description: Name of job-component
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: Name of job
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
	//     description: "scheduled job payload"
	//     schema:
	//        type: "string"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	jobComponentName := mux.Vars(r)["jobComponentName"]
	jobName := mux.Vars(r)["jobName"]

	eh := Init(WithAccounts(accounts))
	payload, err := eh.GetJobPayload(appName, envName, jobComponentName, jobName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.ReaderResponse(w, payload, "text/plain; charset=utf-8")
}
