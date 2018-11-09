package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindMatchingResource(t *testing.T) {
	resourcePatterns := []string{
		"/applications",
		"/applications/{appName}/jobs",
		"/applications/{appName}/jobs/{jobName}",
		"/applications/{appName}/environments/{envName}/pods",
	}

	expected := "/applications"
	actual := findMatchingResource("/applications", resourcePatterns)
	assert.Equal(t, expected, *actual)

	expected = "/applications/{appName}/jobs"
	actual = findMatchingResource("/applications/any-app/jobs", resourcePatterns)
	assert.Equal(t, expected, *actual)

	expected = "/applications/{appName}/jobs/{jobName}"
	actual = findMatchingResource("/applications/any-app/jobs/any-job", resourcePatterns)
	assert.Equal(t, expected, *actual)

	expected = "/applications/{appName}/environments/{envName}/pods"
	actual = findMatchingResource("/applications/any-app/environments/prod/pods", resourcePatterns)
	assert.Equal(t, expected, *actual)

}

func TestGetResourceIdentifiers(t *testing.T) {
	actual := GetResourceIdentifiers("/applications", "/applications")
	expected := []string{}
	assert.Equal(t, expected, actual)

	actual = GetResourceIdentifiers("/applications/{appName}", "/applications/any-app")
	expected = []string{"any-app"}
	assert.Equal(t, expected, actual)

	actual = GetResourceIdentifiers("/applications/{appName}/jobs", "/applications/any-app/jobs")
	expected = []string{"any-app"}
	assert.Equal(t, expected, actual)

	actual = GetResourceIdentifiers("/applications/{appName}/jobs/{jobName}", "/applications/any-app/jobs/any-job")
	expected = []string{"any-app", "any-job"}
	assert.Equal(t, expected, actual)

	actual = GetResourceIdentifiers("/applications/{appName}/environments/{envName}/pods", "/applications/any-app/environments/prod/pods")
	expected = []string{"any-app", "prod"}
	assert.Equal(t, expected, actual)
}

func TestIsResourceIdentifier(t *testing.T) {
	actual := isResourceIdentifier("applications")
	expected := false
	assert.Equal(t, expected, actual)

	actual = isResourceIdentifier("{appName}")
	expected = true
	assert.Equal(t, expected, actual)

	actual = isResourceIdentifier("jobs")
	expected = false
	assert.Equal(t, expected, actual)

	actual = isResourceIdentifier("{jobName}")
	expected = true
	assert.Equal(t, expected, actual)
}
