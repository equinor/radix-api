package applications

import (
	"testing"

	"github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

func TestGetApplications_WithFilterOnSSHRepo_Filter(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	anyApp := NewBuilder().withName("my-app").withRepository("https://github.com/Equinor/my-app").BuildApplicationRegistration()
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
	builder := NewBuilder().withName("Any Name")
	application, _ := HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	expected := ""
	actual := application.PublicKey
	assert.Equal(t, expected, actual, "HandleRegisterApplication - when repo is missing, do not generate deploy key")

	// Restart
	radixclient = fake.NewSimpleClientset()
	builder = NewBuilder().withName("Any Name").withRepository("Any repo")
	application, _ = HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	assert.NotEmpty(t, application.PublicKey, "HandleRegisterApplication - when repo is provided, and deploy key is not, generate deploy key")

	// Restart
	radixclient = fake.NewSimpleClientset()
	builder = NewBuilder().withName("Any Name").withRepository("Any repo").withPublicKey("Any public key")
	application, _ = HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	expected = "Any public key"
	actual = application.PublicKey
	assert.Equal(t, expected, actual, "HandleRegisterApplication - when repo is provided, as well as deploy key, do not generate deploy key")
}

func TestCreateApplication_DuplicateRepo_ShouldFailAsWeCannotHandleThatSituation(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := NewBuilder().withName("Any Name").withRepository("Any repo")
	HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	builder = NewBuilder().withName("Another Name").withRepository("Any repo")
	_, err := HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())
	assert.Error(t, err, "HandleRegisterApplication - Should not be able to create another application with the same repo")
}

func TestUpdateApplication_DuplicateRepo_ShouldFailAsWeCannotHandleThatSituation(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := NewBuilder().withName("Any Name").withRepository("Any repo")
	HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	builder = NewBuilder().withName("Another Name").withRepository("Another repo")
	HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	builder = NewBuilder().withName("Another Name").withRepository("Any repo")
	_, err := HandleChangeRegistrationDetails(radixclient, "Another Name", *builder.BuildApplicationRegistration())
	assert.Error(t, err, "HandleChangeRegistrationDetails - Should not be able to update application with the same repo")
}

func TestUpdateApplication_MismatchingNameOrNotExists_ShouldFailAsIllegalOperation(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := NewBuilder().withName("Any Name")
	HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	builder = NewBuilder().withName("Any Name")
	_, err := HandleChangeRegistrationDetails(radixclient, "Another Name", *builder.BuildApplicationRegistration())
	assert.Error(t, err, "HandleChangeRegistrationDetails - Should not be able to call update application with different name in parameter and body")

	builder = NewBuilder().withName("Another Name")
	_, err = HandleChangeRegistrationDetails(radixclient, "Another Name", *builder.BuildApplicationRegistration())
	assert.Error(t, err, "HandleChangeRegistrationDetails - Should not be able to call update application on a non existing application")

}

func TestUpdateApplication_AbleToSetAnySpecField(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := NewBuilder().withName("Any Name")
	HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())

	builder = NewBuilder().withName("Any Name").withRepository("Any repo")
	application, _ := HandleChangeRegistrationDetails(radixclient, "Any Name", *builder.BuildApplicationRegistration())
	expected := "Any repo"
	actual := application.Repository
	assert.Equal(t, expected, actual, "HandleChangeRegistrationDetails - repository should be updatable")

	builder = NewBuilder().withName("Any Name").withSharedSecret("Any shared secret")
	application, _ = HandleChangeRegistrationDetails(radixclient, "Any Name", *builder.BuildApplicationRegistration())
	expected = "Any shared secret"
	actual = application.SharedSecret
	assert.Equal(t, expected, actual, "HandleChangeRegistrationDetails - shared secret should be updatable")

	builder = NewBuilder().withName("Any Name").withPublicKey("Any public key")
	application, _ = HandleChangeRegistrationDetails(radixclient, "Any Name", *builder.BuildApplicationRegistration())
	expected = "Any public key"
	actual = application.PublicKey
	assert.Equal(t, expected, actual, "HandleChangeRegistrationDetails - public key should be updatable")

}

func TestGetApplication_AllFieldsAreSet(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := NewBuilder().withName("Any Name").withRepository("https://github.com/a-user/a-repo/").withSharedSecret("Any secret").withAdGroups([]string{"Some ad group"})

	HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())
	application, _ := HandleGetApplication(radixclient, "Any Name")
	assert.Equal(t, "https://github.com/a-user/a-repo/", application.Repository, "HandleGetApplication - Repository is not the same")
	assert.Equal(t, "Any secret", application.SharedSecret, "HandleGetApplication - Shared secret is not the same")
	assert.Equal(t, []string{"Some ad group"}, application.AdGroups, "HandleGetApplication - Ad groups is not the same")

}

func TestHandleTriggerPipeline_ExistingAndNonExistingApplication_JobIsCreatedForExisting(t *testing.T) {
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := fake.NewSimpleClientset()

	_, err := HandleTriggerPipeline(kubeclient, radixclient, "", "master")
	assert.Error(t, err, "HandleTriggerPipeline - Cannot run pipeline on non defined application")

	_, err = HandleTriggerPipeline(kubeclient, radixclient, "Any app", "")
	assert.Error(t, err, "HandleTriggerPipeline - Cannot run pipeline on non defined branch")

	_, err = HandleTriggerPipeline(kubeclient, radixclient, "Any app", "master")
	assert.Error(t, err, "HandleTriggerPipeline - Cannot run pipeline on non existing app")

	builder := NewBuilder().withName("Any Name").withRepository("Any repo").withSharedSecret("Any secret").withAdGroups([]string{"Some ad group"})
	HandleRegisterApplication(radixclient, *builder.BuildApplicationRegistration())
	job, err := HandleTriggerPipeline(kubeclient, radixclient, "Any Name", "master")

	assert.NoError(t, err, "HandleTriggerPipeline - Should be able to create job on existing app")
	assert.Equal(t, "Any Name", job.AppName, "HandleTriggerPipeline - Name of app was unexpected")
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
