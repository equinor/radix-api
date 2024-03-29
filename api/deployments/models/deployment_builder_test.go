package models

import (
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/utils"

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
			&v1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:   deploymentName,
					Labels: map[string]string{kube.RadixJobNameLabel: jobName},
				},
				Spec: v1.RadixDeploymentSpec{
					Environment: envName,
					Components: []v1.RadixDeployComponent{
						{Name: "comp1", Image: "comp_image1"},
						{Name: "comp2", Image: "comp_image2"},
					},
					Jobs: []v1.RadixDeployJobComponent{
						{Name: "job1", Image: "job_image1"},
						{Name: "job2", Image: "job_image2"},
					},
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
			Components: []*ComponentSummary{
				{Name: "comp1", Image: "comp_image1", Type: string(v1.RadixComponentTypeComponent)},
				{Name: "comp2", Image: "comp_image2", Type: string(v1.RadixComponentTypeComponent)},
				{Name: "job1", Image: "job_image1", Type: string(v1.RadixComponentTypeJob)},
				{Name: "job2", Image: "job_image2", Type: string(v1.RadixComponentTypeJob)},
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

	t.Run("deploy specific components", func(t *testing.T) {
		t.Parallel()
		b := NewDeploymentBuilder().WithPipelineJob(
			&v1.RadixJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: jobName,
				},
				Spec: v1.RadixJobSpec{
					PipeLineType: v1.Deploy,
					Deploy: v1.RadixDeploySpec{
						ToEnvironment:      "dev",
						CommitID:           commitID,
						ComponentsToDeploy: []string{"comp1", "job1"},
					},
				},
			},
		).WithRadixDeployment(&v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "rd1"},
			Spec: v1.RadixDeploymentSpec{
				Components: []v1.RadixDeployComponent{
					{Name: "comp1"},
					{Name: "comp2"},
				},
				Jobs: []v1.RadixDeployJobComponent{
					{Name: "job1"},
					{Name: "job2"},
				},
			},
		}).WithGitCommitHash("commit1").WithGitTags("git1,git2")

		actual, err := b.BuildDeploymentSummary()

		assert.NoError(t, err)
		assert.Equal(t, "commit1", actual.GitCommitHash)
		assert.Equal(t, "git1,git2", actual.GitTags)
		assert.Equal(t, "comp1", actual.Components[0].Name)
		assert.False(t, actual.Components[0].SkipDeployment)
		assert.Equal(t, "comp2", actual.Components[1].Name)
		assert.True(t, actual.Components[1].SkipDeployment)
		assert.Equal(t, "job1", actual.Components[2].Name)
		assert.False(t, actual.Components[2].SkipDeployment)
		assert.Equal(t, "job2", actual.Components[3].Name)
		assert.True(t, actual.Components[3].SkipDeployment)
	})
}

func Test_DeploymentBuilder_BuildDeployment(t *testing.T) {
	appName, deploymentName, deploymentNamespace, envName, jobName, activeFrom, activeTo, cloneUrl, repoUrl :=
		"app-name", "deployment-name", "deployment-namespace", "env-name", "job-name", time.Now().Add(-10*time.Second).Truncate(1*time.Second),
		time.Now().Truncate(1*time.Second), "git@github.com:equinor/radix-canary-golang.git",
		"https://github.com/equinor/radix-canary-golang"

	rr := utils.NewRegistrationBuilder().
		WithName(appName).
		WithCloneURL(cloneUrl).
		BuildRR()

	t.Run("build with deployment", func(t *testing.T) {
		t.Parallel()

		b := NewDeploymentBuilder().WithRadixDeployment(
			&v1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: deploymentNamespace,
					Labels:    map[string]string{kube.RadixJobNameLabel: jobName},
				},
				Spec: v1.RadixDeploymentSpec{
					Environment: envName,
				},
				Status: v1.RadixDeployStatus{
					ActiveFrom: metav1.NewTime(activeFrom),
					ActiveTo:   metav1.NewTime(activeTo),
				},
			},
		).WithRadixRegistration(rr)

		actual, err := b.BuildDeployment()
		assert.NoError(t, err)
		expected := &Deployment{
			Name:         deploymentName,
			Namespace:    deploymentNamespace,
			CreatedByJob: jobName,
			Environment:  envName,
			ActiveFrom:   radixutils.FormatTimestamp(activeFrom),
			ActiveTo:     radixutils.FormatTimestamp(activeTo),
			Repository:   repoUrl,
		}
		assert.Equal(t, expected, actual)
	})

	t.Run("deploy with specific components", func(t *testing.T) {
		t.Parallel()

		b := NewDeploymentBuilder().
			WithRadixRegistration(rr).
			WithPipelineJob(
				&v1.RadixJob{
					ObjectMeta: metav1.ObjectMeta{
						Name: jobName,
					},
					Spec: v1.RadixJobSpec{
						PipeLineType: v1.Deploy,
						Deploy: v1.RadixDeploySpec{
							ToEnvironment:      "dev",
							ComponentsToDeploy: []string{"comp1", "job1"},
						},
					},
				},
			).WithComponents([]*Component{
			{Name: "comp1"},
			{Name: "comp2"},
			{Name: "job1"},
			{Name: "job2"},
		}).WithRadixDeployment(
			&v1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: deploymentNamespace,
					Labels:    map[string]string{kube.RadixJobNameLabel: jobName},
				},
				Spec: v1.RadixDeploymentSpec{
					Environment: envName,
					Components: []v1.RadixDeployComponent{
						{Name: "comp1"},
						{Name: "comp2"},
					},
					Jobs: []v1.RadixDeployJobComponent{
						{Name: "job1"},
						{Name: "job2"},
					},
				},
				Status: v1.RadixDeployStatus{
					ActiveFrom: metav1.NewTime(activeFrom),
					ActiveTo:   metav1.NewTime(activeTo),
				},
			},
		).
			WithGitCommitHash("commit1").
			WithGitTags("git1,git2")

		actual, err := b.BuildDeployment()

		assert.NoError(t, err)
		assert.Equal(t, "commit1", actual.GitCommitHash)
		assert.Equal(t, "git1,git2", actual.GitTags)
		assert.Equal(t, "comp1", actual.Components[0].Name)
		assert.False(t, actual.Components[0].SkipDeployment)
		assert.Equal(t, "comp2", actual.Components[1].Name)
		assert.True(t, actual.Components[1].SkipDeployment)
		assert.Equal(t, "job1", actual.Components[2].Name)
		assert.False(t, actual.Components[2].SkipDeployment)
		assert.Equal(t, "job2", actual.Components[3].Name)
		assert.True(t, actual.Components[3].SkipDeployment)
	})
}
