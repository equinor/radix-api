package models

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

//ConvertResourceRequirements Convert resource requirements
func ConvertResourceRequirements(resources corev1.ResourceRequirements) ResourceRequirements {
	return ResourceRequirements{
		Limits:   getResources(resources.Limits),
		Requests: getResources(resources.Requests),
	}
}

func getResources(resources corev1.ResourceList) Resources {
	resourceList := Resources{
		CPU:    getResource(resources.Cpu()),
		Memory: getResource(resources.Memory()),
	}
	return resourceList
}

func getResource(resource *resource.Quantity) string {
	if resource == nil {
		return ""
	}
	return resource.String()
}
