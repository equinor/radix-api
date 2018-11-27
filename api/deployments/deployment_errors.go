package deployments

import (
	"fmt"

	"github.com/statoil/radix-api/api/utils"
)

// errors
func nonExistingApplication(underlyingError error, appName string) error {
	return utils.TypeMissingError(fmt.Sprintf("Unable to get application for app %s", appName), underlyingError)
}

func nonExistingFromEnvironment(underlyingError error) error {
	return utils.TypeMissingError("Non existing from environment", underlyingError)
}

func nonExistingToEnvironment(underlyingError error) error {
	return utils.TypeMissingError("Non existing to environment", underlyingError)
}

func nonExistingDeployment(underlyingError error, deploymentName string) error {
	return utils.TypeMissingError(fmt.Sprintf("Non existing deployment %s", deploymentName), underlyingError)
}

func nonExistingComponentName(appName, componentName string) error {
	return utils.TypeMissingError(fmt.Sprintf("Unable to get application component %s for app %s", componentName, appName), nil)
}

func nonExistingPod(appName, podName string) error {
	return utils.TypeMissingError(fmt.Sprintf("Unable to get pod %s for app %s", podName, appName), nil)
}
