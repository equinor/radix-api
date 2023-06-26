package predicate

import radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

func IsActiveRadixDeployment(rd radixv1.RadixDeployment) bool {
	return rd.Status.Condition == radixv1.DeploymentActive
}

func IsNotOrphanEnvironment(re radixv1.RadixEnvironment) bool {
	return !IsOrphanEnvironment(re)
}

func IsOrphanEnvironment(re radixv1.RadixEnvironment) bool {
	return re.Status.Orphaned
}