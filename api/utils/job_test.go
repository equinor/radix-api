package utils

import (
	"testing"

	jobmodels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/stretchr/testify/assert"
)

func TestIsBefore(t *testing.T) {
	job1 := jobmodels.JobSummary{}
	job2 := jobmodels.JobSummary{}

	job1.Created = ""
	job2.Created = ""
	assert.False(t, IsBefore(&job1, &job2))

	job1.Created = "2019-08-26T12:56:48Z"
	job2.Created = ""
	assert.True(t, IsBefore(&job1, &job2))

	job1.Created = "2019-08-26T12:56:48Z"
	job2.Created = "2019-08-26T12:56:49Z"
	assert.True(t, IsBefore(&job1, &job2))

	job1.Created = "2019-08-26T12:56:48Z"
	job2.Created = "2019-08-26T12:56:48Z"
	job1.Started = "2019-08-26T12:56:51Z"
	job2.Started = "2019-08-26T12:56:52Z"
	assert.True(t, IsBefore(&job1, &job2))

	job1.Created = "2019-08-26T12:56:48Z"
	job2.Created = "2019-08-26T12:56:48Z"
	job1.Started = ""
	job2.Started = "2019-08-26T12:56:52Z"
	assert.False(t, IsBefore(&job1, &job2))

}
