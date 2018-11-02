package jobs

import (
	"encoding/json"
	"net/http"

	"k8s.io/client-go/tools/cache"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/informers"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/statoil/radix-api/api/utils"
	"github.com/statoil/radix-api/models"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

const rootPath = "/applications/{appName}"

type jobController struct {
	*models.DefaultController
}

// NewJobController Constructor
func NewJobController() models.Controller {
	return &jobController{}
}

// GetRoutes List the supported routes of this handler
func (jc *jobController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/jobs",
			Method:      "GET",
			HandlerFunc: GetApplicationJobs,
			WatcherFunc: GetApplicationJobStream,
		},
		models.Route{
			Path:        rootPath + "/jobs/{jobID}/logs",
			Method:      "GET",
			HandlerFunc: GetApplicationJobLogs,
		},
	}

	return routes
}

// GetSubscriptions Lists subscriptions this handler offers
func (jc *jobController) GetSubscriptions() models.Subscriptions {
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

// GetApplicationJobLogs Get logs of a job for an application
func GetApplicationJobLogs(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET/applications/{appName}/jobs/{jobID}/logs jobs getApplicationJobLogs
	// ---
	// summary: Gets a pipeline logs, by combining different steps (jobs) logs
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: false
	// - name: jobID
	//   in: path
	//   description: Name of pipeline job
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     description: "Successful operation"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	jobID := mux.Vars(r)["jobID"]
	pipelines, err := HandleGetApplicationJobLogs(client, appName, jobID)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.StringResponse(w, r, pipelines)
}

// GetApplicationJobStream Lists starting pipeline and build jobs
func GetApplicationJobStream(client kubernetes.Interface, radixclient radixclient.Interface, arg string, data chan []byte, unsubscribe chan struct{}) {
	factory := informers.NewSharedInformerFactoryWithOptions(client, 0)
	jobsInformer := factory.Batch().V1().Jobs().Informer()

	handleJobApplied := func(obj interface{}) {
		job := obj.(*batchv1.Job)
		pipelineJob := GetJobSummary(job)
		if pipelineJob == nil {
			return
		}
		result, _ := json.Marshal(pipelineJob)
		data <- result
	}

	jobsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { handleJobApplied(obj) },
		UpdateFunc: func(old interface{}, new interface{}) { handleJobApplied(new) },
		DeleteFunc: func(obj interface{}) { log.Infof("job deleted") },
	})
	utils.StreamInformers(data, unsubscribe, jobsInformer)
}

// GetApplicationJobs gets pipeline jobs
func GetApplicationJobs(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET/applications/{appName}/jobs jobs getApplicationJobs
	// ---
	// summary: Gets the summary of jobs for a given application
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
	//           "$ref": "#/definitions/JobSummary"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	jobSummaries, err := HandleGetApplicationJobs(client, appName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, jobSummaries)
}
