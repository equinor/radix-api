package job

import (
	"net/http"

	"github.com/statoil/radix-api-go/api/utils"
	"github.com/statoil/radix-api-go/models"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

const rootPath = "/job"

// GetRoutes List the supported routes of this handler
func GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/pipeline",
			Method:      "GET",
			HandlerFunc: GetPipelineJobs,
		},
	}

	return routes
}

// GetSubscriptions Lists subscriptions this handler offers
func GetSubscriptions() models.Subscriptions {
	return models.Subscriptions{}
}

// GetPipelineJobs gets pipeline jobs
func GetPipelineJobs(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /job/pipeline getPipelineJobs
	// ---
	// summary: Gets the pipeline jobs
	// responses:
	//   "200":
	//     "$ref": "#/responses/pipelineJobsResp"
	//   "404":
	//     "$ref": "#/responses/notFound"
	pipelines, err := HandleGetPipelineJobs(client)

	if err != nil {
		utils.WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	utils.JSONResponse(w, r, &pipelines)
}
