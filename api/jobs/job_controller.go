package jobs

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/statoil/radix-api/api/utils"
	"github.com/statoil/radix-api/models"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const rootPath = "/applications/{appName}"

// GetRoutes List the supported routes of this handler
func GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/jobs",
			Method:      "GET",
			HandlerFunc: GetApplicationJobDetails,
			WatcherFunc: GetApplicationJobStream,
		},
		models.Route{
			Path:        rootPath + "/jobs",
			Method:      "POST",
			HandlerFunc: StartPipelineJob,
		},
	}

	return routes
}

// GetSubscriptions Lists subscriptions this handler offers
func GetSubscriptions() models.Subscriptions {
	subscriptions := models.Subscriptions{
		models.Subscription{
			SubcribeCommand:    "job_subscribe",
			UnsubscribeCommand: "job_unsubscribe",
			DataType:           "job",
			HandlerFunc:        GetApplicationJobStream,
		},
	}

	return subscriptions
}

// GetApplicationJobStream Lists new pods
func GetApplicationJobStream(client kubernetes.Interface, radixclient radixclient.Interface, arg string, data chan []byte, unsubscribe chan struct{}) {
	watchList := cache.NewFilteredListWatchFromClient(client.BatchV1().RESTClient(), "jobs", corev1.NamespaceAll,
		func(options *metav1.ListOptions) {
		})
	_, controller := cache.NewInformer(
		watchList,
		&batchv1.Job{},
		time.Second*30,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				job := obj.(*batchv1.Job)
				body, _ := json.Marshal(PipelineJob{Name: job.Name})
				data <- body
			},
		},
	)

	stop := make(chan struct{})
	go func() {
		<-unsubscribe
		close(stop)
	}()

	go controller.Run(stop)
}

// GetApplicationJobDetails gets pipeline jobs
func GetApplicationJobDetails(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET/applications/{appName}/jobs jobs getApplicationJobDetails
	// ---
	// summary: Gets the pipeline jobs
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     description: "Successful operation"
	//     schema:
	//        type: "array"
	//        items:
	//           "$ref": "#/definitions/PipelineJob"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	pipelines, err := HandleGetApplicationJobDetails(client, appName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, pipelines)
}

// StartPipelineJob gets pipeline jobs
func StartPipelineJob(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST/applications/{appName}/jobs jobs startPipelineJob
	// ---
	// summary: Create a pipeline job
	// parameters:
	// - name: pipelineJob
	//   in: body
	//   description: Pipeline job to start
	//   required: true
	//   schema:
	//       "$ref": "#/definitions/PipelineJob"
	// responses:
	//   "200":
	//     "$ref": "#/definitions/PipelineJob"
	//   "400":
	//     description: "Invalid job"
	//   "401":
	//     description: "Unauthorized"
	appName := mux.Vars(r)["appName"]

	var pipelineJob PipelineJob
	if err := json.NewDecoder(r.Body).Decode(&pipelineJob); err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	err := HandleStartPipelineJob(client, appName, &pipelineJob)
	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, pipelineJob)
}
