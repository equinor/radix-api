package models

import (
	"fmt"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// BuildApplication builds an Application model.
func BuildApplication(rr *radixv1.RadixRegistration, ra *radixv1.RadixApplication, reList []radixv1.RadixEnvironment, rdList []radixv1.RadixDeployment, rjList []radixv1.RadixJob, userIsAdmin bool, dnsAliases []applicationModels.DNSAlias, appAliasBaseURL string) *applicationModels.Application {
	application := applicationModels.Application{
		Name:               rr.Name,
		Registration:       *BuildApplicationRegistration(rr),
		Jobs:               BuildJobSummaryList(rjList),
		AppAlias:           buildApplicationAlias(ra, appAliasBaseURL),
		UserIsAdmin:        userIsAdmin,
		DNSAliases:         dnsAliases,
		DNSExternalAliases: BuildDNSExternalAliases(rdList),
		UseBuildKit:        useBuildKit(ra),
		UseBuildCache:      useBuildCache(ra),
	}
	if ra != nil {
		application.Environments = BuildEnvironmentSummaryList(rr, ra, reList, rdList, rjList)
	}
	return &application
}

func useBuildKit(ra *radixv1.RadixApplication) bool {
	if ra == nil || ra.Spec.Build == nil || ra.Spec.Build.UseBuildKit == nil {
		return false
	}
	return *ra.Spec.Build.UseBuildKit
}

func useBuildCache(ra *radixv1.RadixApplication) bool {
	if ra == nil || ra.Spec.Build == nil || !useBuildKit(ra) {
		return false
	}
	return ra.Spec.Build.UseBuildCache == nil || *ra.Spec.Build.UseBuildCache
}

// buildApplicationAlias builds an ApplicationAlias model for the first Ingress
func buildApplicationAlias(ra *radixv1.RadixApplication, appAliasBaseURL string) *applicationModels.ApplicationAlias {
	if ra == nil || (ra.Spec.DNSAppAlias.Component == "" && ra.Spec.DNSAppAlias.Environment == "") {
		return nil
	}

	return &applicationModels.ApplicationAlias{
		EnvironmentName: ra.Spec.DNSAppAlias.Environment,
		ComponentName:   ra.Spec.DNSAppAlias.Component,
		URL:             fmt.Sprintf("%s.%s", ra.Name, appAliasBaseURL),
	}
}
