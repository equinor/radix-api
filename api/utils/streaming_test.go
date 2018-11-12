package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindMatchingResource(t *testing.T) {
	resourcePatterns := []string{
		"/api/v1/applications",
		"/api/v1/applications/{appName}/jobs",
		"/api/v1/applications/{appName}/jobs/{jobName}",
		"/api/v1/applications/{appName}/environments/{envName}/pods",
	}

	expected := "/api/v1/applications"
	actual := findMatchingResource("/api/v1/applications", resourcePatterns)
	assert.Equal(t, expected, *actual)

	expected = "/api/v1/applications/{appName}/jobs"
	actual = findMatchingResource("/api/v1/applications/any-app/jobs", resourcePatterns)
	assert.Equal(t, expected, *actual)

	expected = "/api/v1/applications/{appName}/jobs/{jobName}"
	actual = findMatchingResource("/api/v1/applications/any-app/jobs/any-job", resourcePatterns)
	assert.Equal(t, expected, *actual)

	expected = "/api/v1/applications/{appName}/environments/{envName}/pods"
	actual = findMatchingResource("/api/v1/applications/any-app/environments/prod/pods", resourcePatterns)
	assert.Equal(t, expected, *actual)

}

func TestGetResourceIdentifiers(t *testing.T) {
	actual := GetResourceIdentifiers("/api/v1/applications", "/api/v1/applications")
	expected := []string{}
	assert.Equal(t, expected, actual)

	actual = GetResourceIdentifiers("/api/v1/applications/{appName}", "/api/v1/applications/any-app")
	expected = []string{"any-app"}
	assert.Equal(t, expected, actual)

	actual = GetResourceIdentifiers("/api/v1/applications/{appName}/jobs", "/api/v1/applications/any-app/jobs")
	expected = []string{"any-app"}
	assert.Equal(t, expected, actual)

	actual = GetResourceIdentifiers("/api/v1/applications/{appName}/jobs/{jobName}", "/api/v1/applications/any-app/jobs/any-job")
	expected = []string{"any-app", "any-job"}
	assert.Equal(t, expected, actual)

	actual = GetResourceIdentifiers("/api/v1/applications/{appName}/environments/{envName}/pods", "/api/v1/applications/any-app/environments/prod/pods")
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
