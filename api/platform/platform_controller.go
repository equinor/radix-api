package platform

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/Sirupsen/logrus"

	"github.com/gorilla/mux"
	"github.com/statoil/radix-api-go/api/utils"
	"github.com/statoil/radix-api-go/models"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"

	"github.com/graphql-go/graphql"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/statoil/radix-operator/pkg/client/informers/externalversions"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

var repoPattern = regexp.MustCompile("https://github.com/(.*?)")

const sshURL = "git@github.com:"
const rootPath = "/platform"

// GetRoutes List the supported routes of this handler
func GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/registrations",
			Method:      "POST",
			HandlerFunc: CreateRegistation,
		},
		models.Route{
			Path:        rootPath + "/registrations",
			Method:      "GET",
			HandlerFunc: GetRegistations,
			WatcherFunc: GetRegistrationStream,
		},
		models.Route{
			Path:        rootPath + "/registrations/{appName}",
			Method:      "GET",
			HandlerFunc: GetRegistation,
		},
		models.Route{
			Path:        rootPath + "/registrations/{appName}",
			Method:      "DELETE",
			HandlerFunc: DeleteRegistation,
		},
		models.Route{
			Path:        rootPath + "/registrations/{appName}/pipeline/{branch}",
			Method:      "POST",
			HandlerFunc: CreateApplicationPipelineJob,
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
			HandlerFunc:        GetRegistrationStream,
		},
	}

	return subscriptions
}

// GetRegistrationStream Gets stream of registrations
func GetRegistrationStream(client kubernetes.Interface, radixclient radixclient.Interface, arg string, data chan []byte, unsubscribe chan struct{}) {
	if arg == "" {
		arg = `{
			name
			repository
			description
		}`
	}

	factory := informers.NewSharedInformerFactory(radixclient, 0)
	rrInformer := factory.Radix().V1().RadixRegistrations().Informer()
	raInformer := factory.Radix().V1().RadixApplications().Informer()
	rdInformer := factory.Radix().V1().RadixDeployments().Informer()

	//	now := time.Now()

	rrInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			rr := obj.(*v1.RadixRegistration)
			logrus.Infof("Added RR to store for %s", rr.Name)

			//if rr.GetCreationTimestamp().After(now) {
			body, _ := getSubscriptionData(radixclient, arg, rr.Name, rr.Spec.Repository, "New RR Added to Store")
			data <- body
			//}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			rr := new.(*v1.RadixRegistration)
			//if rr.GetCreationTimestamp().After(now) {
			body, _ := getSubscriptionData(radixclient, arg, rr.Name, rr.Spec.Repository, "RR updated")
			data <- body
			//}
		},
		DeleteFunc: func(obj interface{}) {
			rr := obj.(*v1.RadixRegistration)
			//if rr.GetDeletionTimestamp().After(now) {
			body, _ := getSubscriptionData(radixclient, arg, rr.Name, rr.Spec.Repository, "RR Deleted from Store")
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

// GetRegistations Lists registrations
func GetRegistations(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /platform/registrations registrations getRegistations
	// ---
	// summary: Lists the application registrations
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
	appRegistrations, err := HandleGetRegistations(radixclient)

	if err != nil {
		utils.WriteError(w, r, err)
		return
	}

	utils.JSONResponse(w, r, appRegistrations)
}

// GetRegistation Gets registration by application name
func GetRegistation(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /platform/registrations/{appName} registrations getRegistation
	// ---
	// summary: Gets the application registration by name
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
	appRegistration, err := HandleGetRegistation(radixclient, appName)

	if err != nil {
		utils.WriteError(w, r, err)
		return
	}

	utils.JSONResponse(w, r, &appRegistration)
}

// CreateRegistation Creates new registration for application
func CreateRegistation(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /platform/registrations registrations createRegistation
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
	//     description: "Invalid registration"
	//   "401":
	//     description: "Unauthorized"
	//   "409":
	//     description: "Conflict"
	var registration ApplicationRegistration
	if err := json.NewDecoder(r.Body).Decode(&registration); err != nil {
		utils.WriteError(w, r, err)
		return
	}

	appRegistration, err := HandleCreateRegistation(radixclient, registration)
	if err != nil {
		utils.WriteError(w, r, err)
		return
	}

	utils.JSONResponse(w, r, &appRegistration)
}

// DeleteRegistation Deletes registration for application
func DeleteRegistation(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation DELETE /platform/registrations/{appName} registrations deleteRegistation
	// ---
	// summary: Delete registration
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of application
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     description: "Registration deleted ok"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	err := HandleDeleteRegistation(radixclient, appName)

	if err != nil {
		utils.WriteError(w, r, err)
		return
	}

	utils.JSONResponse(w, r, "ok")
}

// CreateApplicationPipelineJob creates a pipeline job for the application
func CreateApplicationPipelineJob(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /platform/registrations/{appName}/pipeline/{branchName} registrations createApplicationPipelineJob
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
	err := HandleCreateApplicationPipelineJob(client, radixclient, appName, branch)

	if err != nil {
		utils.WriteError(w, r, err)
		return
	}

	utils.JSONResponse(w, r, "ok")
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
