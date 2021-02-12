package buildstatus

import (
	"net/http"

	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/models"
	"github.com/gorilla/mux"
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
			Path:        rootPath + "/buildstatus/{env}",
			Method:      "GET",
			HandlerFunc: GetBuildStatus,
		},
	}

	return routes
}

// GetBuildStatus reveals build status for selected environment
func GetBuildStatus(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/buildstatus/{env} application getBuildStatus
	// ---
	// summary: Show the application buildStatus
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: env
	//   in: path
	//   description: name of the environment
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
	env := mux.Vars(r)["env"]

	buildStatusHandler := Init(accounts)
	buildStatus, err := buildStatusHandler.GetBuildStatusForApplication(appName, env)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.ByteArrayResponse(w, r, "text/html; charset=utf-8", *buildStatus)
}
