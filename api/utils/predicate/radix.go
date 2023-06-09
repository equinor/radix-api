package predicate

import radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

func IsActiveRadixDeployment(rd radixv1.RadixDeployment) bool {
	return rd.Status.Condition == radixv1.DeploymentActive
}
