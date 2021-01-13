package deployments

import (
	"testing"
	"time"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRunningReplicaDiffersFromConfig_NoHPA(t *testing.T) {
	// Test replicas 2, pods 3
	replicas := 2
	raEnvironmentConfig := &v1.RadixEnvironmentConfig{
		Replicas: &replicas,
	}
	actualPods := []corev1.Pod{corev1.Pod{}, corev1.Pod{}, corev1.Pod{}}
	isDifferent := runningReplicaDiffersFromConfig(raEnvironmentConfig, actualPods)
	assert.True(t, isDifferent)

	// Test replicas 2, pods 2
	actualPods = []corev1.Pod{corev1.Pod{}, corev1.Pod{}}
	isDifferent = runningReplicaDiffersFromConfig(raEnvironmentConfig, actualPods)
	assert.False(t, isDifferent)

	// Test replicas 0, pods 2
	replicas = 0
	isDifferent = runningReplicaDiffersFromConfig(raEnvironmentConfig, actualPods)
	assert.True(t, isDifferent)

	// Test replicas nil, pods 2
	raEnvironmentConfig.Replicas = nil
	isDifferent = runningReplicaDiffersFromConfig(raEnvironmentConfig, actualPods)
	assert.True(t, isDifferent)

	// Test RadixEnvironmentConfig nil
	raEnvironmentConfig = nil
	isDifferent = runningReplicaDiffersFromConfig(raEnvironmentConfig, actualPods)
	assert.True(t, isDifferent)
}

func TestRunningReplicaDiffersFromConfig_WithHPA(t *testing.T) {
	// Test replicas 0, pods 3, minReplicas 2, maxReplicas 6
	replicas := 0
	minReplicas := int32(2)
	raEnvironmentConfig := &v1.RadixEnvironmentConfig{
		Replicas: &replicas,
		HorizontalScaling: &v1.RadixHorizontalScaling{
			MinReplicas: &minReplicas,
			MaxReplicas: 6,
		},
	}
	actualPods := []corev1.Pod{corev1.Pod{}, corev1.Pod{}, corev1.Pod{}}
	isDifferent := runningReplicaDiffersFromConfig(raEnvironmentConfig, actualPods)
	assert.True(t, isDifferent)

	// Test replicas 4, pods 3, minReplicas 2, maxReplicas 6
	replicas = 4
	isDifferent = runningReplicaDiffersFromConfig(raEnvironmentConfig, actualPods)
	assert.False(t, isDifferent)

	// Test replicas 4, pods 1, minReplicas 2, maxReplicas 6
	actualPods = []corev1.Pod{corev1.Pod{}}
	isDifferent = runningReplicaDiffersFromConfig(raEnvironmentConfig, actualPods)
	assert.True(t, isDifferent)

	// Test replicas 4, pods 1, minReplicas nil, maxReplicas 6
	raEnvironmentConfig.HorizontalScaling.MinReplicas = nil
	isDifferent = runningReplicaDiffersFromConfig(raEnvironmentConfig, actualPods)
	assert.False(t, isDifferent)
}

func TestRunningReplicaDiffersFromSpec_NoHPA(t *testing.T) {
	// Test replicas 0, pods 1
	replicas := 0
	rdComponent := v1.RadixDeployComponent{
		Replicas: &replicas,
	}
	actualPods := []corev1.Pod{corev1.Pod{}}
	isDifferent := runningReplicaDiffersFromSpec(rdComponent, actualPods)
	assert.True(t, isDifferent)

	// Test replicas 1, pods 1
	replicas = 1
	isDifferent = runningReplicaDiffersFromSpec(rdComponent, actualPods)
	assert.False(t, isDifferent)

	// Test replicas nil, pods 1
	rdComponent.Replicas = nil
	isDifferent = runningReplicaDiffersFromSpec(rdComponent, actualPods)
	assert.False(t, isDifferent)
}

func TestRunningReplicaDiffersFromSpec_WithHPA(t *testing.T) {
	// Test replicas 0, pods 1, minReplicas 2, maxReplicas 6
	replicas := 0
	minReplicas := int32(2)
	rdComponent := v1.RadixDeployComponent{
		Replicas: &replicas,
		HorizontalScaling: &v1.RadixHorizontalScaling{
			MinReplicas: &minReplicas,
			MaxReplicas: 6,
		},
	}
	actualPods := []corev1.Pod{corev1.Pod{}}
	isDifferent := runningReplicaDiffersFromSpec(rdComponent, actualPods)
	assert.True(t, isDifferent)

	// Test replicas 1, pods 1, minReplicas 2, maxReplicas 6
	replicas = 1
	isDifferent = runningReplicaDiffersFromSpec(rdComponent, actualPods)
	assert.True(t, isDifferent)

	// Test replicas 1, pods 3, minReplicas 2, maxReplicas 6
	actualPods = []corev1.Pod{corev1.Pod{}, corev1.Pod{}, corev1.Pod{}}
	isDifferent = runningReplicaDiffersFromSpec(rdComponent, actualPods)
	assert.False(t, isDifferent)

	// Test replicas 1, pods 3, minReplicas nil, maxReplicas 6
	rdComponent.HorizontalScaling.MinReplicas = nil
	isDifferent = runningReplicaDiffersFromSpec(rdComponent, actualPods)
	assert.False(t, isDifferent)
}

func TestRunningReplicaOutdatedImage(t *testing.T) {
	// Test replicas 0, pods 1, minReplicas 2, maxReplicas 6
	replicas := 0
	minReplicas := int32(2)
	rdComponent := v1.RadixDeployComponent{
		Image:    "not-outdated",
		Replicas: &replicas,
		HorizontalScaling: &v1.RadixHorizontalScaling{
			MinReplicas: &minReplicas,
			MaxReplicas: 6,
		},
	}

	actualPods := []corev1.Pod{
		{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "test",
						Image: "outdated",
					},
				},
			},
		},
	}

	isOutdated := runningReplicaIsOutdated(rdComponent, actualPods)
	assert.True(t, isOutdated)

}

func TestRunningReplicaNotOutdatedImage_(t *testing.T) {
	// Test replicas 0, pods 1, minReplicas 2, maxReplicas 6
	replicas := 0
	minReplicas := int32(2)
	rdComponent := v1.RadixDeployComponent{
		Image:    "not-outdated",
		Replicas: &replicas,
		HorizontalScaling: &v1.RadixHorizontalScaling{
			MinReplicas: &minReplicas,
			MaxReplicas: 6,
		},
	}

	actualPods := []corev1.Pod{
		{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "test",
						Image: "not-outdated",
					},
				},
			},
		},
	}

	isOutdated := runningReplicaIsOutdated(rdComponent, actualPods)
	assert.False(t, isOutdated)
}

func TestRunningReplicaNotOutdatedImage_TerminatingPod(t *testing.T) {
	// Test replicas 0, pods 1, minReplicas 2, maxReplicas 6
	replicas := 0
	minReplicas := int32(2)
	rdComponent := v1.RadixDeployComponent{
		Image:    "not-outdated",
		Replicas: &replicas,
		HorizontalScaling: &v1.RadixHorizontalScaling{
			MinReplicas: &minReplicas,
			MaxReplicas: 6,
		},
	}

	actualPods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				DeletionTimestamp: &metav1.Time{
					Time: time.Now(),
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "test",
						Image: "not-outdated",
					},
				},
			},
		},
	}

	isOutdated := runningReplicaIsOutdated(rdComponent, actualPods)
	assert.False(t, isOutdated)
}
