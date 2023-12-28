package models

import (
	applicationModels "github.com/equinor/radix-api/api/applications/models"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

// BuildApplication builds an Application model.
func BuildApplication(rr *radixv1.RadixRegistration, ra *radixv1.RadixApplication, reList []radixv1.RadixEnvironment, rdList []radixv1.RadixDeployment, rjList []radixv1.RadixJob, ingressList []networkingv1.Ingress, userIsAdmin bool, dnsAliases []applicationModels.DNSAlias, radixDNSZone string) *applicationModels.Application {
	application := applicationModels.Application{
		Name:         rr.Name,
		Registration: *BuildApplicationRegistration(rr),
		Jobs:         BuildJobSummaryList(rjList),
		AppAlias:     BuildApplicationAlias(ingressList, reList),
		UserIsAdmin:  userIsAdmin,
		DNSAliases:   dnsAliases,
	}
	if ra != nil {
		application.Environments = BuildEnvironmentSummaryList(rr, ra, reList, rdList, rjList)
	}
	return &application
}
