package models

import (
	applicationModels "github.com/equinor/radix-api/api/applications/models"
	"github.com/equinor/radix-api/api/utils/predicate"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	networkingv1 "k8s.io/api/networking/v1"
)

// BuildApplicationAlias builds an ApplicationAlias model for the first Ingress
func BuildApplicationAlias(ingressList []networkingv1.Ingress, reList []radixv1.RadixEnvironment) *applicationModels.ApplicationAlias {
	appAliasIngressList := slice.FindAll(ingressList, predicate.IsAppAliasIngress)
	namespaceReMap := slice.Reduce(reList, map[string]radixv1.RadixEnvironment{}, func(acc map[string]radixv1.RadixEnvironment, re radixv1.RadixEnvironment) map[string]radixv1.RadixEnvironment {
		acc[operatorUtils.GetEnvironmentNamespace(re.Spec.AppName, re.Spec.EnvName)] = re
		return acc
	})

	for _, ingress := range appAliasIngressList {
		if re, ok := namespaceReMap[ingress.Namespace]; ok {
			return &applicationModels.ApplicationAlias{
				EnvironmentName: re.Spec.EnvName,
				ComponentName:   ingress.Labels[kube.RadixComponentLabel],
				URL:             ingress.Spec.Rules[0].Host,
			}
		}
	}

	return nil
}
