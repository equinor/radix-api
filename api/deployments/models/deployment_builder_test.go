package models

import (
	"testing"
	"time"

	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_DeploymentBuilder_BuildDeploymentSummary(t *testing.T) {
	deploymentName, envName, jobName, commitID, promoteFromEnv, activeFrom, activeTo :=
		"deployment-name", "env-name", "job-name", "commit-id", "from-env-name",
		time.Now().Add(-10*time.Second).Truncate(1*time.Second), time.Now().Truncate(1*time.Second)

	t.Run("build with deployment", func(t *testing.T) {
		t.Parallel()
		b := NewDeploymentBuilder().WithRadixDeployment(
			v1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:   deploymentName,
					Labels: map[string]string{kube.RadixJobNameLabel: jobName},
				},
				Spec: v1.RadixDeploymentSpec{
					Environment: envName,
				},
				Status: v1.RadixDeployStatus{
					ActiveFrom: metav1.NewTime(activeFrom),
					ActiveTo:   metav1.NewTime(activeTo),
				},
			},
		)

		actual, err := b.BuildDeploymentSummary()
		assert.NoError(t, err)
		expected := &DeploymentSummary{
			Name:        deploymentName,
			Environment: envName,
			ActiveFrom:  radixutils.FormatTimestamp(activeFrom),
			ActiveTo:    radixutils.FormatTimestamp(activeTo),
			DeploymentSummaryPipelineJobInfo: DeploymentSummaryPipelineJobInfo{
				CreatedByJob: jobName,
			},
		}
		assert.Equal(t, expected, actual)
	})

	t.Run("build with pipeline job info", func(t *testing.T) {
		t.Parallel()
		b := NewDeploymentBuilder().WithPipelineJob(
			&v1.RadixJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: jobName,
				},
				Spec: v1.RadixJobSpec{
					PipeLineType: v1.BuildDeploy,
					Build: v1.RadixBuildSpec{
						CommitID: commitID,
					},
					Promote: v1.RadixPromoteSpec{
						FromEnvironment: promoteFromEnv,
					},
				},
			},
		)
		actual, err := b.BuildDeploymentSummary()
		assert.NoError(t, err)
		expected := &DeploymentSummary{
			DeploymentSummaryPipelineJobInfo: DeploymentSummaryPipelineJobInfo{
				CreatedByJob:            jobName,
				CommitID:                commitID,
				PipelineJobType:         string(v1.BuildDeploy),
				PromotedFromEnvironment: promoteFromEnv,
			},
		}
		assert.Equal(t, expected, actual)
	})
}
