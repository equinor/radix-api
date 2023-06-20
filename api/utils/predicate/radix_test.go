package predicate

import (
	"testing"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
)

func Test_IsActiveRadixDeployment(t *testing.T) {
	assert.True(t, IsActiveRadixDeployment(radixv1.RadixDeployment{Status: radixv1.RadixDeployStatus{Condition: radixv1.DeploymentActive}}))
	assert.False(t, IsActiveRadixDeployment(radixv1.RadixDeployment{Status: radixv1.RadixDeployStatus{Condition: radixv1.DeploymentInactive}}))
	assert.False(t, IsActiveRadixDeployment(radixv1.RadixDeployment{}))
}

func Test_IsNotOrphanEnvironment(t *testing.T) {
	assert.True(t, IsNotOrphanEnvironment(radixv1.RadixEnvironment{}))
	assert.True(t, IsNotOrphanEnvironment(radixv1.RadixEnvironment{Status: radixv1.RadixEnvironmentStatus{Orphaned: false}}))
	assert.False(t, IsNotOrphanEnvironment(radixv1.RadixEnvironment{Status: radixv1.RadixEnvironmentStatus{Orphaned: true}}))
}

func Test_IsOrphanEnvironment(t *testing.T) {
	assert.True(t, IsOrphanEnvironment(radixv1.RadixEnvironment{Status: radixv1.RadixEnvironmentStatus{Orphaned: true}}))
	assert.False(t, IsOrphanEnvironment(radixv1.RadixEnvironment{}))
	assert.False(t, IsOrphanEnvironment(radixv1.RadixEnvironment{Status: radixv1.RadixEnvironmentStatus{Orphaned: false}}))
}
