package deployment

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/statoil/radix-api/api/utils"
	"github.com/statoil/radix-api/models"

	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"

	"k8s.io/client-go/kubernetes"
)

const rootPath = "/platform"

// GetRoutes List the supported routes of this handler
func GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/deployments",
			Method:      "GET",
			HandlerFunc: GetDeployments,
		},
		models.Route{
			Path:        rootPath + "/deployments/{appName}/promote",
			Method:      "POST",
			HandlerFunc: PromoteEnvironment,
		},
	}

	return routes
}

// GetSubscriptions Lists subscriptions this handler offers
func GetSubscriptions() models.Subscriptions {
	subscriptions := models.Subscriptions{}

	return subscriptions
}

// GetDeployments Lists deployments
func GetDeployments(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /platform/deployments platform getDeployments
	// ---
	// summary: Lists the application deployments
	// parameters:
	// - name: appName
	//   in: query
	//   description: name of Radix application
	//   type: string
	//   required: false
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
	appName := r.FormValue("appName")
	environment := r.FormValue("environment")
	latest := r.FormValue("latest")

	var err error
	var useLatest = false
	if strings.TrimSpace(latest) != "" {
		useLatest, err = strconv.ParseBool(r.FormValue("latest"))
		if err != nil {
			utils.ErrorResponse(w, r, err)
			return
		}
	}

	appDeployments, err := HandleGetDeployments(radixclient, appName, environment, useLatest)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, appDeployments)
}

// PromoteEnvironment promote an environment from another environment
func PromoteEnvironment(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /platform/deployments/{appName}/promote platform promoteEnvironment
	// ---
	// summary: Promote an environment from another environment
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
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
	var promotionParameters PromotionParameters
	if err := json.NewDecoder(r.Body).Decode(&promotionParameters); err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	_, err := HandlePromoteEnvironment(client, radixclient, appName, promotionParameters)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, fmt.Sprintf("%s promoted from %s for %s", appName, promotionParameters.FromEnvironment, promotionParameters.ToEnvironment))
}
