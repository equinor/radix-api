package deployments

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/statoil/radix-api/api/utils"
	"github.com/statoil/radix-api/models"

	deploymentModels "github.com/statoil/radix-api/api/deployments/models"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"

	"k8s.io/client-go/kubernetes"
)

const rootPath = "/applications/{appName}"

type deploymentController struct {
	*models.DefaultController
}

// NewDeploymentController Constructor
func NewDeploymentController() models.Controller {
	return &deploymentController{}
}

// GetRoutes List the supported routes of this handler
func (dc *deploymentController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/deployments",
			Method:      "GET",
			HandlerFunc: GetDeployments,
		},
		models.Route{
			Path:        rootPath + "/deployments/{deploymentName}/components/{componentName}/replicas/{podName}/logs",
			Method:      "GET",
			HandlerFunc: GetPodLog,
		},
		models.Route{
			Path:        rootPath + "/deployments/{deploymentName}/promote",
			Method:      "POST",
			HandlerFunc: PromoteToEnvironment,
		},
	}

	return routes
}

// GetSubscriptions Lists subscriptions this handler offers
func (dc *deploymentController) GetSubscriptions() models.Subscriptions {
	subscriptions := models.Subscriptions{}

	return subscriptions
}

// GetDeployments Lists deployments
func GetDeployments(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/deployments application getDeployments
	// ---
	// summary: Lists the application deployments
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: environment
	//   in: query
	//   description: environment of Radix application
	//   type: string
	//   required: false
	// - name: latest
	//   in: query
	//   description: indicator to allow only listing latest
	//   type: boolean
	//   required: false
	// responses:
	//   "200":
	//     description: "Successful operation"
	//     schema:
	//        type: "array"
	//        items:
	//           "$ref": "#/definitions/ApplicationDeployment"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	environment := r.FormValue("environment")
	latest := r.FormValue("latest")

	deploy := Init(client, radixclient)

	var err error
	var useLatest = false
	if strings.TrimSpace(latest) != "" {
		useLatest, err = strconv.ParseBool(r.FormValue("latest"))
		if err != nil {
			utils.ErrorResponse(w, r, err)
			return
		}
	}

	appDeployments, err := deploy.HandleGetDeployments(appName, environment, useLatest)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, appDeployments)
}

// Get logs of a single pod
func GetPodLog(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/deployments/{deploymentName}/components/{componentName}/replicas/{podName}/logs deployment log
	// ---
	// summary: Get logs from a deployed pod
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: deploymentName
	//   in: path
	//   description: Name of deployment
	//   type: string
	//   required: true
	// - name: componentName
	//   in: path
	//   description: Name of component
	//   type: string
	//   required: true
	// - name: podName
	//   in: path
	//   description: Name of pod
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     description: "pod log"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	// deploymentName := mux.Vars(r)["deploymentName"]
	// componentName := mux.Vars(r)["componentName"]
	podName := mux.Vars(r)["podName"]

	deploy := Init(client, radixclient)

	log, err := deploy.HandleGetLogs(appName, podName)
	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.StringResponse(w, r, log)
}

// PromoteToEnvironment promote an environment from another environment
func PromoteToEnvironment(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /applications/{appName}/deployments/{deploymentName}/promote deployment promoteToEnvironment
	// ---
	// summary: Promote an environment from another environment
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: deploymentName
	//   in: path
	//   description: Name of deployment
	//   type: string
	//   required: true
	// - name: promotionParameters
	//   in: body
	//   description: Environment to promote from and to promote to
	//   required: true
	//   schema:
	//       "$ref": "#/definitions/PromotionParameters"
	// responses:
	//   "200":
	//     description: "Promotion ok"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	deploymentName := mux.Vars(r)["deploymentName"]

	deploy := Init(client, radixclient)

	var promotionParameters deploymentModels.PromotionParameters
	if err := json.NewDecoder(r.Body).Decode(&promotionParameters); err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	_, err := deploy.HandlePromoteToEnvironment(appName, deploymentName, promotionParameters)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, fmt.Sprintf("%s promoted from %s for %s", appName, promotionParameters.FromEnvironment, promotionParameters.ToEnvironment))
}
