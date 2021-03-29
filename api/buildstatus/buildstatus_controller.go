package buildstatus

import (
	"net/http"

	build_models "github.com/equinor/radix-api/api/buildstatus/models"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/models"
	"github.com/gorilla/mux"
)

const rootPath = "/applications/{appName}/environments/{envName}"

type buildStatusController struct {
	*models.DefaultController
	build_models.Status
}

// NewBuildStatusController Constructor
func NewBuildStatusController(status build_models.Status) models.Controller {
	return &buildStatusController{Status: status}
}

// GetRoutes List the supported routes of this handler
func (bsc *buildStatusController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:                      rootPath + "/buildstatus",
			Method:                    "GET",
			HandlerFunc:               bsc.GetBuildStatus,
			AllowUnauthenticatedUsers: true,
		},
	}

	return routes
}

// GetBuildStatus reveals build status for selected environment
func (bsc *buildStatusController) GetBuildStatus(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/buildstatus buildstatus getBuildStatus
	// ---
	// summary: Show the application buildStatus
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: envName
	//   in: path
	//   description: name of the environment
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     description: "Successful operation"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	env := mux.Vars(r)["envName"]

	buildStatusHandler := Init(accounts, bsc.Status)
	buildStatus, err := buildStatusHandler.GetBuildStatusForApplication(appName, env)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.ByteArrayResponse(w, r, "image/svg+xml; charset=utf-8", *buildStatus)
}
