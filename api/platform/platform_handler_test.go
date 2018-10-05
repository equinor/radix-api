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
