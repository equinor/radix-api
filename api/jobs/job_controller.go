package jobs

import (
	"net/http"
	"sort"

	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/models"
	"github.com/gorilla/mux"
)

const rootPath = "/applications/{appName}"

type jobController struct {
	*models.DefaultController
}

// NewJobController Constructor
func NewJobController() models.Controller {
	return &jobController{}
}

// GetRoutes List the supported routes of this handler
func (jc *jobController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/jobs",
			Method:      "GET",
			HandlerFunc: GetApplicationJobs,
		},
		models.Route{
			Path:        rootPath + "/jobs/{jobName}/logs",
			Method:      "GET",
			HandlerFunc: GetPipelineJobLogs,
		},
		models.Route{
			Path:        rootPath + "/jobs/{jobName}",
			Method:      "GET",
			HandlerFunc: GetApplicationJob,
		},
		models.Route{
			Path:        rootPath + "/jobs/{jobName}/stop",
			Method:      "POST",
			HandlerFunc: StopApplicationJob,
		},
	}

	return routes
}

// GetPipelineJobLogs Get logs of a job for an application
func GetPipelineJobLogs(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/jobs/{jobName}/logs job getApplicationJobLogs
	// ---
	// summary: Gets a pipeline logs, by combining different steps (jobs) logs
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: Name of pipeline job
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
	//           "$ref": "#/definitions/StepLog"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	jobName := mux.Vars(r)["jobName"]

	handler := Init(accounts)
	pipelines, err := handler.HandleGetApplicationJobLogs(appName, jobName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	sort.Slice(pipelines, func(i, j int) bool { return pipelines[i].Sort < pipelines[j].Sort })
	utils.JSONResponse(w, r, pipelines)
}

// GetApplicationJobs gets job summaries
func GetApplicationJobs(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/jobs job getApplicationJobs
	// ---
	// summary: Gets the summary of jobs for a given application
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
	//           "$ref": "#/definitions/JobSummary"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	handler := Init(accounts)
	jobSummaries, err := handler.GetApplicationJobs(appName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, jobSummaries)
}

// GetApplicationJob gets specific job details
func GetApplicationJob(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/jobs/{jobName} job getApplicationJob
	// ---
	// summary: Gets the detail of a given job for a given application
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: name of job
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
	//     description: "Successful get job"
	//     schema:
	//        "$ref": "#/definitions/Job"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	jobName := mux.Vars(r)["jobName"]

	handler := Init(accounts)
	jobDetail, err := handler.GetApplicationJob(appName, jobName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, jobDetail)
}

// StopApplicationJob Stops job
func StopApplicationJob(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/jobs/{jobName}/stop job stopApplicationJob
	// ---
	// summary: Stops job
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of application
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: name of job
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
	//     description: "Job stopped ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	jobName := mux.Vars(r)["jobName"]

	handler := Init(accounts)
	err := handler.StopJob(appName, jobName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
