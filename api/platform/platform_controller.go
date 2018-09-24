package platform

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"time"

	"github.com/gorilla/mux"
	"github.com/statoil/radix-api-go/api/utils"
	"github.com/statoil/radix-api-go/models"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/graphql-go/graphql"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

var repoPattern = regexp.MustCompile("https://github.com/(.*?)")

const sshURL = "git@github.com:"
const rootPath = "/platform"

// GetRoutes List the supported routes of this handler
func GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/registration",
			Method:      "POST",
			HandlerFunc: CreateRegistation,
		},
		models.Route{
			Path:        rootPath + "/registration",
			Method:      "GET",
			HandlerFunc: GetRegistations,
		},
		models.Route{
			Path:        rootPath + "/registration/{appName}",
			Method:      "GET",
			HandlerFunc: GetRegistation,
		},
		models.Route{
			Path:        rootPath + "/registration/{appName}",
			Method:      "DELETE",
			HandlerFunc: DeleteRegistation,
		},
		models.Route{
			Path:        rootPath + "/registration/{appName}/pipeline",
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
	for {
		select {
		case <-unsubscribe:
			return
		default:
			radixRegistration := &v1.RadixRegistration{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "radix.equinor.com/v1",
					Kind:       "RadixRegistration",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("Test%d", rand.Intn(100)),
				},
				Spec: v1.RadixRegistrationSpec{
					Repository:   "Some Repo",
					CloneURL:     "Some clone URL",
					SharedSecret: "Some Shared Secret",
					DeployKey:    "Some Public Key",
					AdGroups:     nil,
				},
			}

			queryData, err := getDataFromQuery(arg, radixRegistration)
			if err != nil {
				return
			}

			body, _ := json.Marshal(queryData)
			data <- body

			time.Sleep(5 * time.Second)
		}
	}
}

// GetRegistations Lists registrations
func GetRegistations(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /platform/registration registrations
	// ---
	// summary: Lists the application registrations
	// responses:
	//   "200":
	//     "$ref": "#/definitions/applicationRegistration"
	//   "404":
	//     "$ref": "#/responses/notFound"
	appRegistration, err := HandleGetRegistations(radixclient)

	if err != nil {
		utils.WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	utils.JSONResponse(w, r, &appRegistration)
}

// GetRegistation Gets registration by application name
func GetRegistation(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /platform/registration/{appName} registration
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
	//     "$ref": "#/responses/appRegResp"
	//   "404":
	//     "$ref": "#/responses/notFound"
	appName := mux.Vars(r)["appName"]
	appRegistration, err := HandleGetRegistation(radixclient, appName)

	if err != nil {
		utils.WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	utils.JSONResponse(w, r, &appRegistration)
}

// CreateRegistation Creates new registration for application
func CreateRegistation(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	var registration ApplicationRegistration
	if err := json.NewDecoder(r.Body).Decode(&registration); err != nil {
		utils.WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	appRegistration, err := HandleCreateRegistation(radixclient, registration)
	if err != nil {
		utils.WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	utils.JSONResponse(w, r, &appRegistration)
}

// DeleteRegistation Deletes registration for application
func DeleteRegistation(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation DELETE /platform/registration/{appName} application
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
	//     "$ref": "#/responses/ok"
	//   "404":
	//     "$ref": "#/responses/notFound"
	appName := mux.Vars(r)["appName"]
	err := HandleDeleteRegistation(radixclient, appName)

	if err != nil {
		utils.WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	utils.JSONResponse(w, r, "ok")
}

// CreateApplicationPipelineJob creates a pipeline job for the application
func CreateApplicationPipelineJob(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	appName := mux.Vars(r)["appName"]
	err := HandleCreateApplicationPipelineJob(client, radixclient, appName)

	if err != nil {
		utils.WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	utils.JSONResponse(w, r, "ok")
}

func getDataFromQuery(arg string, radixRegistration *v1.RadixRegistration) (*graphql.Result, error) {
	// Schema
	fields := graphql.Fields{
		"name": &graphql.Field{
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return radixRegistration.Name, nil
			},
		},
		"repository": &graphql.Field{
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return radixRegistration.Spec.Repository, nil
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
