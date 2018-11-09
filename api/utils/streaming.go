package utils

import (
	"regexp"
	"strings"

	"github.com/statoil/radix-api/models"
	"k8s.io/client-go/tools/cache"
)

// StreamInformers Starts and stops informers given subscription
func StreamInformers(unsubscribe chan struct{}, informers ...cache.SharedIndexInformer) {
	stop := make(chan struct{})
	go func() {
		<-unsubscribe
		close(stop)
	}()

	for _, informer := range informers {
		go informer.Run(stop)
	}
}

// FindMatchingSubscription Finds matching subscription based on resource pattern
// /applications/any-app/jobs/any-job should match /applications/{appName}/jobs/{jobName}
func FindMatchingSubscription(resource string, availableSubscriptions map[string]*models.Subscription) *models.Subscription {
	keys := make([]string, 0, len(availableSubscriptions))
	for k := range availableSubscriptions {
		keys = append(keys, k)
	}

	matchedResource := findMatchingResource(resource, keys)
	if matchedResource == nil {
		return nil
	}

	return availableSubscriptions[*matchedResource]
}

// GetResourceIdentifiers Will get the identifiers in a slice
// /applications/any-app/jobs/any-job should return {"any-app", "any-job"}
func GetResourceIdentifiers(resourcePattern, resource string) []string {
	identifiers := make([]string, 0)

	resourcePatternElements := strings.Split(resourcePattern, "/")
	resourceElements := strings.Split(resource, "/")

	for index, resourcePatterElement := range resourcePatternElements {
		if isResourceIdentifier(resourcePatterElement) {
			identifiers = append(identifiers, resourceElements[index])
		}
	}

	return identifiers
}

// Will locate the the resource in list of available subscriptions
func findMatchingResource(resource string, resourcePatterns []string) *string {
	resourceElements := strings.Split(resource, "/")
	for _, resourcePattern := range resourcePatterns {
		resourcePatternElements := strings.Split(resourcePattern, "/")

		if len(resourceElements) != len(resourcePatternElements) {
			continue
		}

		matchPattern := true
		for index, resourcePatterElement := range resourcePatternElements {
			if isResourceIdentifier(resourcePatterElement) {
				continue
			}

			if index > len(resourceElements) {
				matchPattern = false
				break
			}

			if !strings.EqualFold(resourceElements[index], resourcePatterElement) {
				matchPattern = false
				break
			}
		}

		if matchPattern {
			return &resourcePattern
		}
	}

	return nil
}

// In /applications/{appName}/jobs/{jobName}, appName and jobName are resource identifiers
// while applications and jobs are resources
func isResourceIdentifier(resourcePatternElement string) bool {
	re := regexp.MustCompile("\\{([^}]+)\\}")
	return re.MatchString(resourcePatternElement)
}
