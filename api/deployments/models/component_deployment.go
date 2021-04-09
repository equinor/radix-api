package models

import (
	"github.com/equinor/radix-api/api/utils"
	corev1 "k8s.io/api/core/v1"
	"strings"
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
	// example: radixdev.azurecr.io/radix-api-server:cdgkg
	Image string `json:"image"`

	// Ports defines the port number and protocol that a component is exposed for internally in environment
	//
	// required: false
	// type: "array"
	// items:
	//    "$ref": "#/definitions/Port"
	Ports []Port `json:"ports"`

	// Component secret names. From radixconfig.yaml
	//
	// required: false
	// example: DB_CON,A_SECRET
	Secrets []string `json:"secrets"`

	// Variable names map to values. From radixconfig.yaml
	//
	// required: false
	Variables map[string]string `json:"variables"`

	// Array of pod names
	//
	// required: false
	// deprecated: true
	// example: server-78fc8857c4-hm76l,server-78fc8857c4-asfa2
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

	// Array of ScheduledJobList
	//
	// required: false
	ScheduledJobList []ScheduledJobSummary `json:"scheduledJobList"`
}

// Port describe an component part of an deployment
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

// ComponentSummary describe an component part of an deployment
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
	// example: radixdev.azurecr.io/radix-api-server:cdgkg
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
	RestartCount int32
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
	// Enum: Pending,Failing,Running,Terminated
	// required: true
	// example: Pending, Failing, Running, Terminated, Starting
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
	Created string `json:"created"`

	// Started timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Started string `json:"started"`

	// Ended timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Ended string `json:"ended"`

	// Status of the job
	//
	// required: false
	// Enum: Waiting,Running,Succeeded,Stopping,Stopped,Failed
	// example: Waiting
	Status string `json:"status"`

	// Array of ReplicaSummary
	//
	// required: false
	ReplicaList []ReplicaSummary `json:"replicaList"`
}

func GetReplicaSummary(pod corev1.Pod) ReplicaSummary {
	replicaSummary := ReplicaSummary{}
	replicaSummary.Name = pod.GetName()
	creationTimestamp := pod.GetCreationTimestamp()
	replicaSummary.Created = utils.FormatTimestamp(creationTimestamp.Time)
	if len(pod.Status.ContainerStatuses) <= 0 {
		return replicaSummary
	}
	// We assume one component container per component pod
	containerStatus := pod.Status.ContainerStatuses[0]
	containerState := containerStatus.State

	// Set default Pending status
	replicaSummary.Status = ReplicaStatus{Status: Pending.String()}

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
	return replicaSummary
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
