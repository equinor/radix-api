package models

import (
	"sort"

	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// BuildJobSummaryList builds a list of JobSummary models.
func BuildJobSummaryList(rjList []radixv1.RadixJob) []*jobModels.JobSummary {
	jobs := slice.Map(rjList, func(rj radixv1.RadixJob) *jobModels.JobSummary { return BuildJobSummary(rj) })
	sort.Slice(jobs, func(i, j int) bool {
		return utils.IsBefore(jobs[j], jobs[i])
	})
	return jobs
}

// BuildJobSummary builds a JobSummary model.
func BuildJobSummary(rj radixv1.RadixJob) *jobModels.JobSummary {
	return jobModels.GetSummaryFromRadixJob(&rj)
}
