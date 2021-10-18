package environments

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/equinor/radix-api/api/alerting"
	alertingModels "github.com/equinor/radix-api/api/alerting/models"
	"github.com/equinor/radix-api/api/deployments"
	environmentmodels "github.com/equinor/radix-api/api/environments/models"
	"github.com/equinor/radix-api/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	crdutils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/gorilla/mux"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			Path:        rootPath + "/environments/{envName}/alerting",
			Method:      "PUT",
			HandlerFunc: EnvironmentRouteWrapperFunc(UpdateAlertingConfig),
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/alerting",
			Method:      http.MethodGet,
			HandlerFunc: EnvironmentRouteWrapperFunc(GetAlertingConfig),
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/alerting/enable",
			Method:      http.MethodPost,
			HandlerFunc: EnvironmentRouteWrapperFunc(EnableAlerting),
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/alerting/disable",
			Method:      http.MethodPost,
			HandlerFunc: EnvironmentRouteWrapperFunc(DisableAlerting),
		},
	}

	return routes
}

// EnvironmentRouteWrapperFunc gets appName and envName from route and verifies that environment exists
// Returns 404 NotFound if environment is not defined, otherwise calls handler
func EnvironmentRouteWrapperFunc(handler models.RadixHandlerFunc) models.RadixHandlerFunc {
	return func(a models.Accounts, rw http.ResponseWriter, r *http.Request) {
		appName := mux.Vars(r)["appName"]
		envName := mux.Vars(r)["envName"]
		envNamespace := crdutils.GetEnvironmentNamespace(appName, envName)

		if _, err := a.UserAccount.RadixClient.RadixV1().RadixEnvironments().Get(context.TODO(), envNamespace, v1.GetOptions{}); err != nil {
			radixhttp.ErrorResponse(rw, r, err)
			return
		}

		handler(a, rw, r)
	}
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

	// Need in cluster client in order to delete namespace using sufficient privileges
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

	var secretParameters environmentmodels.SecretParameters
	if err := json.NewDecoder(r.Body).Decode(&secretParameters); err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	environmentHandler := Init(WithAccounts(accounts))

	_, err := environmentHandler.ChangeEnvironmentComponentSecret(appName, envName, componentName, secretName, secretParameters)
	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, "Success")
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
	err := environmentHandler.StopComponent(appName, envName, componentName)

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
	err := environmentHandler.StartComponent(appName, envName, componentName)

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
	//     - Starts the container again using up to date image
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
	err := environmentHandler.RestartComponent(appName, envName, componentName)

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
	//     - Starts all components in the environment again using up to date image
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
	//     - Starts all components in all environments of the application again using up to date image
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

	sinceTime := r.FormValue("sinceTime")

	var since time.Time
	var err error

	if !strings.EqualFold(strings.TrimSpace(sinceTime), "") {
		since, err = radixutils.ParseTimestamp(sinceTime)
		if err != nil {
			radixhttp.ErrorResponse(w, r, err)
			return
		}
	}

	eh := Init(WithAccounts(accounts))
	log, err := eh.GetLogs(appName, envName, podName, &since)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.StringResponse(w, r, log)
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

	sinceTime := r.FormValue("sinceTime")

	var since time.Time
	var err error

	if !strings.EqualFold(strings.TrimSpace(sinceTime), "") {
		since, err = radixutils.ParseTimestamp(sinceTime)
		if err != nil {
			radixhttp.ErrorResponse(w, r, err)
			return
		}
	}

	eh := Init(WithAccounts(accounts))
	log, err := eh.GetScheduledJobLogs(appName, envName, scheduledJobName, &since)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.StringResponse(w, r, log)
}

// UpdateAlertingConfig Configures alert settings
func UpdateAlertingConfig(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation PUT /applications/{appName}/environments/{envName}/alerting environment updateAlertingConfig
	// ---
	// summary: Update alerts configuration for an environment
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
	// - name: alertsConfig
	//   in: body
	//   description: Alerts configuration
	//   required: true
	//   schema:
	//       "$ref": "#/definitions/UpdateAlertingConfig"
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
	//     description: Successful alerts config update
	//     schema:
	//        "$ref": "#/definitions/AlertingConfig"
	//   "400":
	//     description: "Invalid configuration"
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

	var updateAlertingConfig alertingModels.UpdateAlertingConfig
	if err := json.NewDecoder(r.Body).Decode(&updateAlertingConfig); err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	alertHandler := alerting.NewEnvironmentHandler(accounts, appName, envName)
	alertsConfig, err := alertHandler.UpdateAlertingConfig(updateAlertingConfig)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, alertsConfig)
}

// GetAlertingConfig returns alerts configuration
func GetAlertingConfig(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/alerting environment getAlertingConfig
	// ---
	// summary: Get alerts configuration for an environment
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
	//     description: Successful get alerts config
	//     schema:
	//        "$ref": "#/definitions/AlertingConfig"
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

	alertHandler := alerting.NewEnvironmentHandler(accounts, appName, envName)
	alertsConfig, err := alertHandler.GetAlertingConfig()

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, alertsConfig)
}

// EnableAlerting enables alerting for application environment
func EnableAlerting(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/alerting/enable environment enableAlerting
	// ---
	// summary: Enable alerting for an environment
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
	//     description: Successful enable alerting
	//     schema:
	//        "$ref": "#/definitions/AlertingConfig"
	//   "400":
	//     description: "Alerting already enabled"
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

	alertHandler := alerting.NewEnvironmentHandler(accounts, appName, envName)
	alertsConfig, err := alertHandler.EnableAlerting()

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, alertsConfig)
}

// DisableAlerting disables alerting for application environment
func DisableAlerting(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/environments/{envName}/alerting/disable environment disableAlerting
	// ---
	// summary: Disable alerting for an environment
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
	//   "204":
	//     description: Successful disable alerting
	//   "400":
	//     description: "Alerting already enabled"
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

	alertHandler := alerting.NewEnvironmentHandler(accounts, appName, envName)
	err := alertHandler.DisableAlerting()

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
