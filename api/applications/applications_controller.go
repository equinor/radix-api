package applications

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/models"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/graphql-go/graphql"

	"k8s.io/client-go/tools/cache"
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

// GetSubscriptions Lists subscriptions this controller offers
func (ac *applicationController) GetSubscriptions() models.Subscriptions {
	subscriptions := models.Subscriptions{
		models.Subscription{
			Resource:    rootPath + "/applications",
			DataType:    "ApplicationSummary",
			HandlerFunc: GetApplicationStream,
		},
	}

	return subscriptions
}

// GetApplicationStream Gets stream of applications
func GetApplicationStream(clients models.Clients, resource string, resourceIdentifiers []string, data chan []byte, unsubscribe chan struct{}) {
	arg := `{
			name
			repository
			description
		}`

	factory := informers.NewSharedInformerFactory(clients.OutClusterRadixClient, 0)
	rrInformer := factory.Radix().V1().RadixRegistrations().Informer()
	raInformer := factory.Radix().V1().RadixApplications().Informer()
	rdInformer := factory.Radix().V1().RadixDeployments().Informer()

	handleRR := func(obj interface{}, event string) {
		rr := obj.(*v1.RadixRegistration)
		body, _ := getSubscriptionData(clients.OutClusterRadixClient, arg, rr.Name, crdUtils.GetGithubRepositoryURLFromCloneURL(rr.Spec.CloneURL), event)
		data <- body
	}

	handleRA := func(obj interface{}, event string) {
		ra := obj.(*v1.RadixApplication)
		body, _ := getSubscriptionData(clients.OutClusterRadixClient, arg, ra.Name, "", event)
		data <- body
	}

	handleRD := func(obj interface{}, event string) {
		rd := obj.(*v1.RadixDeployment)
		body, _ := getSubscriptionData(clients.OutClusterRadixClient, arg, rd.Name, "", event)
		data <- body
	}

	defaultResourceEventHandler := func(handler func(interface{}, string)) cache.ResourceEventHandler {
		return cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { handler(obj, fmt.Sprintf("%s added", reflect.TypeOf(obj))) },
			UpdateFunc: func(old interface{}, new interface{}) { handler(new, fmt.Sprintf("%s updated", reflect.TypeOf(new))) },
			DeleteFunc: func(obj interface{}) { handler(obj, fmt.Sprintf("%s deleted", reflect.TypeOf(obj))) },
		}
	}

	rrInformer.AddEventHandler(defaultResourceEventHandler(handleRR))
	raInformer.AddEventHandler(defaultResourceEventHandler(handleRA))
	rdInformer.AddEventHandler(defaultResourceEventHandler(handleRD))

	utils.StreamInformers(unsubscribe, rrInformer, raInformer, rdInformer)
}

// ShowApplications Lists applications
func (ac *applicationController) ShowApplications(clients models.Clients, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications platform showApplications
	// ---
	// summary: Lists the applications
	// parameters:
	// - name: sshRepo
	//   in: query
	//   description: ssh repo to identify Radix application if exists
	//   type: string
	//   required: false
	// - name: Impersonate-User
	//   in: header
	//   description: Allow impersonation of test users (Required if Impersonate-Group is set)
	//   type: string
	//   required: false
	// - name: Impersonate-Group
	//   in: header
	//   description: Allow impersonation of test group (Required if Impersonate-User is set)
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

	handler := Init(clients.OutClusterClient, clients.OutClusterRadixClient, clients.InClusterRadixClient, clients.InClusterClient, true)
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
	//   description: Allow impersonation of test users (Required if Impersonate-Group is set)
	//   type: string
	//   required: false
	// - name: Impersonate-Group
	//   in: header
	//   description: Allow impersonation of test group (Required if Impersonate-User is set)
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     "$ref": "#/definitions/Application"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]

	handler := Init(clients.OutClusterClient, clients.OutClusterRadixClient, clients.InClusterRadixClient, clients.InClusterClient, false)
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
	//   description: Allow impersonation of test users (Required if Impersonate-Group is set)
	//   type: string
	//   required: false
	// - name: Impersonate-Group
	//   in: header
	//   description: Allow impersonation of test group (Required if Impersonate-User is set)
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
	//   description: Allow impersonation of test users (Required if Impersonate-Group is set)
	//   type: string
	//   required: false
	// - name: Impersonate-Group
	//   in: header
	//   description: Allow impersonation of test group (Required if Impersonate-User is set)
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     "$ref": "#/definitions/ApplicationRegistration"
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
	handler := Init(clients.OutClusterClient, clients.OutClusterRadixClient, clients.InClusterRadixClient, clients.InClusterClient, false)
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
	//   description: Allow impersonation of test users (Required if Impersonate-Group is set)
	//   type: string
	//   required: false
	// - name: Impersonate-Group
	//   in: header
	//   description: Allow impersonation of test group (Required if Impersonate-User is set)
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     "$ref": "#/definitions/ApplicationRegistration"
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
	handler := Init(clients.OutClusterClient, clients.OutClusterRadixClient, clients.InClusterRadixClient, clients.InClusterClient, false)
	appRegistration, err := handler.ChangeRegistrationDetails(appName, application)
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
	//   description: Allow impersonation of test users (Required if Impersonate-Group is set)
	//   type: string
	//   required: false
	// - name: Impersonate-Group
	//   in: header
	//   description: Allow impersonation of test group (Required if Impersonate-User is set)
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

	handler := Init(clients.OutClusterClient, clients.OutClusterRadixClient, clients.InClusterRadixClient, clients.InClusterClient, false)
	err := handler.DeleteApplication(appName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
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
	//   - build-deploy
	//   required: true
	// - name: pipelineParameters
	//   in: body
	//   description: Branch to build
	//   required: true
	//   schema:
	//       "$ref": "#/definitions/PipelineParameters"
	// - name: Impersonate-User
	//   in: header
	//   description: Allow impersonation of test users (Required if Impersonate-Group is set)
	//   type: string
	//   required: false
	// - name: Impersonate-Group
	//   in: header
	//   description: Allow impersonation of test group (Required if Impersonate-User is set)
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     "$ref": "#/definitions/JobSummary"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	pipelineName := mux.Vars(r)["pipelineName"]

	var pipelineParameters applicationModels.PipelineParameters
	if err := json.NewDecoder(r.Body).Decode(&pipelineParameters); err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	handler := Init(clients.OutClusterClient, clients.OutClusterRadixClient, clients.InClusterRadixClient, clients.InClusterClient, false)
	jobSummary, err := handler.TriggerPipeline(appName, pipelineName, pipelineParameters)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, &jobSummary)
}

func getSubscriptionData(radixclient radixclient.Interface, arg, name, repo, description string) ([]byte, error) {
	log.Infof("%s", description)
	radixApplication := &applicationModels.ApplicationSummary{
		Name: name,
	}

	queryData, err := getDataFromQuery(arg, radixApplication)
	if err != nil {
		return nil, err
	}

	body, _ := json.Marshal(queryData)
	return body, nil
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
