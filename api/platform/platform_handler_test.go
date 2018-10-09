package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCloneURLRepo(t *testing.T) {
	expected := "git@github.com:Equinor/my-app.git"
	actual, _ := getCloneURLFromRepo("https://github.com/Equinor/my-app")

	assert.Equal(t, actual, expected, "getCloneURLFromRepo - not equal")
}

func TestFilterOnSSHRepo(t *testing.T) {
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
