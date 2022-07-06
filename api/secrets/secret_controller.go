package secrets

import (
	"encoding/json"
	"net/http"

	secretModels "github.com/equinor/radix-api/api/secrets/models"
	"github.com/equinor/radix-api/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	"github.com/gorilla/mux"
)

const rootPath = "/applications/{appName}"

type secretController struct {
	*models.DefaultController
}

// NewSecretController Constructor
func NewSecretController() models.Controller {
	return &secretController{}
}

// GetRoutes List the supported routes of this handler
func (ec *secretController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/secrets/{secretName}",
			Method:      "PUT",
			HandlerFunc: ChangeComponentSecret,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/secrets/azure/keyvault/{azureKeyVaultName}",
			Method:      "GET",
			HandlerFunc: GetAzureKeyVaultSecretStatus,
		},
		//TODO reimplement change-secrets individually for each secret type
		//models.Route{
		//    Path:        rootPath + "/environments/{envName}/components/{componentName}/secrets/azure/keyvault/clientid/{azureKeyVaultName}",
		//    Method:      "PUT",
		//    HandlerFunc: ChangeSecretAzureKeyVaultClientId,
		//},
	}
	return routes
}

// ChangeComponentSecret Modifies an application environment component secret
func ChangeComponentSecret(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation PUT /applications/{appName}/environments/{envName}/components/{componentName}/secrets/{secretName} environment changeComponentSecret
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
	//   description: secret of Radix application
	//   type: string
	//   required: true
	// - name: componentName
	//   in: path
	//   description: secret component of Radix application
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
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"
	//   "409":
	//     description: "Conflict"
	//   "500":
	//     description: "Internal server error"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	componentName := mux.Vars(r)["componentName"]
	secretName := mux.Vars(r)["secretName"]

	var secretParameters secretModels.SecretParameters
	if err := json.NewDecoder(r.Body).Decode(&secretParameters); err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	handler := Init(WithAccounts(accounts))

	if err := handler.ChangeComponentSecret(appName, envName, componentName, secretName, secretParameters); err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, "Success")
}

// GetAzureKeyVaultSecretStatus Gets an application environment component Azure Key vault secret status
func GetAzureKeyVaultSecretStatus(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/components/{componentName}/secrets/azure/keyvault/{azureKeyVaultName} environment getAzureKeyVaultSecretStatus
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
	//   description: secret of Radix application
	//   type: string
	//   required: true
	// - name: componentName
	//   in: path
	//   description: secret component of Radix application
	//   type: string
	//   required: true
	// - name: azureKeyVaultName
	//   in: path
	//   description: Azure Key vault name
	//   type: string
	//   required: true
	// - name: secretName
	//   in: query
	//   description: secret (or key, cert) name in Azure Key vault
	//   type: string
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
	//           "$ref": "#/definitions/AzureKeyVaultSecretStatus"
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
	//   "500":
	//     description: "Internal server error"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	componentName := mux.Vars(r)["componentName"]
	azureKeyVaultName := mux.Vars(r)["azureKeyVaultName"]
	secretName := r.FormValue("secretName")

	handler := Init(WithAccounts(accounts))

	secretStatuses, err := handler.GetAzureKeyVaultSecretStatus(appName, envName, componentName, azureKeyVaultName, secretName)
	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, secretStatuses)
}
