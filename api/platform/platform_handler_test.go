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

func TestGetRegistations_WithFilterOnSSHRepo_Filter(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	anyApp := NewBuilder().withName("my-app").withRepository("https://github.com/Equinor/my-app").BuildRegistration()
	HandleCreateRegistation(radixclient, *anyApp)

	registrations, _ := HandleGetRegistations(radixclient, "git@github.com:Equinor/my-app.git")
	expected := 1
	actual := len(registrations)
	assert.Equal(t, actual, expected, "GetRegistations - expected to be listed")

	registrations, _ = HandleGetRegistations(radixclient, "git@github.com:Equinor/my-app2.git")
	expected = 0
	actual = len(registrations)
	assert.Equal(t, actual, expected, "GetRegistations - expected not to be listed")

	registrations, _ = HandleGetRegistations(radixclient, " ")
	expected = 1
	actual = len(registrations)
	assert.Equal(t, actual, expected, "GetRegistations - expected to be listed when no filter is provided")
}

func TestCreateApplication_WhenRepoAndDeployKeyNotSet_GenerateDeployKey(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := NewBuilder().withName("Any Name")
	registration, _ := HandleCreateRegistation(radixclient, *builder.BuildRegistration())

	expected := ""
	actual := registration.PublicKey
	assert.Equal(t, actual, expected, "HandleCreateRegistation - when repo is missing, do not generate deploy key")

	// Restart
	radixclient = fake.NewSimpleClientset()
	builder = NewBuilder().withName("Any Name").withRepository("Any repo string")
	registration, _ = HandleCreateRegistation(radixclient, *builder.BuildRegistration())

	assert.NotEmpty(t, registration.PublicKey, "HandleCreateRegistation - when repo is provided, and deploy key is not, generate deploy key")

	// Restart
	radixclient = fake.NewSimpleClientset()
	builder = NewBuilder().withName("Any Name").withRepository("Any repo string").withPublicKey("Any public key")
	registration, _ = HandleCreateRegistation(radixclient, *builder.BuildRegistration())

	expected = "Any public key"
	actual = registration.PublicKey
	assert.Equal(t, actual, expected, "HandleCreateRegistation - when repo is provided, as well as deploy key, do not generate deploy key")
}

func TestUpdateApplication_AbleToSetAnySpecField(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	builder := NewBuilder().withName("Any Name")
	HandleCreateRegistation(radixclient, *builder.BuildRegistration())

	builder = NewBuilder().withName("Any Name").withRepository("Any repo string")
	registration, _ := HandleUpdateRegistation(radixclient, "Any Name", *builder.BuildRegistration())
	expected := "Any repo string"
	actual := registration.Repository
	assert.Equal(t, actual, expected, "HandleUpdateRegistation - repository should be updatable")

	builder = NewBuilder().withName("Any Name").withSharedSecret("Any shared secret")
	registration, _ = HandleUpdateRegistation(radixclient, "Any Name", *builder.BuildRegistration())
	expected = "Any shared secret"
	actual = registration.SharedSecret
	assert.Equal(t, actual, expected, "HandleUpdateRegistation - shared secret should be updatable")

	builder = NewBuilder().withName("Any Name").withPublicKey("Any public key")
	registration, _ = HandleUpdateRegistation(radixclient, "Any Name", *builder.BuildRegistration())
	expected = "Any public key"
	actual = registration.PublicKey
	assert.Equal(t, actual, expected, "HandleUpdateRegistation - public key should be updatable")

}
