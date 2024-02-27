package models

import (
	"github.com/equinor/radix-api/api/applications/models"
	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// BuildDNSExternalAliases builds DNSExternalAliases model list
func BuildDNSExternalAliases(ra *radixv1.RadixApplication) []models.DNSExternalAlias {
	return slice.Reduce(ra.Spec.DNSExternalAlias, make([]models.DNSExternalAlias, 0), func(acc []models.DNSExternalAlias, externalAlias radixv1.ExternalAlias) []models.DNSExternalAlias {
		return append(acc, models.DNSExternalAlias{
			URL:             externalAlias.Alias,
			ComponentName:   externalAlias.Component,
			EnvironmentName: externalAlias.Environment,
		})
	})
}
