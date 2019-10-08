package models

import (
	"fmt"

	"github.com/equinor/radix-api/api/utils"
)

// NonExistingApplication No application found by name
func NonExistingApplication(underlyingError error, appName string) error {
	return utils.TypeMissingError(fmt.Sprintf("Unable to get application for app %s", appName), underlyingError)
}

// IllegalEmptyEnvironment From environment does not exist
func IllegalEmptyEnvironment() error {
	return utils.ValidationError("Radix Deployment", "Environment cannot be empty")
}

// NonExistingFromEnvironment From environment does not exist
func NonExistingFromEnvironment(underlyingError error) error {
	return utils.TypeMissingError("Non existing from environment", underlyingError)
}

// NonExistingToEnvironment To environment does not exist
func NonExistingToEnvironment(underlyingError error) error {
	return utils.TypeMissingError("Non existing to environment", underlyingError)
}

// NoActiveDeploymentFoundInEnvironment Deployment wasn't found
func NoActiveDeploymentFoundInEnvironment(appName, envName string) error {
	return utils.TypeMissingError(fmt.Sprintf("Non active deployment for %s was found in %s", appName, envName), nil)
}

// NonExistingDeployment Deployment wasn't found
func NonExistingDeployment(underlyingError error, deploymentName string) error {
	return utils.TypeMissingError(fmt.Sprintf("Non existing deployment %s", deploymentName), underlyingError)
}

// NonExistingComponentName Component by name was not found
func NonExistingComponentName(appName, componentName string) error {
	return utils.TypeMissingError(fmt.Sprintf("Unable to get application component %s for app %s", componentName, appName), nil)
}

// NonExistingPod Pod by name was not found
func NonExistingPod(appName, podName string) error {
	return utils.TypeMissingError(fmt.Sprintf("Unable to get pod %s for app %s", podName, appName), nil)
}
