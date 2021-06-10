package buildstatus

import (
	"html"
	"net/http"
	"time"

	build_models "github.com/equinor/radix-api/api/buildstatus/models"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/models"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/gorilla/mux"
)

const rootPath = "/applications/{appName}/environments/{envName}"

type buildStatusController struct {
	*models.DefaultController
	build_models.PiplineBadgeBuilder
}

// NewBuildStatusController Constructor
func NewBuildStatusController(status build_models.PiplineBadgeBuilder) models.Controller {
	return &buildStatusController{PiplineBadgeBuilder: status}
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
	// - in: query
	//   name: pipeline
	//   description: Type of pipeline job to get status for.
	//   required: false
	//   default: build-deploy
	//   type: string
	//   enum: [build-deploy, deploy, promote]
	// responses:
	//   "200":
	//     description: "Successful operation"
	//   "500":
	//     description: "Internal Server Error"
	appName := mux.Vars(r)["appName"]
	env := mux.Vars(r)["envName"]
	pipeline := string(radixv1.BuildDeploy)
	if queryPipline := r.URL.Query().Get("pipeline"); len(queryPipline) > 0 {

		pipeline = html.EscapeString(queryPipline)
	}

	buildStatusHandler := Init(accounts, bsc.PiplineBadgeBuilder)
	buildStatus, err := buildStatusHandler.GetBuildStatusForApplication(appName, env, pipeline)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	disableClientCaching(w)
	utils.ByteArrayResponse(w, r, "image/svg+xml; charset=utf-8", buildStatus)
}

func disableClientCaching(w http.ResponseWriter) {
	header := w.Header()
	header.Add("Cache-Control", "no-cache")
	header.Add("Cache-Control", "no-store")
	// Set expires to a time in the past to disable Github caching when embedding in markdown files
	cacheUntil := time.Date(1984, 1, 1, 0, 0, 0, 0, time.UTC).Format(http.TimeFormat)
	header.Set("Expires", cacheUntil)
}
