package privateimagehubs

import (
	"encoding/json"
	"net/http"

	environmentModels "github.com/equinor/radix-api/api/environments/models"
	"github.com/equinor/radix-api/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	"github.com/gorilla/mux"
)

const rootPath = "/applications/{appName}"

type privateImageHubController struct {
	*models.DefaultController
}

// NewPrivateImageHubController Constructor
func NewPrivateImageHubController() models.Controller {
	return &privateImageHubController{}
}

// GetRoutes List the supported routes of this handler
func (dc *privateImageHubController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/privateimagehubs",
			Method:      "GET",
			HandlerFunc: GetPrivateImageHubs,
		},
		models.Route{
			Path:        rootPath + "/privateimagehubs/{serverName}",
			Method:      "PUT",
			HandlerFunc: ChangePrivateImageHubSecret,
		},
	}

	return routes
}

// GetPrivateImageHubs Lists private image hubs
func GetPrivateImageHubs(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/privateimagehubs application getPrivateImageHubs
	// ---
	// summary: Lists the application private image hubs
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
	//           "$ref": "#/definitions/ImageHubSecret"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	privateImageHubHandler := Init(accounts)
	imageHubSecrets, err := privateImageHubHandler.GetPrivateImageHubs(appName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, imageHubSecrets)
}

// ChangePrivateImageHubSecret Modifies an application private image hub secret
func ChangePrivateImageHubSecret(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation PUT /applications/{appName}/privateimagehubs/{serverName} application updatePrivateImageHubsSecretValue
	// ---
	// summary: Update an application private image hub secret
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: serverName
	//   in: path
	//   description: server name to update
	//   type: string
	//   required: true
	// - name: imageHubSecret
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
	serverName := mux.Vars(r)["serverName"]

	var secretParameters environmentModels.SecretParameters
	if err := json.NewDecoder(r.Body).Decode(&secretParameters); err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	privateImageHubHandler := Init(accounts)
	err := privateImageHubHandler.UpdatePrivateImageHubValue(appName, serverName, secretParameters.SecretValue)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, "Success")
}
