package buildstatus

import (
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/models"
	"github.com/gorilla/mux"
	"net/http"
)

const rootPath = "/applications/{appName}"

type buildStatusController struct {
	*models.DefaultController
}

// NewBuildStatusController Constructor
func NewBuildStatusController() models.Controller {
	return &buildStatusController{}
}

// GetRoutes List the supported routes of this handler
func (dc *buildStatusController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/buildStatus",
			Method:      "GET",
			HandlerFunc: GetBuildStatus,
		},
	}

	return routes
}

// GetBuildStatus Lists buildStatus
func GetBuildStatus(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /buildstatus/{appName} application getBuildStatus
	// ---
	// summary: Lists the application buildStatus
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
	//           "$ref": "#/definitions/buildStatus"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	buildStatusHandler := Init(accounts)
	buildStatus, err := buildStatusHandler.GetBuildStatusForApplication(appName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.ByteArrayResponse(w, r, "text/html; charset=utf-8", *buildStatus)
}

func Init(accounts models.Accounts) *BuildStatusHandler {
	return &BuildStatusHandler{}
}
