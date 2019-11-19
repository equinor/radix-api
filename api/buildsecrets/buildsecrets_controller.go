package buildsecrets

import (
	"encoding/json"
	"net/http"

	environmentModels "github.com/equinor/radix-api/api/environments/models"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/models"
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
func GetBuildSecrets(clients models.Clients, w http.ResponseWriter, r *http.Request) {
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
	//   description: Works only with custom setup of cluster. Allow impersonation of test group (Required if Impersonate-User is set)
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

	buildSecretsHandler := Init(clients.OutClusterClient, clients.OutClusterRadixClient, clients.InClusterClient, clients.InClusterRadixClient)
	buildSecrets, err := buildSecretsHandler.GetBuildSecrets(appName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, buildSecrets)
}

// ChangeBuildSecret Modifies an application build secret
func ChangeBuildSecret(clients models.Clients, w http.ResponseWriter, r *http.Request) {
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
	secretName := mux.Vars(r)["secretName"]

	var secretParameters environmentModels.SecretParameters
	if err := json.NewDecoder(r.Body).Decode(&secretParameters); err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	buildSecretsHandler := Init(clients.OutClusterClient, clients.OutClusterRadixClient, clients.InClusterClient, clients.InClusterRadixClient)
	err := buildSecretsHandler.ChangeBuildSecret(appName, secretName, secretParameters.SecretValue)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, "Success")
}
