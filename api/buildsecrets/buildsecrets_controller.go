package buildsecrets

import (
	"encoding/json"
	"net/http"

	environmentModels "github.com/equinor/radix-api/api/secrets/models"
	log "github.com/sirupsen/logrus"

	"github.com/equinor/radix-api/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	"github.com/gorilla/mux"
)

const rootPath = "/applications/{appName}"

type buildSecretsController struct {
	*models.DefaultController
}

// NewBuildSecretsController Constructor
func NewBuildSecretsController() models.Controller {
	return &buildSecretsController{}
}

// GetRoutes List the supported routes of this handler
func (dc *buildSecretsController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/buildsecrets",
			Method:      "GET",
			HandlerFunc: GetBuildSecrets,
		},
		models.Route{
			Path:        rootPath + "/buildsecrets/{secretName}",
			Method:      "PUT",
			HandlerFunc: ChangeBuildSecret,
		},
	}

	return routes
}

// GetBuildSecrets Lists build secrets
func GetBuildSecrets(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/buildsecrets application getBuildSecrets
	// ---
	// summary: Lists the application build secrets
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
	//   description: Works only with custom setup of cluster. Allow impersonation of a comma-seperated list of test groups (Required if Impersonate-User is set)
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     description: "Successful operation"
	//     schema:
	//        type: "array"
	//        items:
	//           "$ref": "#/definitions/BuildSecret"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	buildSecretsHandler := Init(accounts)
	buildSecrets, err := buildSecretsHandler.GetBuildSecrets(r.Context(), appName)

	if err != nil {
		if err = radixhttp.ErrorResponse(w, r, err); err != nil {
			log.Errorf("%s: failed to write response: %v", r.URL.Path, err)
		}
		return
	}

	if err = radixhttp.JSONResponse(w, r, buildSecrets); err != nil {
			log.Errorf("%s: failed to write response: %v", r.URL.Path, err)
		}
}

// ChangeBuildSecret Modifies an application build secret
func ChangeBuildSecret(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation PUT /applications/{appName}/buildsecrets/{secretName} application updateBuildSecretsSecretValue
	// ---
	// summary: Update an application build secret
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: secretName
	//   in: path
	//   description: name of secret
	//   type: string
	//   required: true
	// - name: secretValue
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
	//   description: Works only with custom setup of cluster. Allow impersonation of a comma-seperated list of test groups (Required if Impersonate-User is set)
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     description: success
	//   "400":
	//     description: "Invalid application"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"
	//   "409":
	//     description: "Conflict"
	appName := mux.Vars(r)["appName"]
	secretName := mux.Vars(r)["secretName"]

	var secretParameters environmentModels.SecretParameters
	if err := json.NewDecoder(r.Body).Decode(&secretParameters); err != nil {
		if err = radixhttp.ErrorResponse(w, r, err); err != nil {
			log.Errorf("%s: failed to write response: %v", r.URL.Path, err)
		}
		return
	}

	buildSecretsHandler := Init(accounts)
	err := buildSecretsHandler.ChangeBuildSecret(r.Context(), appName, secretName, secretParameters.SecretValue)

	if err != nil {
		if err = radixhttp.ErrorResponse(w, r, err); err != nil {
			log.Errorf("%s: failed to write response: %v", r.URL.Path, err)
		}
		return
	}

	if err = radixhttp.JSONResponse(w, r, "Success"); err != nil {
			log.Errorf("%s: failed to write response: %v", r.URL.Path, err)
		}
}
