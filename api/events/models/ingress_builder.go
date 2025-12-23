package models

import (
	networkingv1 "k8s.io/api/networking/v1"
)

// IngressStateBuilder Build Ingress
type IngressStateBuilder interface {
	// WithIngress sets the Ingress
	WithIngress(*networkingv1.Ingress) IngressStateBuilder
	// WithComponent sets the component name
	WithComponent(componentName string) IngressStateBuilder
	Build() []IngressRule
}

type ingressStateBuilder struct {
	ingress       *networkingv1.Ingress
	componentName string
}

// NewIngressBuilder Constructor for ingressBuilder
func NewIngressBuilder() IngressStateBuilder {
	return &ingressStateBuilder{}
}

func (b *ingressStateBuilder) WithIngress(v *networkingv1.Ingress) IngressStateBuilder {
	b.ingress = v
	return b
}

func (b *ingressStateBuilder) WithComponent(componentName string) IngressStateBuilder {
	b.componentName = componentName
	return b
}

func (b *ingressStateBuilder) Build() []IngressRule {
	if b.ingress == nil {
		return nil
	}

	var ingressRules []IngressRule
	for _, rule := range b.ingress.Spec.Rules {
		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				ingressRule := IngressRule{Host: rule.Host, Path: path.Path}

				if path.Backend.Service != nil {
					if len(b.componentName) == 0 {
						ingressRule.Service = path.Backend.Service.Name // provide service name only component name is not set
					}
					ingressRule.Port = path.Backend.Service.Port.Number
				}
				ingressRules = append(ingressRules, ingressRule)
			}
		}
	}

	return ingressRules
}
