package platform

import (
	"testing"

	"github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

func TestGetRegistations_WithFilterOnSSHRepo_Filter(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	anyApp := NewBuilder().withName("my-app").withRepository("https://github.com/Equinor/my-app").BuildRegistration()
	HandleCreateRegistation(radixclient, *anyApp)

	registrations, _ := HandleGetRegistations(radixclient, "git@github.com:Equinor/my-app.git")
	expected := 1
	actual := len(registrations)
	assert.Equal(t, expected, actual, "GetRegistations - expected to be listed")

	registrations, _ = HandleGetRegistations(radixclient, "git@github.com:Equinor/my-app2.git")
	expected = 0
	actual = len(registrations)
	assert.Equal(t, expected, actual, "GetRegistations - expected not to be listed")

	registrations, _ = HandleGetRegistations(radixclient, " ")
	expected = 1
	actual = len(registrations)
	assert.Equal(t, expected, actual, "GetRegistations - expected to be listed when no filter is provided")
}

func TestCreateApplication_NoName_ValidationError(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := NewBuilder().withName("")
	_, err := HandleCreateRegistation(radixclient, *builder.BuildRegistration())
	assert.Error(t, err, "HandleCreateRegistation - Cannot create application without name")
}

func TestCreateApplication_WhenRepoAndDeployKeyNotSet_GenerateDeployKey(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := NewBuilder().withName("Any Name")
	registration, _ := HandleCreateRegistation(radixclient, *builder.BuildRegistration())

	expected := ""
	actual := registration.PublicKey
	assert.Equal(t, expected, actual, "HandleCreateRegistation - when repo is missing, do not generate deploy key")

	// Restart
	radixclient = fake.NewSimpleClientset()
	builder = NewBuilder().withName("Any Name").withRepository("Any repo")
	registration, _ = HandleCreateRegistation(radixclient, *builder.BuildRegistration())

	assert.NotEmpty(t, registration.PublicKey, "HandleCreateRegistation - when repo is provided, and deploy key is not, generate deploy key")

	// Restart
	radixclient = fake.NewSimpleClientset()
	builder = NewBuilder().withName("Any Name").withRepository("Any repo").withPublicKey("Any public key")
	registration, _ = HandleCreateRegistation(radixclient, *builder.BuildRegistration())

	expected = "Any public key"
	actual = registration.PublicKey
	assert.Equal(t, expected, actual, "HandleCreateRegistation - when repo is provided, as well as deploy key, do not generate deploy key")
}

func TestCreateApplication_DuplicateRepo_ShouldFailAsWeCannotHandleThatSituation(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := NewBuilder().withName("Any Name").withRepository("Any repo")
	HandleCreateRegistation(radixclient, *builder.BuildRegistration())

	builder = NewBuilder().withName("Another Name").withRepository("Any repo")
	_, err := HandleCreateRegistation(radixclient, *builder.BuildRegistration())
	assert.Error(t, err, "HandleCreateRegistation - Should not be able to create another application with the same repo")
}

func TestUpdateApplication_DuplicateRepo_ShouldFailAsWeCannotHandleThatSituation(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := NewBuilder().withName("Any Name").withRepository("Any repo")
	HandleCreateRegistation(radixclient, *builder.BuildRegistration())

	builder = NewBuilder().withName("Another Name").withRepository("Another repo")
	HandleCreateRegistation(radixclient, *builder.BuildRegistration())

	builder = NewBuilder().withName("Another Name").withRepository("Any repo")
	_, err := HandleUpdateRegistation(radixclient, "Another Name", *builder.BuildRegistration())
	assert.Error(t, err, "HandleUpdateRegistation - Should not be able to update application with the same repo")
}

func TestUpdateApplication_MismatchingNameOrNotExists_ShouldFailAsIllegalOperation(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := NewBuilder().withName("Any Name")
	HandleCreateRegistation(radixclient, *builder.BuildRegistration())

	builder = NewBuilder().withName("Any Name")
	_, err := HandleUpdateRegistation(radixclient, "Another Name", *builder.BuildRegistration())
	assert.Error(t, err, "HandleUpdateRegistation - Should not be able to call update application with different name in parameter and body")

	builder = NewBuilder().withName("Another Name")
	_, err = HandleUpdateRegistation(radixclient, "Another Name", *builder.BuildRegistration())
	assert.Error(t, err, "HandleUpdateRegistation - Should not be able to call update application on a non existing application")

}

func TestUpdateApplication_AbleToSetAnySpecField(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := NewBuilder().withName("Any Name")
	HandleCreateRegistation(radixclient, *builder.BuildRegistration())

	builder = NewBuilder().withName("Any Name").withRepository("Any repo")
	registration, _ := HandleUpdateRegistation(radixclient, "Any Name", *builder.BuildRegistration())
	expected := "Any repo"
	actual := registration.Repository
	assert.Equal(t, expected, actual, "HandleUpdateRegistation - repository should be updatable")

	builder = NewBuilder().withName("Any Name").withSharedSecret("Any shared secret")
	registration, _ = HandleUpdateRegistation(radixclient, "Any Name", *builder.BuildRegistration())
	expected = "Any shared secret"
	actual = registration.SharedSecret
	assert.Equal(t, expected, actual, "HandleUpdateRegistation - shared secret should be updatable")

	builder = NewBuilder().withName("Any Name").withPublicKey("Any public key")
	registration, _ = HandleUpdateRegistation(radixclient, "Any Name", *builder.BuildRegistration())
	expected = "Any public key"
	actual = registration.PublicKey
	assert.Equal(t, expected, actual, "HandleUpdateRegistation - public key should be updatable")

}

func TestGetApplication_AllFieldsAreSet(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := NewBuilder().withName("Any Name").withRepository("https://github.com/a-user/a-repo/").withSharedSecret("Any secret").withAdGroups([]string{"Some ad group"})

	HandleCreateRegistation(radixclient, *builder.BuildRegistration())
	registration, _ := HandleGetRegistation(radixclient, "Any Name")
	assert.Equal(t, "https://github.com/a-user/a-repo/", registration.Repository, "HandleGetRegistation - Repository is not the same")
	assert.Equal(t, "Any secret", registration.SharedSecret, "HandleGetRegistation - Shared secret is not the same")
	assert.Equal(t, []string{"Some ad group"}, registration.AdGroups, "HandleGetRegistation - Ad groups is not the same")

}

func TestHandleCreateApplicationPipelineJob_ExistingAndNonExistingRegistration_JobIsCreatedForExisting(t *testing.T) {
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := fake.NewSimpleClientset()

	_, err := HandleCreateApplicationPipelineJob(kubeclient, radixclient, "", "master")
	assert.Error(t, err, "HandleCreateApplicationPipelineJob - Cannot run pipeline on non defined application")

	_, err = HandleCreateApplicationPipelineJob(kubeclient, radixclient, "Any app", "")
	assert.Error(t, err, "HandleCreateApplicationPipelineJob - Cannot run pipeline on non defined branch")

	_, err = HandleCreateApplicationPipelineJob(kubeclient, radixclient, "Any app", "master")
	assert.Error(t, err, "HandleCreateApplicationPipelineJob - Cannot run pipeline on non existing app")

	builder := NewBuilder().withName("Any Name").withRepository("Any repo").withSharedSecret("Any secret").withAdGroups([]string{"Some ad group"})
	HandleCreateRegistation(radixclient, *builder.BuildRegistration())
	job, err := HandleCreateApplicationPipelineJob(kubeclient, radixclient, "Any Name", "master")

	assert.NoError(t, err, "HandleCreateApplicationPipelineJob - Should be able to create job on existing app")
	assert.Equal(t, "Any Name", job.AppName, "HandleCreateApplicationPipelineJob - Name of app was unexpected")
	assert.Equal(t, "master", job.Branch, "HandleCreateApplicationPipelineJob - Branch was unexpected")
	assert.NotEmpty(t, job.Name, "HandleCreateApplicationPipelineJob - Expected a jobname")
	assert.NotEmpty(t, job.SSHRepo, "HandleCreateApplicationPipelineJob - Expected a repo")
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
