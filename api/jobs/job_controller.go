package jobs

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"k8s.io/client-go/tools/cache"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/statoil/radix-api/api/utils"
	"github.com/statoil/radix-api/models"
	"github.com/statoil/radix-operator/pkg/apis/kube"
	crdUtils "github.com/statoil/radix-operator/pkg/apis/utils"
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
		},
		models.Route{
			Path:        rootPath + "/jobs/{jobName}/logs",
			Method:      "GET",
			HandlerFunc: GetPipelineJobLogs,
		},
		models.Route{
			Path:        rootPath + "/jobs/{jobName}",
			Method:      "GET",
			HandlerFunc: GetApplicationJob,
		},
	}

	return routes
}

// GetSubscriptions Lists subscriptions this handler offers
func (jc *jobController) GetSubscriptions() models.Subscriptions {
	subscriptions := models.Subscriptions{
		models.Subscription{
			Resource:    rootPath + "/jobs",
			DataType:    "JobSummary",
			HandlerFunc: GetApplicationJobsStream,
		},
		models.Subscription{
			Resource:    rootPath + "/jobs/{jobName}",
			DataType:    "Job",
			HandlerFunc: GetApplicationJobStream,
		},
	}

	return subscriptions
}

// GetPipelineJobLogs Get logs of a job for an application
func GetPipelineJobLogs(clients models.Clients, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/jobs/{jobName}/logs job getApplicationJobLogs
	// ---
	// summary: Gets a pipeline logs, by combining different steps (jobs) logs
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: Name of pipeline job
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     description: "Successful operation"
	//     schema:
	//        type: "array"
	//        items:
	//           "$ref": "#/definitions/StepLog"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	jobName := mux.Vars(r)["jobName"]

	handler := Init(clients.OutClusterClient, clients.OutClusterRadixClient)
	pipelines, err := handler.HandleGetApplicationJobLogs(appName, jobName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	sort.Slice(pipelines, func(i, j int) bool { return pipelines[i].Sort < pipelines[j].Sort })
	utils.JSONResponse(w, r, pipelines)
}

// GetApplicationJobsStream Lists starting pipeline and build jobs
func GetApplicationJobsStream(clients models.Clients, resource string, resourceIdentifiers []string, data chan []byte, unsubscribe chan struct{}) {
	factory := informers.NewSharedInformerFactoryWithOptions(clients.OutClusterClient, 0)
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
	utils.StreamInformers(unsubscribe, jobsInformer)
}

// GetApplicationJobStream Lists starting pipeline and build jobs
func GetApplicationJobStream(clients models.Clients, resource string, resourceIdentifiers []string, data chan []byte, unsubscribe chan struct{}) {
	appNameToWatch := resourceIdentifiers[0]
	jobNameToWatch := resourceIdentifiers[1]
	namespaceToWatch := crdUtils.GetAppNamespace(appNameToWatch)

	factory := informers.NewSharedInformerFactoryWithOptions(clients.OutClusterClient, 0, informers.WithNamespace(namespaceToWatch))
	jobsInformer := factory.Batch().V1().Jobs().Informer()
	podsInformer := factory.Core().V1().Pods().Informer()

	handleJobApplied := func(obj interface{}) {
		var jobName string

		switch obj.(type) {
		case *batchv1.Job:
			job := obj.(*batchv1.Job)
			jobName = job.Labels[kube.RadixJobNameLabel]

		case *corev1.Pod:
			pod := obj.(*corev1.Pod)
			jobName = pod.Labels[kube.RadixJobNameLabel]

		default:
			return
		}

		if !strings.EqualFold(jobName, jobNameToWatch) {
			return
		}

		handler := Init(clients.OutClusterClient, clients.OutClusterRadixClient)
		radixJob, err := handler.GetApplicationJob(appNameToWatch, jobNameToWatch)
		if err != nil {
			log.Errorf("Problems getting job %s. Error was %v", jobNameToWatch, err)
			return
		}

		result, _ := json.Marshal(*radixJob)
		data <- result
	}

	jobsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { handleJobApplied(obj) },
		UpdateFunc: func(old interface{}, new interface{}) { handleJobApplied(new) },
	})

	podsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { handleJobApplied(obj) },
		UpdateFunc: func(old interface{}, new interface{}) { handleJobApplied(new) },
	})

	utils.StreamInformers(unsubscribe, jobsInformer, podsInformer)
}

// GetApplicationJobs gets job summaries
func GetApplicationJobs(clients models.Clients, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/jobs job getApplicationJobs
	// ---
	// summary: Gets the summary of jobs for a given application
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
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

	handler := Init(clients.OutClusterClient, clients.OutClusterRadixClient)
	jobSummaries, err := handler.GetApplicationJobs(appName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, jobSummaries)
}

// GetApplicationJob gets specific job details
func GetApplicationJob(clients models.Clients, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/jobs/{jobName} job getApplicationJob
	// ---
	// summary: Gets the detail of a given job for a given application
	// parameters:
	// - name: appName
	//   in: path
	//   description: name of Radix application
	//   type: string
	//   required: true
	// - name: jobName
	//   in: path
	//   description: name of job
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     description: "Successful operation"
	//     schema:
	//        type: "array"
	//        items:
	//           "$ref": "#/definitions/Job"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	appName := mux.Vars(r)["appName"]
	jobName := mux.Vars(r)["jobName"]

	handler := Init(clients.OutClusterClient, clients.OutClusterRadixClient)
	jobDetail, err := handler.GetApplicationJob(appName, jobName)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, jobDetail)
}
