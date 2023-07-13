package models

import (
	"fmt"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// ComponentDeploymentSummaryBuilder Builds DTOs
type ComponentDeploymentSummaryBuilder interface {
	WithRadixDeployment(*v1.RadixDeployment) ComponentDeploymentSummaryBuilder
	WithRadixDeployComponent(component v1.RadixCommonDeployComponent) ComponentDeploymentSummaryBuilder
	Build() (*ComponentDeploymentSummary, error)
}

type componentDeploymentSummaryBuilder struct {
	component       v1.RadixCommonDeployComponent
	radixDeployment *v1.RadixDeployment
}

// NewComponentDeploymentSummaryBuilder Constructor for application ComponentDeploymentSummaryBuilder
func NewComponentDeploymentSummaryBuilder() ComponentDeploymentSummaryBuilder {
	return &componentDeploymentSummaryBuilder{}
}

// WithRadixDeployComponent With RadixDeployComponent
func (b *componentDeploymentSummaryBuilder) WithRadixDeployComponent(component v1.RadixCommonDeployComponent) ComponentDeploymentSummaryBuilder {
	b.component = component
	return b
}

func (b *componentDeploymentSummaryBuilder) WithRadixDeployment(rd *v1.RadixDeployment) ComponentDeploymentSummaryBuilder {
	b.radixDeployment = rd
	return b
}

func (b *componentDeploymentSummaryBuilder) Build() (*ComponentDeploymentSummary, error) {
	if b.component == nil || b.radixDeployment == nil {
		return nil, fmt.Errorf("component or RadixDeployment are empty")
	}
	return &ComponentDeploymentSummary{
		Name:          b.radixDeployment.GetName(),
		ComponentName: b.component.GetName(),
		ActiveFrom:    radixutils.FormatTimestamp(b.radixDeployment.Status.ActiveFrom.Time),
		ActiveTo:      radixutils.FormatTimestamp(b.radixDeployment.Status.ActiveTo.Time),
		Condition:     string(b.radixDeployment.Status.Condition),
		GitCommitHash: b.radixDeployment.GetLabels()[kube.RadixCommitLabel],
	}, nil
}
