package applications

import (
	"testing"

	"github.com/statoil/radix-api/api/utils"
	"github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

func TestGetApplications_WithFilterOnSSHRepo_Filter(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	anyApp := ABuilder().
		withRepository("https://github.com/Equinor/my-app").
		BuildApplicationRegistration()

	HandleRegisterApplication(radixclient, *anyApp)

	applications, _ := HandleGetApplications(radixclient, "git@github.com:Equinor/my-app.git")
	expected := 1
	actual := len(applications)
	assert.Equal(t, expected, actual, "GetApplications - expected to be listed")

	applications, _ = HandleGetApplications(radixclient, "git@github.com:Equinor/my-app2.git")
	expected = 0
	actual = len(applications)
	assert.Equal(t, expected, actual, "GetApplications - expected not to be listed")

	applications, _ = HandleGetApplications(radixclient, " ")
	expected = 1
	actual = len(applications)
	assert.Equal(t, expected, actual, "GetApplications - expected to be listed when no filter is provided")
}

func TestCreateApplication_NoName_ValidationError(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := ABuilder().withName("")
	_, err := HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())
	assert.Error(t, err, "HandleRegisterApplication - Cannot create application without name")
}

func TestCreateApplication_WhenRepoAndDeployKeyNotSet_GenerateDeployKey(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := ABuilder().withRepository("")
	application, _ := HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	expected := ""
	actual := application.PublicKey
	assert.Equal(t, expected, actual, "HandleRegisterApplication - when repo is missing, do not generate deploy key")

	// Restart
	radixclient = fake.NewSimpleClientset()
	builder = ABuilder().
		withName("any-name").
		withRepository("https://github.com/Equinor/any-repo")

	application, _ = HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	assert.NotEmpty(t, application.PublicKey, "HandleRegisterApplication - when repo is provided, and deploy key is not, generate deploy key")

	// Restart
	radixclient = fake.NewSimpleClientset()
	builder = ABuilder().
		withName("any-name").
		withRepository("https://github.com/Equinor/any-repo").
		withPublicKey("Any public key")
	application, _ = HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	expected = "Any public key"
	actual = application.PublicKey
	assert.Equal(t, expected, actual, "HandleRegisterApplication - when repo is provided, as well as deploy key, do not generate deploy key")
}

func TestCreateApplication_DuplicateRepo_ShouldFailAsWeCannotHandleThatSituation(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := ABuilder().
		withName("any-name").
		withRepository("https://github.com/Equinor/any-repo")

	HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	builder = ABuilder().
		withName("any-other-name").
		withRepository("https://github.com/Equinor/any-repo")

	_, err := HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())
	assert.Error(t, err, "HandleRegisterApplication - Should not be able to create another application with the same repo")
}

func TestUpdateApplication_DuplicateRepo_ShouldFailAsWeCannotHandleThatSituation(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := ABuilder().
		withName("any-name").
		withRepository("https://github.com/Equinor/any-repo")

	HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	builder = ABuilder().
		withName("any-other-name").
		withRepository("https://github.com/Equinor/any-other-repo")

	HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	builder = ABuilder().
		withName("any-other-name").
		withRepository("https://github.com/Equinor/any-repo")

	_, err := HandleChangeRegistrationDetails(radixclient, "any-other-name", *builder.BuildApplicationRegistration())
	assert.Error(t, err, "HandleChangeRegistrationDetails - Should not be able to update application with the same repo")
}

func TestUpdateApplication_MismatchingNameOrNotExists_ShouldFailAsIllegalOperation(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := ABuilder().withName("any-name")
	HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	builder = ABuilder().withName("any-name")
	_, err := HandleChangeRegistrationDetails(radixclient, "another-name", *builder.BuildApplicationRegistration())
	assert.Error(t, err, "HandleChangeRegistrationDetails - Should not be able to call update application with different name in parameter and body")

	builder = ABuilder().withName("another-name")
	_, err = HandleChangeRegistrationDetails(radixclient, "another-name", *builder.BuildApplicationRegistration())
	assert.Error(t, err, "HandleChangeRegistrationDetails - Should not be able to call update application on a non existing application")

}

func TestUpdateApplication_AbleToSetAnySpecField(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := ABuilder().
		withName("any-name").
		withRepository("").
		withSharedSecret("").
		withPublicKey("")
	HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	builder = builder.
		withRepository("https://github.com/Equinor/any-repo")

	application, _ := HandleChangeRegistrationDetails(radixclient, "any-name", *builder.BuildApplicationRegistration())
	expected := "https://github.com/Equinor/any-repo"
	actual := application.Repository
	assert.Equal(t, expected, actual, "HandleChangeRegistrationDetails - repository should be updatable")

	builder = builder.
		withSharedSecret("Any shared secret")

	application, _ = HandleChangeRegistrationDetails(radixclient, "any-name", *builder.BuildApplicationRegistration())
	expected = "Any shared secret"
	actual = application.SharedSecret
	assert.Equal(t, expected, actual, "HandleChangeRegistrationDetails - shared secret should be updatable")

	builder = builder.
		withPublicKey("Any public key")

	application, _ = HandleChangeRegistrationDetails(radixclient, "any-name", *builder.BuildApplicationRegistration())
	expected = "Any public key"
	actual = application.PublicKey
	assert.Equal(t, expected, actual, "HandleChangeRegistrationDetails - public key should be updatable")

}

func TestGetApplication_AllFieldsAreSet(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := ABuilder().
		withName("any-name").
		withRepository("https://github.com/Equinor/any-repo").
		withSharedSecret("Any secret").
		withAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"})

	HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())
	application, _ := HandleGetApplication(radixclient, "any-name")
	assert.Equal(t, "https://github.com/Equinor/any-repo", application.Repository, "HandleGetApplication - Repository is not the same")
	assert.Equal(t, "Any secret", application.SharedSecret, "HandleGetApplication - Shared secret is not the same")
	assert.Equal(t, []string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"}, application.AdGroups, "HandleGetApplication - Ad groups is not the same")

}

func TestHandleTriggerPipeline_ExistingAndNonExistingApplication_JobIsCreatedForExisting(t *testing.T) {
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := fake.NewSimpleClientset()

	const pushCommitID = "4faca8595c5283a9d0f17a623b9255a0d9866a2e"

	_, err := HandleTriggerPipeline(kubeclient, radixclient, "", BuildDeploy.String(), PipelineParameters{Branch: "master", CommitID: pushCommitID})
	assert.Error(t, err, "HandleTriggerPipeline - Cannot run pipeline on non defined application")
	assert.Equal(t, "App name, branch and commit ID are required", (err.(*utils.Error)).Message)

	_, err = HandleTriggerPipeline(kubeclient, radixclient, "any-app", BuildDeploy.String(), PipelineParameters{Branch: "", CommitID: pushCommitID})
	assert.Error(t, err, "HandleTriggerPipeline - Cannot run pipeline on non defined branch")
	assert.Equal(t, "App name, branch and commit ID are required", (err.(*utils.Error)).Message)

	_, err = HandleTriggerPipeline(kubeclient, radixclient, "any-app", BuildDeploy.String(), PipelineParameters{Branch: "master", CommitID: ""})
	assert.Error(t, err, "HandleTriggerPipeline - Cannot run pipeline on non defined commit ID")
	assert.Equal(t, "App name, branch and commit ID are required", (err.(*utils.Error)).Message)

	_, err = HandleTriggerPipeline(kubeclient, radixclient, "any-app", BuildDeploy.String(), PipelineParameters{Branch: "master", CommitID: pushCommitID})
	assert.Error(t, err, "HandleTriggerPipeline - Cannot run pipeline on non existing app")
	assert.Equal(t, "radixregistrations.radix.equinor.com \"any-app\" not found", (err.(*errors.StatusError)).ErrStatus.Message)

	builder := ABuilder().
		withName("any-app")
	HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())
	job, err := HandleTriggerPipeline(kubeclient, radixclient, "any-app", BuildDeploy.String(), PipelineParameters{Branch: "master", CommitID: pushCommitID})

	assert.NoError(t, err, "HandleTriggerPipeline - Should be able to create job on existing app")
	assert.Equal(t, "master", job.Branch, "HandleTriggerPipeline - Branch was expected")
	assert.Equal(t, pushCommitID, job.CommitID, "HandleTriggerPipeline - CommitID was expected")
	assert.NotEmpty(t, job.Name, "HandleTriggerPipeline - Expected a jobname")
	assert.NotEmpty(t, job.SSHRepo, "HandleTriggerPipeline - Expected a repo")
}
