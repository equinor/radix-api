package models

import (
	applicationModels "github.com/equinor/radix-api/api/applications/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

// BuildApplication builds an Application model.
func BuildApplication(rr *radixv1.RadixRegistration, ra *radixv1.RadixApplication, reList []radixv1.RadixEnvironment, rdList []radixv1.RadixDeployment, rjList []radixv1.RadixJob, ingressList []networkingv1.Ingress) *applicationModels.Application {
	var environments []*environmentModels.EnvironmentSummary
	if ra != nil {
		environments = BuildEnvironmentSummaryList(rr, ra, reList, rdList, rjList)
	}
	registration := BuildApplicationRegistration(rr)
	jobs := BuildJobSummaryList(rjList)
	appAlias := BuildApplicationAlias(ingressList, reList)

	return &applicationModels.Application{
		Name:         rr.Name,
		Registration: *registration,
		Jobs:         jobs,
		Environments: environments,
		AppAlias:     appAlias,
	}
}
