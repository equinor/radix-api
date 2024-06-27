package models

import (
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/utils"

	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
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
			&radixv1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:   deploymentName,
					Labels: map[string]string{kube.RadixJobNameLabel: jobName},
				},
				Spec: radixv1.RadixDeploymentSpec{
					Environment: envName,
					Components: []radixv1.RadixDeployComponent{
						{Name: "comp1", Image: "comp_image1"},
						{Name: "comp2", Image: "comp_image2", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}},
						{Name: "comp3", Image: "comp_image3", Node: radixv1.RadixNode{Gpu: "anygpu", GpuCount: "3"}},
					},
					Jobs: []radixv1.RadixDeployJobComponent{
						{Name: "job1", Image: "job_image1"},
						{Name: "job2", Image: "job_image2", Runtime: &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureArm64}},
						{Name: "job3", Image: "job_image3", Node: radixv1.RadixNode{Gpu: "anygpu", GpuCount: "3"}},
					},
				},
				Status: radixv1.RadixDeployStatus{
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
				{Name: "comp1", Image: "comp_image1", Type: string(radixv1.RadixComponentTypeComponent), Runtime: &Runtime{Architecture: defaults.DefaultNodeSelectorArchitecture}},
				{Name: "comp2", Image: "comp_image2", Type: string(radixv1.RadixComponentTypeComponent), Runtime: &Runtime{Architecture: string(radixv1.RuntimeArchitectureArm64)}},
				{Name: "comp3", Image: "comp_image3", Type: string(radixv1.RadixComponentTypeComponent), Runtime: &Runtime{Architecture: defaults.DefaultNodeSelectorArchitecture}},
				{Name: "job1", Image: "job_image1", Type: string(radixv1.RadixComponentTypeJob), Runtime: &Runtime{Architecture: defaults.DefaultNodeSelectorArchitecture}},
				{Name: "job2", Image: "job_image2", Type: string(radixv1.RadixComponentTypeJob), Runtime: &Runtime{Architecture: string(radixv1.RuntimeArchitectureArm64)}},
				{Name: "job3", Image: "job_image3", Type: string(radixv1.RadixComponentTypeJob), Runtime: &Runtime{Architecture: defaults.DefaultNodeSelectorArchitecture}},
			},
		}
		assert.Equal(t, expected, actual)
	})

	t.Run("build with pipeline job info", func(t *testing.T) {
		t.Parallel()
		b := NewDeploymentBuilder().WithPipelineJob(
			&radixv1.RadixJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: jobName,
				},
				Spec: radixv1.RadixJobSpec{
					PipeLineType: radixv1.BuildDeploy,
					Build: radixv1.RadixBuildSpec{
						CommitID: commitID,
					},
					Promote: radixv1.RadixPromoteSpec{
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
				PipelineJobType:         string(radixv1.BuildDeploy),
				PromotedFromEnvironment: promoteFromEnv,
			},
		}
		assert.Equal(t, expected, actual)
	})

	t.Run("deploy specific components", func(t *testing.T) {
		t.Parallel()
		b := NewDeploymentBuilder().WithPipelineJob(
			&radixv1.RadixJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: jobName,
				},
				Spec: radixv1.RadixJobSpec{
					PipeLineType: radixv1.Deploy,
					Deploy: radixv1.RadixDeploySpec{
						ToEnvironment:      "dev",
						CommitID:           commitID,
						ComponentsToDeploy: []string{"comp1", "job1"},
					},
				},
			},
		).WithRadixDeployment(&radixv1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "rd1"},
			Spec: radixv1.RadixDeploymentSpec{
				Components: []radixv1.RadixDeployComponent{
					{Name: "comp1"},
					{Name: "comp2"},
				},
				Jobs: []radixv1.RadixDeployJobComponent{
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
			&radixv1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: deploymentNamespace,
					Labels:    map[string]string{kube.RadixJobNameLabel: jobName},
				},
				Spec: radixv1.RadixDeploymentSpec{
					Environment: envName,
				},
				Status: radixv1.RadixDeployStatus{
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
				&radixv1.RadixJob{
					ObjectMeta: metav1.ObjectMeta{
						Name: jobName,
					},
					Spec: radixv1.RadixJobSpec{
						PipeLineType: radixv1.Deploy,
						Deploy: radixv1.RadixDeploySpec{
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
			&radixv1.RadixDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: deploymentNamespace,
					Labels:    map[string]string{kube.RadixJobNameLabel: jobName},
				},
				Spec: radixv1.RadixDeploymentSpec{
					Environment: envName,
					Components: []radixv1.RadixDeployComponent{
						{Name: "comp1"},
						{Name: "comp2"},
					},
					Jobs: []radixv1.RadixDeployJobComponent{
						{Name: "job1"},
						{Name: "job2"},
					},
				},
				Status: radixv1.RadixDeployStatus{
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
