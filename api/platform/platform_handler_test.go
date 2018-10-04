package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetProjectNameFromRepo(t *testing.T) {
	expected := "my-app"
	actual, _ := getProjectNameFromRepo("https://github.com/Equinor/my-app")

	assert.Equal(t, actual, expected, "getProjectNameFromRepo - not equal")
}

func TestGetCloneURLRepo(t *testing.T) {
	expected := "git@github.com:Equinor/my-app.git"
	actual, _ := getCloneURLFromRepo("https://github.com/Equinor/my-app")

	assert.Equal(t, actual, expected, "getCloneURLFromRepo - not equal")
}

func TestFilterOnSshRepo(t *testing.T) {
	builder := NewBuilder()
	rr, _ := builder.withRepository("https://github.com/Equinor/my-app").BuildRR()

	expected := false
	actual := filterOnSshRepo(rr, "git@github.com:Equinor/my-app.git")
	assert.Equal(t, actual, expected, "filterOnSshRepo - expected to not be filtered")

	expected = true
	actual = filterOnSshRepo(rr, "git@github.com:Equinor/my-app2.git")
	assert.Equal(t, actual, expected, "filterOnSshRepo - expected to be filtered")
}
