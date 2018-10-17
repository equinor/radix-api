package applications

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/statoil/radix-api/api/utils"
	"github.com/statoil/radix-api/models"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"

	"github.com/graphql-go/graphql"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/statoil/radix-operator/pkg/client/informers/externalversions"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const repoURL = "https://github.com/"
const sshURL = "git@github.com:"
const rootPath = ""

var repoPattern = regexp.MustCompile(fmt.Sprintf("%s(.*?)", repoURL))

// GetRoutes List the supported routes of this handler
func GetRoutes() models.Routes {
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
			HandlerFunc: ShowApplications,
			WatcherFunc: GetApplicationStream,
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
			Path:        rootPath + "/applications/{appName}/pipeline/{branch}",
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

// GetSubscriptions Lists subscriptions this handler offers
func GetSubscriptions() models.Subscriptions {
	subscriptions := models.Subscriptions{
		models.Subscription{
			SubcribeCommand:    "application_subscribe",
			UnsubscribeCommand: "application_unsubscribe",
			DataType:           "application",
			HandlerFunc:        GetApplicationStream,
		},
	}

	return subscriptions
}

// GetApplicationStream Gets stream of applications
func GetApplicationStream(client kubernetes.Interface, radixclient radixclient.Interface, arg string, data chan []byte, unsubscribe chan struct{}) {
	if arg == "" {
		arg = `{
			name
			repository
			description
		}`
	}

	factory := informers.NewSharedInformerFactory(radixclient, 0)
	rrInformer := factory.Radix().V1().RadixApplications().Informer()
	raInformer := factory.Radix().V1().RadixApplications().Informer()
	rdInformer := factory.Radix().V1().RadixDeployments().Informer()

	//	now := time.Now()

	rrInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			rr := obj.(*v1.RadixRegistration)
			log.Infof("Added RR to store for %s", rr.Name)

			//if rr.GetCreationTimestamp().After(now) {
			body, _ := getSubscriptionData(radixclient, arg, rr.Name, getRepositoryURLFromCloneURL(rr.Spec.CloneURL), "New RR Added to Store")
			data <- body
			//}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			rr := new.(*v1.RadixRegistration)
			//if rr.GetCreationTimestamp().After(now) {
			body, _ := getSubscriptionData(radixclient, arg, rr.Name, getRepositoryURLFromCloneURL(rr.Spec.CloneURL), "RR updated")
			data <- body
			//}
		},
		DeleteFunc: func(obj interface{}) {
			rr := obj.(*v1.RadixRegistration)
			//if rr.GetDeletionTimestamp().After(now) {
			body, _ := getSubscriptionData(radixclient, arg, rr.Name, getRepositoryURLFromCloneURL(rr.Spec.CloneURL), "RR Deleted from Store")
			data <- body
			//}
		},
	})

	raInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ra := obj.(*v1.RadixApplication)
			//if ra.GetCreationTimestamp().After(now) {
			body, _ := getSubscriptionData(radixclient, arg, ra.Name, "", "RA Added to Store")
			data <- body
			//}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			ra := new.(*v1.RadixApplication)
			//if ra.GetCreationTimestamp().After(now) {
			body, _ := getSubscriptionData(radixclient, arg, ra.Name, "", "RA updated")
			data <- body
			//}
		},
		DeleteFunc: func(obj interface{}) {
			ra := obj.(*v1.RadixApplication)
			//if ra.GetDeletionTimestamp().After(now) {
			body, _ := getSubscriptionData(radixclient, arg, ra.Name, "", "RA deleted")
			data <- body
			//}
		},
	})

	rdInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			rd := obj.(*v1.RadixDeployment)
			//if rd.GetCreationTimestamp().After(now) {
			body, _ := getSubscriptionData(radixclient, arg, rd.Name, "", "New RD Added to Store")
			data <- body
			//}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			rd := new.(*v1.RadixDeployment)
			//if rd.GetCreationTimestamp().After(now) {
			body, _ := getSubscriptionData(radixclient, arg, rd.Name, "", "RD updated")
			data <- body
			//}
		},
		DeleteFunc: func(obj interface{}) {
			rd := obj.(*v1.RadixDeployment)
			//if rd.GetDeletionTimestamp().After(now) {
			body, _ := getSubscriptionData(radixclient, arg, rd.Name, "", "RD deleted")
			data <- body
			//}
		},
	})

	stop := make(chan struct{})
	go func() {
		<-unsubscribe
		close(stop)
	}()

	go rrInformer.Run(stop)
	go raInformer.Run(stop)
	go rdInformer.Run(stop)
}

// ShowApplications Lists applications
func ShowApplications(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications platform showApplications
	// ---
	// summary: Lists the applications
	// parameters:
	// - name: sshRepo
	//   in: query
	//   description: ssh repo to identify Radix application if exists
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     description: "Successful operation"
	//     schema:
	//        type: "array"
	//        items:
	//           "$ref": "#/definitions/ApplicationRegistration"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	sshRepo := r.FormValue("sshRepo")

	appRegistrations, err := HandleGetApplications(radixclient, sshRepo)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, appRegistrations)
}

// GetApplication Gets application by application name
func GetApplication(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName} application getApplication
	// ---
	// summary: Gets the application application by name
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/definitions/ApplicationRegistration"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	appRegistration, err := HandleGetApplication(radixclient, appName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, &appRegistration)
}

// IsDeployKeyValidHandler validates deploy key for radix application found for application name
func IsDeployKeyValidHandler(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/deploykey-valid application isDeployKeyValid
	// ---
	// summary: Checks if the deploy key is correctly setup for application by cloning the repository
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     description: "Deploy key is valid"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	isDeployKeyValid, err := IsDeployKeyValid(client, radixclient, appName)

	if isDeployKeyValid {
		utils.JSONResponse(w, r, &isDeployKeyValid)
		return
	}

	utils.ErrorResponse(w, r, err)
}

// RegisterApplication Creates new application registation
func RegisterApplication(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
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
	// responses:
	//   "200":
	//     "$ref": "#/definitions/ApplicationRegistration"
	//   "400":
	//     description: "Invalid application registration"
	//   "401":
	//     description: "Unauthorized"
	//   "409":
	//     description: "Conflict"
	var application ApplicationRegistration
	if err := json.NewDecoder(r.Body).Decode(&application); err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	appRegistration, err := HandleRegisterApplication(radixclient, application)
	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, &appRegistration)
}

// ChangeRegistrationDetails Updates application registration
func ChangeRegistrationDetails(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
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

	var application ApplicationRegistration
	if err := json.NewDecoder(r.Body).Decode(&application); err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	appRegistration, err := HandleChangeRegistrationDetails(radixclient, appName, application)
	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, &appRegistration)
}

// DeleteApplication Deletes application
func DeleteApplication(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation DELETE /applications/{appName} application deleteApplication
	// ---
	// summary: Delete application
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of application
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     description: "Application deleted ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	err := HandleDeleteApplication(radixclient, appName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, "ok")
}

// TriggerPipeline creates a pipeline job for the application
func TriggerPipeline(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/pipeline/{branchName} application triggerPipeline
	// ---
	// summary: Create an application pipeline for a given application and branch
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: branchName
	//   in: path
	//   description: Name of branch
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     description: "Pipeline job started ok"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	branch := mux.Vars(r)["branch"]
	jobSpec, err := HandleTriggerPipeline(client, radixclient, appName, branch)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, fmt.Sprintf("Pipeline %s for %s on branch %s started", jobSpec.Name, appName, jobSpec.Branch))
}

func getSubscriptionData(radixclient radixclient.Interface, arg, name, repo, description string) ([]byte, error) {
	radixApplication := &Application{
		Name:        name,
		Repository:  repo,
		Description: description,
	}

	queryData, err := getDataFromQuery(arg, radixApplication)
	if err != nil {
		return nil, err
	}

	body, _ := json.Marshal(queryData)
	return body, nil
}

func getDataFromQuery(arg string, radixApplication *Application) (*graphql.Result, error) {
	// Schema
	fields := graphql.Fields{
		"name": &graphql.Field{
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return radixApplication.Name, nil
			},
		},
		"repository": &graphql.Field{
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return radixApplication.Repository, nil
			},
		},
		"description": &graphql.Field{
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return radixApplication.Description, nil
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
