package models

import (
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	"testing"
)

func Test_GetStatusFromJobStatus(t *testing.T) {
	scenarios := []struct {
		name           string
		jobStatus      batchv1.JobStatus
		replicaList    []deploymentModels.ReplicaSummary
		expectedStatus string
	}{
		{
			name:      "Succeeded job",
			jobStatus: batchv1.JobStatus{Succeeded: 1},
			replicaList: []deploymentModels.ReplicaSummary{
				{
					Name:          "replica1",
					Status:        deploymentModels.ReplicaStatus{Status: Succeeded.String()},
					StatusMessage: "",
				},
			},
			expectedStatus: "Succeeded",
		},
		{
			name:      "Failed job by job status",
			jobStatus: batchv1.JobStatus{Failed: 1},
			replicaList: []deploymentModels.ReplicaSummary{
				{
					Name:          "replica1",
					Status:        deploymentModels.ReplicaStatus{Status: Succeeded.String()},
					StatusMessage: "",
				},
			},
			expectedStatus: "Failed",
		},
		{
			name:      "Failed job by failed replica",
			jobStatus: batchv1.JobStatus{Active: 1},
			replicaList: []deploymentModels.ReplicaSummary{
				{
					Name:          "replica1",
					Status:        deploymentModels.ReplicaStatus{Status: deploymentModels.Failing.String()},
					StatusMessage: "some error",
				},
			},
			expectedStatus: "Failed",
		},
		{
			name: "Failed job with condition",
			jobStatus: batchv1.JobStatus{Conditions: []batchv1.JobCondition{
				{
					Type: batchv1.JobFailed,
				},
			}},
			replicaList: []deploymentModels.ReplicaSummary{
				{
					Name:   "replica1",
					Status: deploymentModels.ReplicaStatus{Status: deploymentModels.Pending.String()},
				},
			},
			expectedStatus: "Failed",
		},
	}
	t.Run("Get status", func(t *testing.T) {
		t.Parallel()
		for _, scenario := range scenarios {
			t.Log(scenario.name)
			status := GetStatusFromJobStatus(scenario.jobStatus, scenario.replicaList)

			assert.Equal(t, scenario.expectedStatus, status.String())
		}
	})
}
