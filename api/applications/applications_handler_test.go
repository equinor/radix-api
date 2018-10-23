package applications

import (
	"testing"

	"github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

func TestGetApplications_WithFilterOnSSHRepo_Filter(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	anyApp := NewBuilder().withName("my-app").withRepository("https://github.com/Equinor/my-app").withAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"}).BuildApplicationRegistration()
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
	builder := NewBuilder().withName("")
	_, err := HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())
	assert.Error(t, err, "HandleRegisterApplication - Cannot create application without name")
}

func TestCreateApplication_WhenRepoAndDeployKeyNotSet_GenerateDeployKey(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := NewBuilder().withName("any-name").withAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"})
	application, _ := HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	expected := ""
	actual := application.PublicKey
	assert.Equal(t, expected, actual, "HandleRegisterApplication - when repo is missing, do not generate deploy key")

	// Restart
	radixclient = fake.NewSimpleClientset()
	builder = NewBuilder().withName("any-name").withRepository("https://github.com/Equinor/an-app").withAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"})
	application, _ = HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	assert.NotEmpty(t, application.PublicKey, "HandleRegisterApplication - when repo is provided, and deploy key is not, generate deploy key")

	// Restart
	radixclient = fake.NewSimpleClientset()
	builder = NewBuilder().withName("any-name").withRepository("https://github.com/Equinor/an-app").withPublicKey("Any public key").withAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"})
	application, _ = HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	expected = "Any public key"
	actual = application.PublicKey
	assert.Equal(t, expected, actual, "HandleRegisterApplication - when repo is provided, as well as deploy key, do not generate deploy key")
}

func TestCreateApplication_DuplicateRepo_ShouldFailAsWeCannotHandleThatSituation(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := NewBuilder().withName("any-name").withRepository("https://github.com/Equinor/an-app").withAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"})
	HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	builder = NewBuilder().withName("any-other-name").withRepository("https://github.com/Equinor/an-app").withAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"})
	_, err := HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())
	assert.Error(t, err, "HandleRegisterApplication - Should not be able to create another application with the same repo")
}

func TestUpdateApplication_DuplicateRepo_ShouldFailAsWeCannotHandleThatSituation(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := NewBuilder().withName("any-name").withRepository("https://github.com/Equinor/an-app").withAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"})
	HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	builder = NewBuilder().withName("any-other-name").withRepository("https://github.com/Equinor/another-app").withAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"})
	HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	builder = NewBuilder().withName("any-other-name").withRepository("https://github.com/Equinor/an-app").withAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"})
	_, err := HandleChangeRegistrationDetails(radixclient, "Another Name", *builder.BuildApplicationRegistration())
	assert.Error(t, err, "HandleChangeRegistrationDetails - Should not be able to update application with the same repo")
}

func TestUpdateApplication_MismatchingNameOrNotExists_ShouldFailAsIllegalOperation(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := NewBuilder().withName("any-name").withAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"})
	HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	builder = NewBuilder().withName("any-name").withAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"})
	_, err := HandleChangeRegistrationDetails(radixclient, "Another Name", *builder.BuildApplicationRegistration())
	assert.Error(t, err, "HandleChangeRegistrationDetails - Should not be able to call update application with different name in parameter and body")

	builder = NewBuilder().withName("another-name").withAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"})
	_, err = HandleChangeRegistrationDetails(radixclient, "another-name", *builder.BuildApplicationRegistration())
	assert.Error(t, err, "HandleChangeRegistrationDetails - Should not be able to call update application on a non existing application")

}

func TestUpdateApplication_AbleToSetAnySpecField(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := NewBuilder().withName("any-name").withAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"})
	HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	expected := "https://github.com/Equinor/an-app"
	builder = NewBuilder().withName("any-name").withRepository(expected).withAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"})
	application, _ := HandleChangeRegistrationDetails(radixclient, "any-name", *builder.BuildApplicationRegistration())
	actual := application.Repository
	assert.Equal(t, expected, actual, "HandleChangeRegistrationDetails - repository should be updatable")

	builder = NewBuilder().withName("any-name").withAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"}).withSharedSecret("Any shared secret")
	application, _ = HandleChangeRegistrationDetails(radixclient, "any-name", *builder.BuildApplicationRegistration())
	expected = "Any shared secret"
	actual = application.SharedSecret
	assert.Equal(t, expected, actual, "HandleChangeRegistrationDetails - shared secret should be updatable")

	builder = NewBuilder().withName("any-name").withAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"}).withPublicKey("Any public key")
	application, _ = HandleChangeRegistrationDetails(radixclient, "any-name", *builder.BuildApplicationRegistration())
	expected = "Any public key"
	actual = application.PublicKey
	assert.Equal(t, expected, actual, "HandleChangeRegistrationDetails - public key should be updatable")

}

func TestGetApplication_AllFieldsAreSet(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := NewBuilder().withName("any-name").withRepository("https://github.com/a-user/a-repo").withSharedSecret("Any secret").withAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"})

	HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())
	application, _ := HandleGetApplication(radixclient, "any-name")
	assert.Equal(t, "https://github.com/a-user/a-repo", application.Repository, "HandleGetApplication - Repository is not the same")
	assert.Equal(t, "Any secret", application.SharedSecret, "HandleGetApplication - Shared secret is not the same")
	assert.Equal(t, []string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"}, application.AdGroups, "HandleGetApplication - Ad groups is not the same")

}

func TestHandleTriggerPipeline_ExistingAndNonExistingApplication_JobIsCreatedForExisting(t *testing.T) {
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := fake.NewSimpleClientset()

	_, err := HandleTriggerPipeline(kubeclient, radixclient, "", BuildDeploy.String(), PipelineParameters{Branch: "master"})
	assert.Error(t, err, "HandleTriggerPipeline - Cannot run pipeline on non defined application")

	_, err = HandleTriggerPipeline(kubeclient, radixclient, "any-app", BuildDeploy.String(), PipelineParameters{Branch: ""})
	assert.Error(t, err, "HandleTriggerPipeline - Cannot run pipeline on non defined branch")

	_, err = HandleTriggerPipeline(kubeclient, radixclient, "any-app", BuildDeploy.String(), PipelineParameters{Branch: "master"})
	assert.Error(t, err, "HandleTriggerPipeline - Cannot run pipeline on non existing app")

	builder := NewBuilder().withName("any-app").withRepository("https://github.com/Equinor/an-app").withAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"}).withSharedSecret("Any secret")
	HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())
	job, err := HandleTriggerPipeline(kubeclient, radixclient, "any-app", BuildDeploy.String(), PipelineParameters{Branch: "master"})

	assert.NoError(t, err, "HandleTriggerPipeline - Should be able to create job on existing app")
	assert.Equal(t, "master", job.Branch, "HandleTriggerPipeline - Branch was unexpected")
	assert.NotEmpty(t, job.Name, "HandleTriggerPipeline - Expected a jobname")
	assert.NotEmpty(t, job.SSHRepo, "HandleTriggerPipeline - Expected a repo")
}

func TestCloneToRepositoryURL_ValidUrl(t *testing.T) {
	cloneURL := "git@github.com:Statoil/radix-api.git"
	repo := getRepositoryURLFromCloneURL(cloneURL)

	assert.Equal(t, "https://github.com/Statoil/radix-api", repo)
}

func TestCloneToRepositoryURL_EmptyURL(t *testing.T) {
	cloneURL := ""
	repo := getRepositoryURLFromCloneURL(cloneURL)

	assert.Equal(t, "", repo)
}

func TestGetCloneURLRepo_ValidRepo_CreatesValidClone(t *testing.T) {
	expected := "git@github.com:Equinor/my-app.git"
	actual := getCloneURLFromRepo("https://github.com/Equinor/my-app")

	assert.Equal(t, expected, actual, "getCloneURLFromRepo - not equal")
}

func TestGetCloneURLRepo_EmptyRepo_CreatesEmptyClone(t *testing.T) {
	expected := ""
	actual := getCloneURLFromRepo("")

	assert.Equal(t, expected, actual, "getCloneURLFromRepo - not equal")
}
