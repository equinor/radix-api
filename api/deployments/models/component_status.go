package models

import appsv1 "k8s.io/api/apps/v1"

// ComponentStatus Enumeration of the statuses of component
type ComponentStatus int

const (
	// StoppedComponent stopped component
	StoppedComponent ComponentStatus = iota

	// ConsistentComponent consistent component
	ConsistentComponent

	// ComponentReconciling Component reconciling
	ComponentReconciling

	// ComponentRestarting restarting component
	ComponentRestarting

	// ComponentOutdated has outdated image
	ComponentOutdated

	numComponentStatuses
)

func (p ComponentStatus) String() string {
	if p >= numComponentStatuses {
		return "Unsupported"
	}
	return [...]string{"Stopped", "Consistent", "Reconciling", "Restarting", "Outdated"}[p]
}

func ComponentStatusFromDeployment(deployment *appsv1.Deployment) ComponentStatus {
	status := ConsistentComponent

	switch {
	case deployment.Status.Replicas == 0:
		status = StoppedComponent
	case deployment.Status.UnavailableReplicas > 0:
		status = ComponentReconciling
	}

	return status
}
