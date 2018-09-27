package job

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"

	"github.com/statoil/radix-api-go/api/utils"
	"github.com/statoil/radix-api-go/models"
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
		logrus.Info("Unsubscribe to job data")
		close(stop)
	}()

	go controller.Run(stop)
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
