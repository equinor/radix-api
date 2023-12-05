package models

import (
	"fmt"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// BuildDNSAlias builds an DNSAlias model for the application
func BuildDNSAlias(ra *radixv1.RadixApplication, radixDNSAliasList []radixv1.RadixDNSAlias, radixDNSZone string) []applicationModels.DNSAlias {
	var dnsAliases []applicationModels.DNSAlias
	radixDNSAliasMap := slice.Reduce(radixDNSAliasList, make(map[string]*radixv1.RadixDNSAlias), func(acc map[string]*radixv1.RadixDNSAlias, radixDNSAlias radixv1.RadixDNSAlias) map[string]*radixv1.RadixDNSAlias {
		acc[radixDNSAlias.GetName()] = &radixDNSAlias
		return acc
	})
	for _, dnsAlias := range ra.Spec.DNSAlias {
		aliasModel := applicationModels.DNSAlias{
			URL:             fmt.Sprintf("%s.%s", dnsAlias.Alias, radixDNSZone),
			ComponentName:   dnsAlias.Component,
			EnvironmentName: dnsAlias.Environment,
		}
		if radixDNSAlias, ok := radixDNSAliasMap[dnsAlias.Alias]; ok {
			aliasModel.Status = applicationModels.DNSAliasStatus{
				Condition: string(radixDNSAlias.Status.Condition),
				Message:   radixDNSAlias.Status.Message,
			}
		}
		dnsAliases = append(dnsAliases, aliasModel)
	}
	return dnsAliases
}
