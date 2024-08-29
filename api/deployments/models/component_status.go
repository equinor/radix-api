package models

import (
	"strings"

	"github.com/equinor/radix-api/api/utils/owner"
	commonutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	operatordefaults "github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	appsv1 "k8s.io/api/apps/v1"
)

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

func GetComponentStatus(component radixv1.RadixCommonDeployComponent, kd *appsv1.Deployment, rd *radixv1.RadixDeployment) ComponentStatus {
	if kd == nil {
		return ComponentReconciling
	}
	replicasUnavailable := kd.Status.UnavailableReplicas
	replicasReady := kd.Status.ReadyReplicas
	replicas := pointers.Val(kd.Spec.Replicas)

	if component.GetType() == radixv1.RadixComponentTypeJob && replicas == 0 {
		return StoppedComponent
	}

	if isComponentManuallyStopped(component) && replicas == 0 {
		return StoppedComponent
	}

	if isCopmonentRestarting(component, rd) {
		return ComponentRestarting
	}

	// Check if component is scaling up or down
	if replicasUnavailable > 0 || replicas < replicasReady {
		return ComponentReconciling
	}

	if owner.VerifyCorrectObjectGeneration(rd, kd, kube.RadixDeploymentObservedGeneration) {
		return ComponentOutdated
	}

	return ConsistentComponent
}

func isComponentManuallyStopped(component radixv1.RadixCommonDeployComponent) bool {
	override := component.GetReplicasOverride()

	return override != nil && *override == 0
}

func isCopmonentRestarting(component radixv1.RadixCommonDeployComponent, rd *radixv1.RadixDeployment) bool {
	restarted := component.GetEnvironmentVariables()[operatordefaults.RadixRestartEnvironmentVariable]
	if strings.EqualFold(restarted, "") {
		return false
	}
	restartedTime, err := commonutils.ParseTimestamp(restarted)
	if err != nil {
		log.Logger.Warn().Err(err).Msgf("unable to parse restarted time %v, component: %s", restarted, component.GetName())
		return false
	}
	reconciledTime := rd.Status.Reconciled
	if reconciledTime.IsZero() || restartedTime.After(reconciledTime.Time) {
		return true
	}
	return false
}
