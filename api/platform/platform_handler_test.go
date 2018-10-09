package platform

import (
	"testing"

	"github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
)

func TestGetCloneURLRepo_ValidRepo_CreatesValidClone(t *testing.T) {
	expected := "git@github.com:Equinor/my-app.git"
	actual, _ := getCloneURLFromRepo("https://github.com/Equinor/my-app")

	assert.Equal(t, actual, expected, "getCloneURLFromRepo - not equal")
}

func TestFilterOnSSHRepo_WhenSet_Filter(t *testing.T) {
	builder := NewBuilder()
	rr, _ := builder.withRepository("https://github.com/Equinor/my-app").BuildRR()

	expected := false
	actual := filterOnSSHRepo(rr, "git@github.com:Equinor/my-app.git")
	assert.Equal(t, actual, expected, "filterOnSSHRepo - expected to not be filtered")

	expected = true
	actual = filterOnSSHRepo(rr, "git@github.com:Equinor/my-app2.git")
	assert.Equal(t, actual, expected, "filterOnSSHRepo - expected to be filtered")

	expected = false
	actual = filterOnSSHRepo(rr, " ")
	assert.Equal(t, actual, expected, "filterOnSSHRepo - expected to not be filtered as filter is not provided")
}

func TestCreateApplication_WhenRepoAndDeployKeyNotSet_GenerateDeployKey(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := NewBuilder()
	builder.withName("Some Name")
	registration, _ := HandleCreateRegistation(radixclient, *builder.BuildRegistration())

	expected := ""
	actual := registration.PublicKey
	assert.Equal(t, actual, expected, "HandleCreateRegistation - when repo is missing, do not generate deploy key")

	// Restart
	radixclient = fake.NewSimpleClientset()
	builder = NewBuilder()
	builder.withName("Some Name").withRepository("Some repo string")
	registration, _ = HandleCreateRegistation(radixclient, *builder.BuildRegistration())

	assert.NotEmpty(t, registration.PublicKey, "HandleCreateRegistation - when repo is provided, and deploy key is not, generate deploy key")

	// Restart
	radixclient = fake.NewSimpleClientset()
	builder = NewBuilder()
	builder.withName("Some Name").withRepository("Some repo string").withPublicKey("Some public key")
	registration, _ = HandleCreateRegistation(radixclient, *builder.BuildRegistration())

	expected = "Some public key"
	actual = registration.PublicKey
	assert.Equal(t, actual, expected, "HandleCreateRegistation - when repo is provided, as well as deploy key, do not generate deploy key")
}
