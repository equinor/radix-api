package alerting

import (
	operatoralert "github.com/equinor/radix-operator/pkg/apis/alert"
)

type AlertNameLister interface {
	List() []string
}

type radixOperatorAlertNames struct {
	scope operatoralert.AlertScope
}

func (lister *radixOperatorAlertNames) List() []string {
	var alertNames []string
	for alertName, alertConfig := range operatoralert.GetDefaultAlertConfigs() {
		if alertConfig.Scope == lister.scope {
			alertNames = append(alertNames, alertName)
		}
	}
	return alertNames
}
