package job

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/statoil/radix-api/api/utils"
	"github.com/statoil/radix-api/models"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const rootPath = "/job"

// GetRoutes List the supported routes of this handler
func GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/pipelines",
			Method:      "GET",
			HandlerFunc: GetPipelineJobs,
			WatcherFunc: GetPipelineJobStream,
		},
		models.Route{
			Path:        rootPath + "/pipelines",
			Method:      "POST",
			HandlerFunc: CreatePipelineJob,
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
			HandlerFunc:        GetPipelineJobStream,
		},
	}

	return subscriptions
}

// GetPipelineJobStream Lists new pods
func GetPipelineJobStream(client kubernetes.Interface, radixclient radixclient.Interface, arg string, data chan []byte, unsubscribe chan struct{}) {
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

// GetPipelineJobs gets pipeline jobs
func GetPipelineJobs(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /job/pipelines pipelines getPipelineJobs
	// ---
	// summary: Gets the pipeline jobs
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
	pipelines, err := HandleGetPipelineJobs(client)

	if err != nil {
		utils.WriteError(w, r, err)
		return
	}

	utils.JSONResponse(w, r, pipelines)
}

// CreatePipelineJob gets pipeline jobs
func CreatePipelineJob(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /job/pipelines pipelines createPipelineJob
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
	var pipelineJob PipelineJob
	if err := json.NewDecoder(r.Body).Decode(&pipelineJob); err != nil {
		utils.WriteError(w, r, err)
		return
	}

	err := HandleCreatePipelineJob(client, &pipelineJob)
	if err != nil {
		utils.WriteError(w, r, err)
		return
	}

	utils.JSONResponse(w, r, pipelineJob)
}
