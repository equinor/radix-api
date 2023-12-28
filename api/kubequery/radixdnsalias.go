package kubequery

import (
	"context"
	"fmt"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetDNSAliases returns all RadixDNSAliases for the specified application.
func GetDNSAliases(ctx context.Context, client radixclient.Interface, radixApplication *radixv1.RadixApplication, dnsZone string) []applicationModels.DNSAlias {
	if radixApplication == nil {
		return nil
	}
	return slice.Reduce(radixApplication.Spec.DNSAlias, []applicationModels.DNSAlias{}, func(acc []applicationModels.DNSAlias, dnsAlias radixv1.DNSAlias) []applicationModels.DNSAlias {
		radixDNSAlias, err := client.RadixV1().RadixDNSAliases().Get(ctx, dnsAlias.Alias, metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) && !errors.IsForbidden(err) {
				log.Errorf("failed to get DNS alias %s: %v", dnsAlias.Alias, err)
			}
			return acc
		}
		aliasModel := applicationModels.DNSAlias{
			URL:             fmt.Sprintf("%s.%s", dnsAlias.Alias, dnsZone),
			ComponentName:   dnsAlias.Component,
			EnvironmentName: dnsAlias.Environment,
			Status: applicationModels.DNSAliasStatus{
				Condition: string(radixDNSAlias.Status.Condition),
				Message:   radixDNSAlias.Status.Message,
			},
		}
		return append(acc, aliasModel)
	})
}
