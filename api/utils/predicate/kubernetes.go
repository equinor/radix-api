package predicate

import (
	"github.com/equinor/radix-api/api/utils/labelselector"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func IsAppAliasIngress(ingress networkingv1.Ingress) bool {
	return labelselector.ForIsAppAlias().AsSelector().Matches(labels.Set(ingress.Labels))
}
