package jobs

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/equinor/radix-api/api/deployments"
	"github.com/equinor/radix-api/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
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
		models.Route{
			Path:        rootPath + "/jobs/{jobName}/steps/{stepName}/output/scan",
			Method:      "GET",
			HandlerFunc: GetPipelineJobStepScanOutput,
		},
		models.Route{
			Path:        rootPath + "/jobs/{jobName}/pipelineruns",
			Method:      "GET",
			HandlerFunc: GetTektonPipelineRuns,
		},
	}

	return routes
}

// GetPipelineJobLogs Get logs of a pipeline-job for an application
func GetPipelineJobLogs(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/jobs/{jobName}/logs pipeline-job getApplicationJobLogs
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
	// - name: sinceTime
	//   in: query
	//   description: Get log only from sinceTime (example 2020-03-18T07:20:41+00:00)
	//   type: string
	//   format: date-time
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
	//           "$ref": "#/definitions/StepLog"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	jobName := mux.Vars(r)["jobName"]
	sinceTime := r.FormValue("sinceTime")

	var since time.Time
	var err error

	if !strings.EqualFold(strings.TrimSpace(sinceTime), "") {
		since, err = radixutils.ParseTimestamp(sinceTime)
		if err != nil {
			radixhttp.ErrorResponse(w, r, err)
			return
		}
	}

	handler := Init(accounts, deployments.Init(accounts))
	pipelines, err := handler.HandleGetApplicationJobLogs(appName, jobName, &since)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	sort.Slice(pipelines, func(i, j int) bool { return pipelines[i].Sort < pipelines[j].Sort })
	radixhttp.JSONResponse(w, r, pipelines)
}

// GetApplicationJobs gets pipeline-job summaries
func GetApplicationJobs(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/jobs pipeline-job getApplicationJobs
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

	handler := Init(accounts, deployments.Init(accounts))
	jobSummaries, err := handler.GetApplicationJobs(appName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, jobSummaries)
}

// GetApplicationJob gets specific pipeline-job details
func GetApplicationJob(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/jobs/{jobName} pipeline-job getApplicationJob
	// ---
	// summary: Gets the detail of a given pipeline-job for a given application
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

	handler := Init(accounts, deployments.Init(accounts))
	jobDetail, err := handler.GetApplicationJob(appName, jobName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, jobDetail)
}

// StopApplicationJob Stops job
func StopApplicationJob(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/jobs/{jobName}/stop pipeline-job stopApplicationJob
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
	//   "204":
	//     description: "Job stopped ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	jobName := mux.Vars(r)["jobName"]

	handler := Init(accounts, deployments.Init(accounts))
	err := handler.StopJob(appName, jobName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetPipelineJobStepScanOutput Get logs of a pipeline-job for an application
func GetPipelineJobStepScanOutput(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/jobs/{jobName}/steps/{stepName}/output/scan pipeline-job getPipelineJobStepScanOutput
	// ---
	// summary: Gets list of vulnerabilities found by the scan step
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
	// - name: stepName
	//   in: path
	//   description: Name of the step
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
	//           "$ref": "#/definitions/Vulnerability"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	jobName := mux.Vars(r)["jobName"]
	stepName := mux.Vars(r)["stepName"]

	handler := Init(accounts, deployments.Init(accounts))
	scanDetails, err := handler.GetPipelineJobStepScanOutput(appName, jobName, stepName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, scanDetails)
}

// GetTektonPipelineRuns Get the Tekton pipeline-runs overview
func GetTektonPipelineRuns(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/jobs/{jobName}/pipelinerun pipeline-job getTektonPipelines
	// ---
	// summary: Gets list of vulnerabilities found by the scan step
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
	//     description: "List of PipelineRun-s"
	//     schema:
	//        type: "array"
	//        items:
	//           "$ref": "#/definitions/PipelineRun"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	jobName := mux.Vars(r)["jobName"]

	handler := Init(accounts, deployments.Init(accounts))
	tektonPipelineRuns, err := handler.GetTektonPipelineRuns(appName, jobName)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	radixhttp.JSONResponse(w, r, tektonPipelineRuns)
}
