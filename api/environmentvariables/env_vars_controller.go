package environmentvariables

import (
	//"encoding/json"
	//environmentmodels "github.com/equinor/radix-api/api/environments/models"
	"github.com/equinor/radix-api/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"net/http"
)

const rootPath = "/applications/{appName}"

type envVarsController struct {
	*models.DefaultController
}

// NewEnvVarsController Constructor
func NewEnvVarsController() models.Controller {
	return &envVarsController{}
}

// GetRoutes List the supported routes of this handler
func (ec *envVarsController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/envvars",
			Method:      "GET",
			HandlerFunc: GetComponentEnvVars,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/envvars/{envVarName}",
			Method:      "PUT",
			HandlerFunc: ChangeEnvVar,
		},
	}

	return routes
}

// GetComponentEnvVars Get log from a scheduled job
func GetComponentEnvVars(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/components/{componentName}/envvars component envVars
	// ---
	// summary: Get environment variables for component
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
	//     description: "environment variables"
	//     schema:
	//        type: "array"
	//        items:
	//           "$ref": "#/definitions/EnvVar"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	componentName := mux.Vars(r)["componentName"]

	eh := Init(WithAccounts(accounts))
	envVars, err := eh.GetComponentEnvVars(appName, envName, componentName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, envVars)
}

// ChangeEnvVar Modifies an environment variable
func ChangeEnvVar(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation PUT /applications/{appName}/environments/{envName}/components/{componentName}/envvars/{envVarName} component changeEnvVar
	// ---
	// summary: Update an environment variable
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
	// - name: envVarName
	//   in: path
	//   description: environment variable name to be updated
	//   type: string
	//   required: true
	// - name: environment variable value and metadata
	//   in: body
	//   description: New value and metadata
	//   required: true
	//   schema:
	//       "$ref": "#/definitions/EnvVarParameters"
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
	envVarName := mux.Vars(r)["envVarName"]

	log.Debugf("%s,%s,%s,%s", appName, envName, componentName, envVarName)

	//var secretParameters environmentmodels.SecretParameters
	//if err := json.NewDecoder(r.Body).Decode(&secretParameters); err != nil {
	//	radixhttp.ErrorResponse(w, r, err)
	//	return
	//}
	//
	//environmentHandler := Init(WithAccounts(accounts))
	//
	//_, err := environmentHandler.ChangeEnvironmentComponentSecret(appName, envName, componentName, secretName, secretParameters)
	//if err != nil {
	//	radixhttp.ErrorResponse(w, r, err)
	//	return
	//}
	//
	radixhttp.JSONResponse(w, r, "Success")
}
