package models

import (
	"fmt"
	"sort"
	"strings"

	radixutils "github.com/equinor/radix-common/utils"
	corev1 "k8s.io/api/core/v1"
)

// Component describe an component part of an deployment
// swagger:model Component
type Component struct {
	// Name the component
	//
	// required: true
	// example: server
	Name string `json:"name"`

	// Type of component
	//
	// required: true
	// example: component
	Type string `json:"type"`

	// Status of the component
	// required: false
	// - Stopped = Component is stopped (no replica)
	// - Consistent = Component is consistent with config
	// - Restarting = User has triggered restart, but this is not reconciled
	//
	// example: Consistent
	Status string `json:"status"`

	// Image name
	//
	// required: true
	// example: radixdev.azurecr.io/app-server:cdgkg
	Image string `json:"image"`

	// Ports defines the port number and protocol that a component is exposed for internally in environment
	//
	// required: false
	// type: "array"
	// items:
	//    "$ref": "#/definitions/Port"
	Ports []Port `json:"ports"`

	// SchedulerPort defines the port number that a Job Scheduler is exposed internally in environment
	//
	// required: false
	// example: 8080
	SchedulerPort *int32 `json:"schedulerPort,omitempty"`

	// ScheduledJobPayloadPath defines the payload path, where payload for Job Scheduler will be mapped as a file. From radixconfig.yaml
	//
	// required: false
	// example: "/tmp/payload"
	ScheduledJobPayloadPath string `json:"scheduledJobPayloadPath,omitempty"`

	// Component secret names. From radixconfig.yaml
	//
	// required: false
	// example: ["DB_CON", "A_SECRET"]
	Secrets []string `json:"secrets"`

	// Variable names map to values. From radixconfig.yaml
	//
	// required: false
	Variables map[string]string `json:"variables"`

	// Array of pod names
	//
	// required: false
	// deprecated: true
	// example: ["server-78fc8857c4-hm76l", "server-78fc8857c4-asfa2"]
	// Deprecated: Use ReplicaList instead.
	Replicas []string `json:"replicas"`

	// Array of ReplicaSummary
	//
	// required: false
	ReplicaList []ReplicaSummary `json:"replicaList"`

	// HorizontalScaling defines horizontal scaling summary for this component
	//
	// required: false
	HorizontalScalingSummary *HorizontalScalingSummary `json:"horizontalScalingSummary"`

	// External identity information
	//
	// required: false
	Identity *Identity `json:"identity,omitempty"`

	// Notifications is the spec for notification about internal events or changes
	Notifications *Notifications `json:"notifications,omitempty"`

	AuxiliaryResource `json:",inline"`
}

// Identity describes external identities
type Identity struct {
	// Azure identity
	//
	// required: false
	Azure *AzureIdentity `json:"azure,omitempty"`
}

// Notifications is the spec for notification about internal events or changes
type Notifications struct {
	// Webhook is a URL for notification about internal events or changes. The URL should be of a Radix component or job-component, with not public port.
	//
	// required: false
	Webhook *string `json:"webhook,omitempty"`
}

// AzureIdentity describes an identity in Azure
type AzureIdentity struct {
	// ClientId is the client ID of an Azure User Assigned Managed Identity
	// or the application ID of an Azure AD Application Registration
	//
	// required: true
	ClientId string `json:"clientId,omitempty"`

	// The Service Account name to use when configuring Kubernetes Federation Credentials for the identity
	//
	// required: true
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

// AuxiliaryResource describes an auxiliary resources for a component
type AuxiliaryResource struct {
	// OAuth2 describes the oauth2 resource
	//
	// required: false
	// - oauth: OAuth2 auxiliary resource
	OAuth2 *OAuth2AuxiliaryResource `json:"oauth2,omitempty"`
}

type OAuth2AuxiliaryResource struct {
	// Deployment describes the underlying Kubernetes deployment for the resource
	//
	// required: true
	Deployment AuxiliaryResourceDeployment `json:"deployment,omitempty"`
}

// AuxiliaryResourceDeployment describes the state of the auxiliary resource's deployment
// swagger:model AuxiliaryResourceDeployment
type AuxiliaryResourceDeployment struct {
	// Status of the auxiliary resource's deployment
	// required: true
	// - Consistent: All replicas are running with the desired state
	// - Reconciling: Waiting for new replicas to enter desired state
	// - Stopped: Replica count is set to 0
	//
	// example: Consistent
	Status string `json:"status"`

	// Running replicas of the auxiliary resource's deployment
	//
	// required: false
	ReplicaList []ReplicaSummary `json:"replicaList"`
}

// Port describe a port of a component
// swagger:model Port
type Port struct {
	// Component port name. From radixconfig.yaml
	//
	// required: true
	// example: http
	Name string `json:"name"`

	// Component port number. From radixconfig.yaml
	//
	// required: false
	// example: 8080
	Port int32 `json:"port"`
}

// ComponentSummary describe a component part of a deployment
// swagger:model ComponentSummary
type ComponentSummary struct {
	// Name the component
	//
	// required: true
	// example: server
	Name string `json:"name"`

	// Type of component
	//
	// required: true
	// example: component
	Type string `json:"type"`

	// Image name
	//
	// required: true
	// example: radixdev.azurecr.io/app-server:cdgkg
	Image string `json:"image"`
}

// ReplicaSummary describes condition of a pod
// swagger:model ReplicaSummary
type ReplicaSummary struct {
	// Pod name
	//
	// required: true
	// example: server-78fc8857c4-hm76l
	Name string `json:"name"`

	// Created timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Created string `json:"created"`

	// Status describes the component container status
	//
	// required: false
	Status ReplicaStatus `json:"replicaStatus"`

	// StatusMessage provides message describing the status of a component container inside a pod
	//
	// required: false
	StatusMessage string `json:"statusMessage"`

	// RestartCount count of restarts of a component container inside a pod
	//
	// required: false
	RestartCount int32 `json:"restartCount"`

	// The image the container is running.
	//
	// required: false
	// example: radixdev.azurecr.io/app-server:cdgkg
	Image string `json:"image"`

	// ImageID of the container's image.
	//
	// required: false
	// example: radixdev.azurecr.io/app-server@sha256:d40cda01916ef63da3607c03785efabc56eb2fc2e0dab0726b1a843e9ded093f
	ImageId string `json:"imageId"`

	// Resources Resource requirements for the pod
	//
	// required: false
	Resources ResourceRequirements `json:"resources,omitempty"`
}

// ReplicaStatus describes the status of a component container inside a pod
// swagger:model ReplicaStatus
type ReplicaStatus struct {
	// Status of the container
	// - Pending = Container in Waiting state and the reason is ContainerCreating
	// - Failing = Container in Waiting state and the reason is anything else but ContainerCreating
	// - Running = Container in Running state
	// - Terminated = Container in Terminated state
	//
	// Enum: Pending,Failing,Running,Terminated,Starting
	// required: true
	// example: Running
	Status string `json:"status"`
}

// HorizontalScalingSummary describe the summary of horizontal scaling of a component
// swagger:model HorizontalScalingSummary
type HorizontalScalingSummary struct {
	// Component minimum replicas. From radixconfig.yaml
	//
	// required: false
	// example: 2
	MinReplicas int32 `json:"minReplicas"`

	// Component maximum replicas. From radixconfig.yaml
	//
	// required: false
	// example: 5
	MaxReplicas int32 `json:"maxReplicas"`

	// Component current average CPU utilization over all pods, represented as a percentage of requested CPU
	//
	// required: false
	// example: 70
	CurrentCPUUtilizationPercentage int32 `json:"currentCPUUtilizationPercentage"`

	// Component target average CPU utilization over all pods
	//
	// required: false
	// example: 80
	TargetCPUUtilizationPercentage int32 `json:"targetCPUUtilizationPercentage"`
}

// ScheduledJobSummary holds general information about scheduled job
// swagger:model ScheduledJobSummary
type ScheduledJobSummary struct {
	// Name of the scheduled job
	//
	// required: false
	// example: job-component-20181029135644-algpv-6hznh
	Name string `json:"name"`

	// Created timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Created string `json:"created,omitempty"`

	// Started timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Started string `json:"started,omitempty"`

	// Ended timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Ended string `json:"ended,omitempty"`

	// Status of the job
	//
	// required: true
	// Enum: Waiting,Running,Succeeded,Stopping,Stopped,Failed
	// example: Waiting
	Status string `json:"status"`

	// Message of a status, if any, of the job
	//
	// required: false
	// example: "Error occurred"
	Message string `json:"message,omitempty"`

	// Array of ReplicaSummary
	//
	// required: false
	ReplicaList []ReplicaSummary `json:"replicaList,omitempty"`

	// JobId JobId, if any
	//
	// required: false
	// example: "job1"
	JobId string `json:"jobId,omitempty"`

	// BatchName Batch name, if any
	//
	// required: false
	// example: "batch-abc"
	BatchName string `json:"batchName,omitempty"`

	// TimeLimitSeconds How long the job supposed to run at maximum
	//
	// required: false
	// example: 3600
	TimeLimitSeconds *int64 `json:"timeLimitSeconds,omitempty"`

	// BackoffLimit Amount of retries due to a logical error in configuration etc.
	//
	// required: true
	// example: 1
	BackoffLimit int32 `json:"backoffLimit"`

	// Resources Resource requirements for the job
	//
	// required: false
	Resources ResourceRequirements `json:"resources,omitempty"`

	// Node Defines node attributes, where pod should be scheduled
	//
	// required: false
	Node *Node `json:"node,omitempty"`

	// DeploymentName name of RadixDeployment for the job
	//
	// required: false
	DeploymentName string `json:"deploymentName,omitempty"`

	// FailedCount defines number of times the job has failed
	//
	// required: true
	// example: 1
	FailedCount int32 `json:"failedCount"`
}

// Node Defines node attributes, where pod should be scheduled
type Node struct {
	// Gpu Holds lists of node GPU types, with dashed types to exclude
	Gpu string `json:"gpu,omitempty"`
	// GpuCount Holds minimum count of GPU on node
	GpuCount string `json:"gpuCount,omitempty"`
}

// Resources Required for pods
type Resources struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// ResourceRequirements Requirements of resources for pods
type ResourceRequirements struct {
	Limits   Resources `json:"limits,omitempty"`
	Requests Resources `json:"requests,omitempty"`
}

// ScheduledBatchSummary holds information about scheduled batch
// swagger:model ScheduledBatchSummary
type ScheduledBatchSummary struct {
	// Name of the scheduled batch
	//
	// required: true
	// example: batch-20181029135644-algpv-6hznh
	Name string `json:"name"`

	// Created timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Created string `json:"created,omitempty"`

	// Started timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Started string `json:"started,omitempty"`

	// Ended timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Ended string `json:"ended,omitempty"`

	// Status of the job
	//
	// required: true
	// Enum: Waiting,Running,Succeeded,Failed
	// example: Waiting
	Status string `json:"status"`

	// Deprecated: Message of a status, if any, of the job
	//
	// required: false
	// example: "Error occurred"
	Message string `json:"message,omitempty"`

	// Deprecated: ReplicaSummary
	//
	// required: false
	Replica *ReplicaSummary `json:"replica,omitempty"`

	// TotalJobCount count of jobs, requested to be scheduled by a batch
	//
	// required: true
	// example: 5
	TotalJobCount int `json:"totalJobCount"`

	// Jobs within the batch of ScheduledJobSummary
	//
	// required: false
	JobList []ScheduledJobSummary `json:"jobList,omitempty"`

	// DeploymentName name of RadixDeployment for the batch
	//
	// required: true
	DeploymentName string `json:"deploymentName,omitempty"`
}

func GetReplicaSummary(pod corev1.Pod) ReplicaSummary {
	replicaSummary := ReplicaSummary{}
	replicaSummary.Name = pod.GetName()
	creationTimestamp := pod.GetCreationTimestamp()
	replicaSummary.Created = radixutils.FormatTimestamp(creationTimestamp.Time)

	// Set default Pending status
	replicaSummary.Status = ReplicaStatus{Status: Pending.String()}

	if len(pod.Status.ContainerStatuses) <= 0 {
		condition := getLastReadyCondition(pod.Status.Conditions)
		if condition != nil {
			replicaSummary.Status = ReplicaStatus{Status: getReplicaStatusByPodStatus(pod.Status.Phase)}
			replicaSummary.StatusMessage = fmt.Sprintf("%s: %s", condition.Reason, condition.Message)
		}
		return replicaSummary
	}
	// We assume one component container per component pod
	containerStatus := pod.Status.ContainerStatuses[0]
	containerState := containerStatus.State

	if containerState.Waiting != nil {
		replicaSummary.StatusMessage = containerState.Waiting.Message
		if !strings.EqualFold(containerState.Waiting.Reason, "ContainerCreating") {
			replicaSummary.Status = ReplicaStatus{Status: Failing.String()}
		}
	}
	if containerState.Running != nil {
		if containerStatus.Ready {
			replicaSummary.Status = ReplicaStatus{Status: Running.String()}
		} else {
			replicaSummary.Status = ReplicaStatus{Status: Starting.String()}
		}
	}
	if containerState.Terminated != nil {
		replicaSummary.Status = ReplicaStatus{Status: Terminated.String()}
		replicaSummary.StatusMessage = containerState.Terminated.Message
	}
	replicaSummary.RestartCount = containerStatus.RestartCount
	replicaSummary.Image = containerStatus.Image
	replicaSummary.ImageId = containerStatus.ImageID
	if len(pod.Spec.Containers) > 0 {
		replicaSummary.Resources = ConvertResourceRequirements(pod.Spec.Containers[0].Resources)
	}
	return replicaSummary
}

func getReplicaStatusByPodStatus(podPhase corev1.PodPhase) string {
	switch podPhase {
	case corev1.PodPending:
		return Pending.String()
	case corev1.PodRunning:
		return Running.String()
	case corev1.PodFailed:
		return Failing.String()
	case corev1.PodSucceeded:
		return Terminated.String()
	default:
		return ""
	}
}

func getLastReadyCondition(conditions []corev1.PodCondition) *corev1.PodCondition {
	if len(conditions) == 1 {
		return &conditions[0]
	}
	conditions = sortStatusConditionsDesc(conditions)
	for _, condition := range conditions {
		if condition.Status == corev1.ConditionTrue {
			return &condition
		}
	}
	if len(conditions) > 0 {
		return &conditions[0]
	}
	return nil
}

func sortStatusConditionsDesc(conditions []corev1.PodCondition) []corev1.PodCondition {
	sort.Slice(conditions, func(i, j int) bool {
		if conditions[i].LastTransitionTime.Time.IsZero() || conditions[j].LastTransitionTime.Time.IsZero() {
			return false
		}
		return conditions[j].LastTransitionTime.Time.Before(conditions[i].LastTransitionTime.Time)
	})
	return conditions
}

func (job *ScheduledJobSummary) GetCreated() string {
	return job.Created
}

func (job *ScheduledJobSummary) GetStarted() string {
	return job.Started
}

func (job *ScheduledJobSummary) GetEnded() string {
	return job.Ended
}

func (job *ScheduledJobSummary) GetStatus() string {
	return job.Status
}

func (job *ScheduledBatchSummary) GetCreated() string {
	return job.Created
}

func (job *ScheduledBatchSummary) GetStarted() string {
	return job.Started
}

func (job *ScheduledBatchSummary) GetEnded() string {
	return job.Ended
}

func (job *ScheduledBatchSummary) GetStatus() string {
	return job.Status
}
