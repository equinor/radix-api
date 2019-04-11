package models_test

import (
	"testing"

	models "github.com/equinor/radix-api/api/jobs/models"
	"github.com/stretchr/testify/assert"
)

func Test_StringToPipeline(t *testing.T) {
	_, err := models.GetPipelineFromName("NA")

	assert.Error(t, err)
}

func Test_StringToPipelineToString(t *testing.T) {
	p, _ := models.GetPipelineFromName("build-deploy")

	assert.Equal(t, "build-deploy", p.String())

	p, _ = models.GetPipelineFromName("build")

	assert.Equal(t, "build", p.String())
}
