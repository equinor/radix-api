package applications

import (
	"encoding/json"
	"fmt"
	"net/http"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/models"
	"github.com/gorilla/mux"

	"github.com/graphql-go/graphql"
)

const rootPath = ""

type applicationController struct {
	*models.DefaultController
	hasAccessToRR
}

// NewApplicationController Constructor
func NewApplicationController(hasAccessTo hasAccessToRR) models.Controller {
	if hasAccessTo == nil {
		hasAccessTo = hasAccess
	}

	return &applicationController{
		hasAccessToRR: hasAccessTo,
	}
}

// GetRoutes List the supported routes of this controller
func (ac *applicationController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/applications",
			Method:      "POST",
			HandlerFunc: RegisterApplication,
		},
		models.Route{
			Path:        rootPath + "/applications/{appName}",
			Method:      "PUT",
			HandlerFunc: ChangeRegistrationDetails,
		},
		models.Route{
			Path:        rootPath + "/applications/{appName}",
			Method:      "PATCH",
			HandlerFunc: ModifyRegistrationDetails,
		},
		models.Route{
			Path:        rootPath + "/applications",
			Method:      "GET",
			HandlerFunc: ac.ShowApplications,
		},
		models.Route{
			Path:        rootPath + "/applications/{appName}",
			Method:      "GET",
			HandlerFunc: GetApplication,
		},
		models.Route{
			Path:        rootPath + "/applications/{appName}",
			Method:      "DELETE",
			HandlerFunc: DeleteApplication,
		},
		models.Route{
			Path:        rootPath + "/applications/{appName}/pipelines",
			Method:      "GET",
			HandlerFunc: ListPipelines,
		},
		models.Route{
			Path:        rootPath + "/applications/{appName}/pipelines/{pipelineName}",
			Method:      "POST",
			HandlerFunc: TriggerPipeline,
		},
		models.Route{
			Path:        rootPath + "/applications/{appName}/deploykey-valid",
			Method:      "GET",
			HandlerFunc: IsDeployKeyValidHandler,
		},
	}

	return routes
}

// ShowApplications Lists applications
func (ac *applicationController) ShowApplications(clients models.Clients, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications platform showApplications
	//
	// ---
	// summary: Lists the applications. NOTE - doesn't get applicationSummary.latestJob.Environments
	// parameters:
	// - name: sshRepo
	//   in: query
	//   description: ssh repo to identify Radix application if exists
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
	//           "$ref": "#/definitions/ApplicationSummary"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	sshRepo := r.FormValue("sshRepo")

	handler := Init(clients.OutClusterClient, clients.OutClusterRadixClient, clients.InClusterClient, clients.InClusterRadixClient)
	appRegistrations, err := handler.GetApplications(sshRepo, ac.hasAccessToRR)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, appRegistrations)
}

// GetApplication Gets application by application name
func GetApplication(clients models.Clients, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName} application getApplication
	// ---
	// summary: Gets the application application by name
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
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
	//     description: Successful get application
	//     schema:
	//       "$ref": "#/definitions/Application"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	handler := Init(clients.OutClusterClient, clients.OutClusterRadixClient, clients.InClusterClient, clients.InClusterRadixClient)
	application, err := handler.GetApplication(appName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, &application)
}

// IsDeployKeyValidHandler validates deploy key for radix application found for application name
func IsDeployKeyValidHandler(clients models.Clients, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/deploykey-valid application isDeployKeyValid
	// ---
	// summary: Checks if the deploy key is correctly setup for application by cloning the repository
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
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
	//     description: "Deploy key is valid"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	isDeployKeyValid, err := IsDeployKeyValid(clients.OutClusterClient, clients.OutClusterRadixClient, appName)

	if isDeployKeyValid {
		utils.JSONResponse(w, r, &isDeployKeyValid)
		return
	}

	utils.ErrorResponse(w, r, err)
}

// RegisterApplication Creates new application registation
func RegisterApplication(clients models.Clients, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications platform registerApplication
	// ---
	// summary: Create an application registration
	// parameters:
	// - name: applicationRegistration
	//   in: body
	//   description: Application to register
	//   required: true
	//   schema:
	//       "$ref": "#/definitions/ApplicationRegistration"
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
	//     description: Successful application registration
	//     schema:
	//       "$ref": "#/definitions/ApplicationRegistration"
	//   "400":
	//     description: "Invalid application registration"
	//   "401":
	//     description: "Unauthorized"
	//   "409":
	//     description: "Conflict"
	var application applicationModels.ApplicationRegistration
	if err := json.NewDecoder(r.Body).Decode(&application); err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	// Need in cluster Radix client in order to validate registration using sufficient priviledges
	handler := Init(clients.OutClusterClient, clients.OutClusterRadixClient, clients.InClusterClient, clients.InClusterRadixClient)
	appRegistration, err := handler.RegisterApplication(application)
	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, &appRegistration)
}

// ChangeRegistrationDetails Updates application registration
func ChangeRegistrationDetails(clients models.Clients, w http.ResponseWriter, r *http.Request) {
	// swagger:operation PUT /applications/{appName} application changeRegistrationDetails
	// ---
	// summary: Update application registration
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: applicationRegistration
	//   in: body
	//   description: Application to register
	//   required: true
	//   schema:
	//       "$ref": "#/definitions/ApplicationRegistration"
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
	//     description: Successful change registration details
	//     schema:
	//       "$ref": "#/definitions/ApplicationRegistration"
	//   "400":
	//     description: "Invalid application"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	//   "409":
	//     description: "Conflict"
	appName := mux.Vars(r)["appName"]

	var application applicationModels.ApplicationRegistration
	if err := json.NewDecoder(r.Body).Decode(&application); err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	// Need in cluster Radix client in order to validate registration using sufficient priviledges
	handler := Init(clients.OutClusterClient, clients.OutClusterRadixClient, clients.InClusterClient, clients.InClusterRadixClient)
	appRegistration, err := handler.ChangeRegistrationDetails(appName, application)
	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, &appRegistration)
}

// ModifyRegistrationDetails Updates specific field(s) of an application registration
func ModifyRegistrationDetails(clients models.Clients, w http.ResponseWriter, r *http.Request) {
	// swagger:operation PATCH /applications/{appName} application modifyRegistrationDetails
	// ---
	// summary: Updates specific field(s) of an application registration
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: patchRequest
	//   in: body
	//   description: Application to patch
	//   required: true
	//   schema:
	//       "$ref": "#/definitions/ApplicationPatchRequest"
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
	//     description: Successful at modifying registration details
	//     schema:
	//       "$ref": "#/definitions/ApplicationRegistration"
	//   "400":
	//     description: "Invalid application"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	//   "409":
	//     description: "Conflict"
	appName := mux.Vars(r)["appName"]

	var application applicationModels.ApplicationPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&application); err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	// Need in cluster Radix client in order to validate registration using sufficient priviledges
	handler := Init(clients.OutClusterClient, clients.OutClusterRadixClient, clients.InClusterClient, clients.InClusterRadixClient)
	appRegistration, err := handler.ModifyRegistrationDetails(appName, application)
	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, &appRegistration)
}

// DeleteApplication Deletes application
func DeleteApplication(clients models.Clients, w http.ResponseWriter, r *http.Request) {
	// swagger:operation DELETE /applications/{appName} application deleteApplication
	// ---
	// summary: Delete application
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of application
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
	//     description: "Application deleted ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	handler := Init(clients.OutClusterClient, clients.OutClusterRadixClient, clients.InClusterClient, clients.InClusterRadixClient)
	err := handler.DeleteApplication(appName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// ListPipelines Lists supported pipelines
func ListPipelines(clients models.Clients, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/pipelines application listPipelines
	// ---
	// summary: Lists the supported pipelines
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     description: "Successful operation"
	//     schema:
	//        type: array
	//        items:
	//           type: string

	// It was suggested to keep this under /applications/{appName} endpoint, but for now this will be the same for all applications
	handler := Init(clients.OutClusterClient, clients.OutClusterRadixClient, clients.InClusterClient, clients.InClusterRadixClient)
	supportedPipelines := handler.GetSupportedPipelines()
	utils.JSONResponse(w, r, supportedPipelines)
}

// TriggerPipeline creates a pipeline job for the application
func TriggerPipeline(clients models.Clients, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/pipelines/{pipelineName} application triggerPipeline
	// ---
	// summary: Run a pipeline for a given application and branch
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: pipelineName
	//   in: path
	//   description: Name of pipeline
	//   type: string
	//   enum:
	//   - build
	//   - build-deploy
	//   - promote
	//   required: true
	// - name: PipelineParameters
	//   description: Pipeline parameters
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/PipelineParameters"
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
	//     description: Successful trigger pipeline
	//     schema:
	//       "$ref": "#/definitions/JobSummary"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	pipelineName := mux.Vars(r)["pipelineName"]

	handler := Init(clients.OutClusterClient, clients.OutClusterRadixClient, clients.InClusterClient, clients.InClusterRadixClient)
	jobSummary, err := handler.TriggerPipeline(appName, pipelineName, r)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, &jobSummary)
}

func getDataFromQuery(arg string, radixApplication *applicationModels.ApplicationSummary) (*graphql.Result, error) {
	// Schema
	fields := graphql.Fields{
		"name": &graphql.Field{
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return radixApplication.Name, nil
			},
		},
	}
	rootQuery := graphql.ObjectConfig{Name: "RootQuery", Fields: fields}
	schemaConfig := graphql.SchemaConfig{Query: graphql.NewObject(rootQuery)}
	schema, err := graphql.NewSchema(schemaConfig)
	if err != nil {
		return nil, err
	}

	params := graphql.Params{Schema: schema, RequestString: arg}
	r := graphql.Do(params)
	if len(r.Errors) > 0 {
		return nil, fmt.Errorf("Failed to execute graphql operation, errors: %+v", r.Errors)
	}

	return r, nil
}
