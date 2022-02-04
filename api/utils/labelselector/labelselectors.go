package labelselector

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	kubeLabels "k8s.io/apimachinery/pkg/labels"
)

// ForAuxiliaryResource returns a label set to be used as LabelSelector for auxiliary resource queries
func ForAuxiliaryResource(appName, componentName, auxType string) kubeLabels.Set {
	appSet := ForApplication(appName)
	appSet[kube.RadixAuxiliaryComponentLabel] = componentName
	appSet[kube.RadixAuxiliaryComponentTypeLabel] = auxType
	return appSet
}

func ForApplication(appName string) kubeLabels.Set {
	return kubeLabels.Set{
		kube.RadixAppLabel: appName,
	}
}

func ForIsAppAlias() kubeLabels.Set {
	return kubeLabels.Set{
		kube.RadixAppAliasLabel: "true",
	}
}
