package applications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/equinor/radix-api/models"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	jobModels "github.com/equinor/radix-api/api/jobs/models"
	controllertest "github.com/equinor/radix-api/api/test"
	"github.com/equinor/radix-api/api/utils"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	jobPipeline "github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretsstorevclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	clusterName     = "AnyClusterName"
	dnsZone         = "dev.radix.equinor.com"
	appAliasDNSZone = "app.dev.radix.equinor.com"
	egressIps       = "0.0.0.0"
)

func setupTest(requireAppConfigurationItem, requireAppADGroups bool) (*commontest.Utils, *controllertest.Utils, *kubefake.Clientset, *fake.Clientset, prometheusclient.Interface, secretsstorevclient.Interface) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := fake.NewSimpleClientset()
	prometheusclient := prometheusfake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient, secretproviderclient)
	commonTestUtils.CreateClusterPrerequisites(clusterName, egressIps)
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, clusterName)

	// controllerTestUtils is used for issuing HTTP request and processing responses
	controllerTestUtils := controllertest.NewTestUtils(
		kubeclient,
		radixclient,
		secretproviderclient,
		NewApplicationController(
			func(_ context.Context, _ kubernetes.Interface, _ v1.RadixRegistration) (bool, error) {
				return true, nil
			},
			newTestApplicationHandlerFactory(
				ApplicationHandlerConfig{RequireAppConfigurationItem: requireAppConfigurationItem, RequireAppADGroups: requireAppADGroups},
				func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
					return true, nil
				},
			),
		),
	)

	return &commonTestUtils, &controllerTestUtils, kubeclient, radixclient, prometheusclient, secretproviderclient
}

func TestGetApplications_HasAccessToSomeRR(t *testing.T) {
	commonTestUtils, _, kubeclient, radixclient, _, secretproviderclient := setupTest(true, true)

	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithCloneURL("git@github.com:Equinor/my-app.git"))
	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithCloneURL("git@github.com:Equinor/my-second-app.git").WithAdGroups([]string{"2"}).WithName("my-second-app"))

	t.Run("no access", func(t *testing.T) {
		controllerTestUtils := controllertest.NewTestUtils(
			kubeclient,
			radixclient,
			secretproviderclient,
			NewApplicationController(
				func(_ context.Context, _ kubernetes.Interface, _ v1.RadixRegistration) (bool, error) {
					return false, nil
				}, newTestApplicationHandlerFactory(ApplicationHandlerConfig{RequireAppConfigurationItem: true, RequireAppADGroups: true},
					func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
						return true, nil
					})))
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications")
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		controllertest.GetResponseBody(response, &applications)
		assert.Equal(t, 0, len(applications))
	})

	t.Run("access to single app", func(t *testing.T) {
		controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, secretproviderclient, NewApplicationController(
			func(_ context.Context, _ kubernetes.Interface, rr v1.RadixRegistration) (bool, error) {
				return rr.GetName() == "my-second-app", nil
			}, newTestApplicationHandlerFactory(ApplicationHandlerConfig{RequireAppConfigurationItem: true, RequireAppADGroups: true},
				func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
					return true, nil
				})))
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications")
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		controllertest.GetResponseBody(response, &applications)
		assert.Equal(t, 1, len(applications))
	})

	t.Run("access to all app", func(t *testing.T) {
		controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, secretproviderclient, NewApplicationController(
			func(_ context.Context, _ kubernetes.Interface, _ v1.RadixRegistration) (bool, error) {
				return true, nil
			}, newTestApplicationHandlerFactory(ApplicationHandlerConfig{RequireAppConfigurationItem: true, RequireAppADGroups: true},
				func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
					return true, nil
				})))
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications")
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		controllertest.GetResponseBody(response, &applications)
		assert.Equal(t, 2, len(applications))
	})
}

func TestGetApplications_WithFilterOnSSHRepo_Filter(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _, _ := setupTest(true, true)
	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithCloneURL("git@github.com:Equinor/my-app.git"))

	// Test
	t.Run("matching repo", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications?sshRepo=%s", url.QueryEscape("git@github.com:Equinor/my-app.git")))
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		controllertest.GetResponseBody(response, &applications)
		assert.Equal(t, 1, len(applications))
	})

	t.Run("unmatching repo", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications?sshRepo=%s", url.QueryEscape("git@github.com:Equinor/my-app2.git")))
		response := <-responseChannel

		applications := make([]*applicationModels.ApplicationSummary, 0)
		controllertest.GetResponseBody(response, &applications)
		assert.Equal(t, 0, len(applications))
	})

	t.Run("no filter", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications")
		response := <-responseChannel

		applications := make([]*applicationModels.ApplicationSummary, 0)
		controllertest.GetResponseBody(response, &applications)
		assert.Equal(t, 1, len(applications))
	})
}

func TestSearchApplications(t *testing.T) {
	// Setup
	commonTestUtils, _, kubeclient, radixclient, _, secretproviderclient := setupTest(true, true)
	appNames := []string{"app-1", "app-2"}

	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().WithName(appNames[0]))
	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().WithName(appNames[1]))
	commonTestUtils.ApplyDeployment(
		builders.
			ARadixDeployment().
			WithAppName(appNames[1]).
			WithComponent(
				builders.
					NewDeployComponentBuilder(),
			),
	)

	app2Job1Started, _ := radixutils.ParseTimestamp("2018-11-12T12:30:14Z")
	createRadixJob(commonTestUtils, appNames[1], "app-2-job-1", app2Job1Started)

	controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, secretproviderclient, NewApplicationController(
		func(_ context.Context, _ kubernetes.Interface, _ v1.RadixRegistration) (bool, error) {
			return true, nil
		}, newTestApplicationHandlerFactory(ApplicationHandlerConfig{RequireAppConfigurationItem: true, RequireAppADGroups: true},
			func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
				return true, nil
			})))

	// Tests
	t.Run("search for "+appNames[0], func(t *testing.T) {
		searchParam := applicationModels.ApplicationsSearchRequest{Names: []string{appNames[0]}}
		responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications/_search", &searchParam)
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		controllertest.GetResponseBody(response, &applications)
		assert.Equal(t, 1, len(applications))
		assert.Equal(t, appNames[0], applications[0].Name)
	})

	t.Run("search for both apps", func(t *testing.T) {
		searchParam := applicationModels.ApplicationsSearchRequest{Names: appNames}
		responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications/_search", &searchParam)
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		controllertest.GetResponseBody(response, &applications)
		assert.Equal(t, 2, len(applications))
	})

	t.Run("empty appname list", func(t *testing.T) {
		searchParam := applicationModels.ApplicationsSearchRequest{Names: []string{}}
		responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications/_search", &searchParam)
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		controllertest.GetResponseBody(response, &applications)
		assert.Equal(t, 0, len(applications))
	})

	t.Run("search for "+appNames[1]+" - with includeFields 'LatestJobSummary'", func(t *testing.T) {
		searchParam := applicationModels.ApplicationsSearchRequest{
			Names: []string{appNames[1]},
			IncludeFields: applicationModels.ApplicationSearchIncludeFields{
				LatestJobSummary: true,
			},
		}
		responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications/_search", &searchParam)
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		controllertest.GetResponseBody(response, &applications)
		assert.Equal(t, 1, len(applications))
		assert.Equal(t, appNames[1], applications[0].Name)
		assert.NotNil(t, applications[0].LatestJob)
		assert.Nil(t, applications[0].EnvironmentActiveComponents)
	})

	t.Run("search for "+appNames[1]+" - with includeFields 'EnvironmentActiveComponents'", func(t *testing.T) {
		searchParam := applicationModels.ApplicationsSearchRequest{
			Names: []string{appNames[1]},
			IncludeFields: applicationModels.ApplicationSearchIncludeFields{
				EnvironmentActiveComponents: true,
			},
		}
		responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications/_search", &searchParam)
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		controllertest.GetResponseBody(response, &applications)
		assert.Equal(t, 1, len(applications))
		assert.Equal(t, appNames[1], applications[0].Name)
		assert.Nil(t, applications[0].LatestJob)
		assert.NotNil(t, applications[0].EnvironmentActiveComponents)
	})

	t.Run("search for "+appNames[0]+" - no access", func(t *testing.T) {
		controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, secretproviderclient, NewApplicationController(
			func(_ context.Context, _ kubernetes.Interface, _ v1.RadixRegistration) (bool, error) {
				return false, nil
			}, newTestApplicationHandlerFactory(ApplicationHandlerConfig{RequireAppConfigurationItem: true, RequireAppADGroups: true},
				func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
					return true, nil
				})))
		searchParam := applicationModels.ApplicationsSearchRequest{Names: []string{appNames[0]}}
		responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications/_search", &searchParam)
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		controllertest.GetResponseBody(response, &applications)
		assert.Equal(t, 0, len(applications))
	})
}

func TestSearchApplications_WithJobs_ShouldOnlyHaveLatest(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _ := setupTest(true, true)
	appNames := []string{"app-1", "app-2", "app-3"}

	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName(appNames[0]))
	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName(appNames[1]))
	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName(appNames[2]))

	commontest.CreateAppNamespace(kubeclient, appNames[0])
	commontest.CreateAppNamespace(kubeclient, appNames[1])
	commontest.CreateAppNamespace(kubeclient, appNames[2])

	app1Job1Started, _ := radixutils.ParseTimestamp("2018-11-12T11:45:26Z")
	app2Job1Started, _ := radixutils.ParseTimestamp("2018-11-12T12:30:14Z")
	app2Job2Started, _ := radixutils.ParseTimestamp("2018-11-20T09:00:00Z")
	app2Job3Started, _ := radixutils.ParseTimestamp("2018-11-20T09:00:01Z")

	createRadixJob(commonTestUtils, appNames[0], "app-1-job-1", app1Job1Started)
	createRadixJob(commonTestUtils, appNames[1], "app-2-job-1", app2Job1Started)
	createRadixJob(commonTestUtils, appNames[1], "app-2-job-2", app2Job2Started)
	createRadixJob(commonTestUtils, appNames[1], "app-2-job-3", app2Job3Started)

	// Test
	searchParam := applicationModels.ApplicationsSearchRequest{
		Names: appNames,
		IncludeFields: applicationModels.ApplicationSearchIncludeFields{
			LatestJobSummary: true,
		},
	}
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications/_search", &searchParam)
	response := <-responseChannel

	applications := make([]*applicationModels.ApplicationSummary, 0)
	controllertest.GetResponseBody(response, &applications)

	for _, application := range applications {
		if strings.EqualFold(application.Name, appNames[0]) {
			assert.NotNil(t, application.LatestJob)
			assert.Equal(t, "app-1-job-1", application.LatestJob.Name)
		} else if strings.EqualFold(application.Name, appNames[1]) {
			assert.NotNil(t, application.LatestJob)
			assert.Equal(t, "app-2-job-3", application.LatestJob.Name)
		} else if strings.EqualFold(application.Name, appNames[2]) {
			assert.Nil(t, application.LatestJob)
		}
	}
}

func TestCreateApplication_NoName_ValidationError(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _ := setupTest(true, true)

	// Test
	parameters := buildApplicationRegistrationRequest(
		anApplicationRegistration().WithName("").Build(),
		false,
	)
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, "Error: app name cannot be empty", errorResponse.Message)
}

func TestCreateApplication_WhenRequiredConfigurationItemIsNotSet_ReturnError(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _ := setupTest(true, true)

	// Test
	parameters := buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-name-2").
			WithRepository("https://github.com/Equinor/any-repo").
			WithConfigurationItem("").
			Build(),
		false)
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := radixvalidators.ResourceNameCannotBeEmptyError("configuration item")
	assert.Equal(t, fmt.Sprintf("Error: %v", expectedError), errorResponse.Message)
}

func TestCreateApplication_WhenOptionalConfigurationItemIsNotSet_ReturnSuccess(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _ := setupTest(false, true)

	// Test
	parameters := buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-name-2").
			WithRepository("https://github.com/Equinor/any-repo").
			WithConfigurationItem("").
			Build(),
		false)
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestCreateApplication_WhenRequiredAdGroupsIsNotSet_ReturnError(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _ := setupTest(true, true)

	// Test
	parameters := buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-name-2").
			WithRepository("https://github.com/Equinor/any-repo").
			WithAdGroups(nil).
			Build(),
		false)
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := radixvalidators.ResourceNameCannotBeEmptyError("AD groups")
	assert.Equal(t, fmt.Sprintf("Error: %v", expectedError), errorResponse.Message)
}

func TestCreateApplication_WhenOptionalAdGroupsIsNotSet_ReturnSuccess(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _ := setupTest(true, false)

	// Test
	parameters := buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-name-2").
			WithRepository("https://github.com/Equinor/any-repo").
			WithAdGroups(nil).
			Build(),
		false,
	)
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestCreateApplication_WhenConfigBranchIsNotSet_ReturnError(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _ := setupTest(true, true)

	// Test
	parameters := buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-name").
			WithRepository("https://github.com/Equinor/any-repo").
			WithConfigBranch("").
			Build(),
		false,
	)
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := radixvalidators.ResourceNameCannotBeEmptyError("branch name")
	assert.Equal(t, fmt.Sprintf("Error: %v", expectedError), errorResponse.Message)
}

func TestCreateApplication_WhenConfigBranchIsInvalid_ReturnError(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _ := setupTest(true, true)

	// Test
	configBranch := "main.."
	parameters := buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-name").
			WithRepository("https://github.com/Equinor/any-repo").
			WithConfigBranch(configBranch).
			Build(),
		false)
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := radixvalidators.InvalidConfigBranchName(configBranch)
	assert.Equal(t, fmt.Sprintf("Error: %v", expectedError), errorResponse.Message)
}

func TestCreateApplication_WithRadixConfigFullName(t *testing.T) {
	scenarios := []struct {
		radixConfigFullName                   string
		expectedError                         bool
		expectedRegisteredRadixConfigFullName string
	}{
		{radixConfigFullName: "", expectedError: false, expectedRegisteredRadixConfigFullName: ""},
		{radixConfigFullName: "radixconfig.yaml", expectedError: false, expectedRegisteredRadixConfigFullName: "radixconfig.yaml"},
		{radixConfigFullName: "a.yaml", expectedError: false, expectedRegisteredRadixConfigFullName: "a.yaml"},
		{radixConfigFullName: "abc/a.yaml", expectedError: false, expectedRegisteredRadixConfigFullName: "abc/a.yaml"},
		{radixConfigFullName: "/abc/a.yaml", expectedError: false, expectedRegisteredRadixConfigFullName: "abc/a.yaml"},
		{radixConfigFullName: " /abc/a.yaml ", expectedError: false, expectedRegisteredRadixConfigFullName: "abc/a.yaml"},
		{radixConfigFullName: "/abc/de.f/a.yaml", expectedError: false, expectedRegisteredRadixConfigFullName: "abc/de.f/a.yaml"},
		{radixConfigFullName: "abc\\de.f\\a.yaml", expectedError: false, expectedRegisteredRadixConfigFullName: "abc/de.f/a.yaml"},
		{radixConfigFullName: "abc/d-e_f/radixconfig.yaml", expectedError: false, expectedRegisteredRadixConfigFullName: "abc/d-e_f/radixconfig.yaml"},
		{radixConfigFullName: "abc/12.3abc/radixconfig.yaml", expectedError: false, expectedRegisteredRadixConfigFullName: "abc/12.3abc/radixconfig.yaml"},
		{radixConfigFullName: ".yaml", expectedError: true},
		{radixConfigFullName: "radixconfig.yml", expectedError: false, expectedRegisteredRadixConfigFullName: "radixconfig.yml"},
		{radixConfigFullName: "a", expectedError: true},
		{radixConfigFullName: "#radixconfig.yaml", expectedError: true},
	}
	for _, scenario := range scenarios {
		t.Run(fmt.Sprintf("Test for radixConfigFullName: '%s'", scenario.radixConfigFullName), func(t *testing.T) {
			// Setup
			_, controllerTestUtils, _, _, _, _ := setupTest(true, true)

			// Test
			configBranch := "main"
			parameters := buildApplicationRegistrationRequest(
				anApplicationRegistration().
					WithName("any-name").
					WithRepository("https://github.com/Equinor/any-repo").
					WithConfigBranch(configBranch).
					WithRadixConfigFullName(scenario.radixConfigFullName).
					Build(),
				false)
			responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
			response := <-responseChannel

			if scenario.expectedError {
				require.Equal(t, http.StatusBadRequest, response.Code)
				errorResponse, _ := controllertest.GetErrorResponse(response)
				assert.Equal(t, fmt.Sprintf("Error: %v", "invalid file name for radixconfig. See https://www.radix.equinor.com/references/reference-radix-config/ for more information"), errorResponse.Message)
			} else {
				require.Equal(t, http.StatusOK, response.Code)
				registrationResponse := applicationModels.ApplicationRegistrationUpsertResponse{}
				if err := json.NewDecoder(bytes.NewReader(response.Body.Bytes())).Decode(&registrationResponse); err != nil {
					assert.Fail(t, err.Error())
				} else {
					assert.Equal(t, scenario.expectedRegisteredRadixConfigFullName, registrationResponse.ApplicationRegistration.RadixConfigFullName)
				}
			}
		})
	}
}

func TestCreateApplication_DuplicateRepo_ShouldWarn(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _ := setupTest(true, true)

	parameters := buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-name").
			WithRepository("https://github.com/Equinor/any-repo").
			Build(),
		false,
	)
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	// Test
	parameters = buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-other-name").
			WithRepository("https://github.com/Equinor/any-repo").
			Build(),
		false,
	)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)
	applicationRegistrationUpsertResponse := applicationModels.ApplicationRegistrationUpsertResponse{}
	controllertest.GetResponseBody(response, &applicationRegistrationUpsertResponse)
	assert.NotEmpty(t, applicationRegistrationUpsertResponse.Warnings)
	assert.Contains(t, applicationRegistrationUpsertResponse.Warnings, "Repository is used in other application(s)")
}

func TestCreateApplication_DuplicateRepoWithAcknowledgeWarning_ShouldSuccess(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _ := setupTest(true, true)

	parameters := buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-name").
			WithRepository("https://github.com/Equinor/any-repo").
			Build(),
		false,
	)
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	// Test
	parameters = buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-other-name").
			WithRepository("https://github.com/Equinor/any-repo").
			Build(),
		true,
	)
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)
	applicationRegistrationUpsertResponse := applicationModels.ApplicationRegistrationUpsertResponse{}
	controllertest.GetResponseBody(response, &applicationRegistrationUpsertResponse)
	assert.Empty(t, applicationRegistrationUpsertResponse.Warnings)
	assert.NotEmpty(t, applicationRegistrationUpsertResponse.ApplicationRegistration)
}

func TestGetApplication_AllFieldsAreSet(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _ := setupTest(true, true)

	parameters := buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-name").
			WithRepository("https://github.com/Equinor/any-repo").
			WithSharedSecret("Any secret").
			WithAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"}).
			WithConfigBranch("abranch").
			WithRadixConfigFullName("a/custom-radixconfig.yaml").
			WithConfigurationItem("ci").
			Build(),
		false,
	)

	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	// Test
	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response := <-responseChannel

	application := applicationModels.Application{}
	controllertest.GetResponseBody(response, &application)

	assert.Equal(t, "https://github.com/Equinor/any-repo", application.Registration.Repository)
	assert.Equal(t, "Any secret", application.Registration.SharedSecret)
	assert.Equal(t, []string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"}, application.Registration.AdGroups)
	assert.Equal(t, "not-existing-test-radix-email@equinor.com", application.Registration.Creator)
	assert.Equal(t, "abranch", application.Registration.ConfigBranch)
	assert.Equal(t, "a/custom-radixconfig.yaml", application.Registration.RadixConfigFullName)
	assert.Equal(t, "ci", application.Registration.ConfigurationItem)
}

func TestGetApplication_WithJobs(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _ := setupTest(true, true)
	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName("any-name"))

	commontest.CreateAppNamespace(kubeclient, "any-name")
	app1Job1Started, _ := radixutils.ParseTimestamp("2018-11-12T11:45:26Z")
	app1Job2Started, _ := radixutils.ParseTimestamp("2018-11-12T12:30:14Z")
	app1Job3Started, _ := radixutils.ParseTimestamp("2018-11-20T09:00:00Z")

	createRadixJob(commonTestUtils, "any-name", "any-name-job-1", app1Job1Started)
	createRadixJob(commonTestUtils, "any-name", "any-name-job-2", app1Job2Started)
	createRadixJob(commonTestUtils, "any-name", "any-name-job-3", app1Job3Started)

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response := <-responseChannel

	application := applicationModels.Application{}
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, 3, len(application.Jobs))
}

func TestGetApplication_WithEnvironments(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, radix, _, _ := setupTest(true, true)

	anyAppName := "any-app"
	anyOrphanedEnvironment := "feature"

	commonTestUtils.ApplyRegistration(builders.
		NewRegistrationBuilder().
		WithName(anyAppName))

	commonTestUtils.ApplyApplication(builders.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment("dev", "master").
		WithEnvironment("prod", "release"))

	commonTestUtils.ApplyDeployment(builders.
		NewDeploymentBuilder().
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithImageTag("someimageindev"))

	commonTestUtils.ApplyDeployment(builders.
		NewDeploymentBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyOrphanedEnvironment).
		WithImageTag("someimageinfeature"))

	// Set RE statuses
	devRe, err := radix.RadixV1().RadixEnvironments().Get(context.Background(), builders.GetEnvironmentNamespace(anyAppName, "dev"), metav1.GetOptions{})
	require.NoError(t, err)
	devRe.Status.Reconciled = metav1.Now()
	radix.RadixV1().RadixEnvironments().UpdateStatus(context.Background(), devRe, metav1.UpdateOptions{})
	prodRe, err := radix.RadixV1().RadixEnvironments().Get(context.Background(), builders.GetEnvironmentNamespace(anyAppName, "prod"), metav1.GetOptions{})
	require.NoError(t, err)
	prodRe.Status.Reconciled = metav1.Time{}
	radix.RadixV1().RadixEnvironments().UpdateStatus(context.Background(), prodRe, metav1.UpdateOptions{})

	orphanedRe, _ := commonTestUtils.ApplyEnvironment(builders.
		NewEnvironmentBuilder().
		WithAppLabel().
		WithAppName(anyAppName).
		WithEnvironmentName(anyOrphanedEnvironment))
	orphanedRe.Status.Reconciled = metav1.Now()
	orphanedRe.Status.Orphaned = true
	radix.RadixV1().RadixEnvironments().Update(context.Background(), orphanedRe, metav1.UpdateOptions{})

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", anyAppName))
	response := <-responseChannel

	application := applicationModels.Application{}
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, 3, len(application.Environments))

	for _, environment := range application.Environments {
		if strings.EqualFold(environment.Name, "dev") {
			assert.Equal(t, environmentModels.Consistent.String(), environment.Status)
			assert.NotNil(t, environment.ActiveDeployment)
		} else if strings.EqualFold(environment.Name, "prod") {
			assert.Equal(t, environmentModels.Pending.String(), environment.Status)
			assert.Nil(t, environment.ActiveDeployment)
		} else if strings.EqualFold(environment.Name, anyOrphanedEnvironment) {
			assert.Equal(t, environmentModels.Orphan.String(), environment.Status)
			assert.NotNil(t, environment.ActiveDeployment)
		}
	}
}

func TestUpdateApplication_DuplicateRepo_ShouldWarn(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _ := setupTest(true, true)

	parameters := buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-name").
			WithRepository("https://github.com/Equinor/any-repo").
			Build(),
		false,
	)

	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	parameters = buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-other-name").
			WithRepository("https://github.com/Equinor/any-other-repo").
			Build(),
		false,
	)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	// Test
	parameters = buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-other-name").
			WithRepository("https://github.com/Equinor/any-repo").
			Build(),
		false,
	)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-other-name"), parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)
	registrationUpsertResponse := applicationModels.ApplicationRegistrationUpsertResponse{}
	controllertest.GetResponseBody(response, &registrationUpsertResponse)
	assert.NotEmpty(t, registrationUpsertResponse.Warnings)
}

func TestUpdateApplication_DuplicateRepoWithAcknowledgeWarnings_ShouldSuccess(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _ := setupTest(true, true)

	parameters := buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-name").
			WithRepository("https://github.com/Equinor/any-repo").
			Build(),
		false,
	)

	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	parameters = buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-other-name").
			WithRepository("https://github.com/Equinor/any-other-repo").
			Build(),
		false,
	)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	// Test
	parameters = buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-other-name").
			WithRepository("https://github.com/Equinor/any-repo").
			Build(),
		true,
	)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-other-name"), parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)
	registrationUpsertResponse := applicationModels.ApplicationRegistrationUpsertResponse{}
	controllertest.GetResponseBody(response, &registrationUpsertResponse)
	assert.Empty(t, registrationUpsertResponse.Warnings)
	assert.NotNil(t, registrationUpsertResponse.ApplicationRegistration)
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestUpdateApplication_MismatchingNameOrNotExists_ShouldFailAsIllegalOperation(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _ := setupTest(true, true)

	parameters := buildApplicationRegistrationRequest(anApplicationRegistration().WithName("any-name").Build(), false)
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	// Test
	parameters = buildApplicationRegistrationRequest(anApplicationRegistration().WithName("any-name").Build(), false)
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "another-name"), parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusNotFound, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, controllertest.AppNotFoundErrorMsg("another-name"), errorResponse.Message)

	parameters = buildApplicationRegistrationRequest(anApplicationRegistration().WithName("another-name").Build(), false)
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), parameters)
	response = <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	assert.Equal(t, "App name any-name does not correspond with application name another-name", errorResponse.Message)

	parameters = buildApplicationRegistrationRequest(anApplicationRegistration().WithName("another-name").Build(), false)
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "another-name"), parameters)
	response = <-responseChannel
	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestUpdateApplication_AbleToSetAnySpecField(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _ := setupTest(true, true)

	builder :=
		anApplicationRegistration().
			WithName("any-name").
			WithRepository("https://github.com/Equinor/a-repo").
			WithSharedSecret("")
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", buildApplicationRegistrationRequest(builder.Build(), false))
	<-responseChannel

	// Test Repository
	newRepository := "https://github.com/Equinor/any-repo"
	builder = builder.
		WithRepository(newRepository)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), buildApplicationRegistrationRequest(builder.Build(), false))
	response := <-responseChannel

	applicationRegistrationUpsertResponse := applicationModels.ApplicationRegistrationUpsertResponse{}
	controllertest.GetResponseBody(response, &applicationRegistrationUpsertResponse)
	assert.NotEmpty(t, applicationRegistrationUpsertResponse.ApplicationRegistration)
	assert.Equal(t, newRepository, applicationRegistrationUpsertResponse.ApplicationRegistration.Repository)

	// Test SharedSecret
	newSharedSecret := "Any shared secret"
	builder = builder.
		WithSharedSecret(newSharedSecret)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), buildApplicationRegistrationRequest(builder.Build(), false))
	response = <-responseChannel
	applicationRegistrationUpsertResponse = applicationModels.ApplicationRegistrationUpsertResponse{}
	controllertest.GetResponseBody(response, &applicationRegistrationUpsertResponse)
	assert.Equal(t, newSharedSecret, applicationRegistrationUpsertResponse.ApplicationRegistration.SharedSecret)

	// Test WBS
	newWbs := "new.wbs.code"
	builder = builder.
		WithWBS(newWbs)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), buildApplicationRegistrationRequest(builder.Build(), false))
	response = <-responseChannel
	applicationRegistrationUpsertResponse = applicationModels.ApplicationRegistrationUpsertResponse{}
	controllertest.GetResponseBody(response, &applicationRegistrationUpsertResponse)
	assert.Equal(t, newWbs, applicationRegistrationUpsertResponse.ApplicationRegistration.WBS)

	// Test ConfigBranch
	newConfigBranch := "newcfgbranch"
	builder = builder.
		WithConfigBranch(newConfigBranch)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), buildApplicationRegistrationRequest(builder.Build(), false))
	response = <-responseChannel
	applicationRegistrationUpsertResponse = applicationModels.ApplicationRegistrationUpsertResponse{}
	controllertest.GetResponseBody(response, &applicationRegistrationUpsertResponse)
	assert.Equal(t, newConfigBranch, applicationRegistrationUpsertResponse.ApplicationRegistration.ConfigBranch)

	// Test ConfigurationItem
	newConfigurationItem := "newci"
	builder = builder.
		WithConfigurationItem(newConfigurationItem)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), buildApplicationRegistrationRequest(builder.Build(), false))
	response = <-responseChannel
	applicationRegistrationUpsertResponse = applicationModels.ApplicationRegistrationUpsertResponse{}
	controllertest.GetResponseBody(response, &applicationRegistrationUpsertResponse)
	assert.Equal(t, newConfigurationItem, applicationRegistrationUpsertResponse.ApplicationRegistration.ConfigurationItem)
}

func TestModifyApplication_AbleToSetField(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _ := setupTest(true, true)

	builder := anApplicationRegistration().
		WithName("any-name").
		WithRepository("https://github.com/Equinor/a-repo").
		WithSharedSecret("").
		WithAdGroups([]string{"a5dfa635-dc00-4a28-9ad9-9e7f1e56919d"}).
		WithReaderAdGroups([]string{"d5df55c1-78b7-4330-9d2c-f1b1aa5584ca"}).
		WithOwner("AN_OWNER@equinor.com").
		WithWBS("T.O123A.AZ.45678").
		WithConfigBranch("main1").
		WithConfigurationItem("ci-initial")
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", buildApplicationRegistrationRequest(builder.Build(), false))
	<-responseChannel

	// Test
	anyNewAdGroup := []string{"98765432-dc00-4a28-9ad9-9e7f1e56919d"}
	patchRequest := applicationModels.ApplicationRegistrationPatchRequest{
		ApplicationRegistrationPatch: &applicationModels.ApplicationRegistrationPatch{
			AdGroups: &anyNewAdGroup,
		},
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel

	application := applicationModels.Application{}
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, anyNewAdGroup, application.Registration.AdGroups)
	assert.Equal(t, "AN_OWNER@equinor.com", application.Registration.Owner)
	assert.Equal(t, "T.O123A.AZ.45678", application.Registration.WBS)
	assert.Equal(t, "main1", application.Registration.ConfigBranch)
	assert.Equal(t, "ci-initial", application.Registration.ConfigurationItem)

	// Test
	anyNewReaderAdGroup := []string{"44643b96-0f6d-4bdc-af2c-a4f596d821eb"}
	patchRequest = applicationModels.ApplicationRegistrationPatchRequest{
		ApplicationRegistrationPatch: &applicationModels.ApplicationRegistrationPatch{
			ReaderAdGroups: &anyNewReaderAdGroup,
		},
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	<-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, anyNewReaderAdGroup, application.Registration.ReaderAdGroups)

	// Test
	anyNewOwner := "A_NEW_OWNER@equinor.com"
	patchRequest = applicationModels.ApplicationRegistrationPatchRequest{
		ApplicationRegistrationPatch: &applicationModels.ApplicationRegistrationPatch{
			Owner: &anyNewOwner,
		},
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	<-responseChannel

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel

	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, anyNewAdGroup, application.Registration.AdGroups)
	assert.Equal(t, anyNewOwner, application.Registration.Owner)

	// Test
	anyNewWBS := "A.BCD.00.999"
	patchRequest = applicationModels.ApplicationRegistrationPatchRequest{
		ApplicationRegistrationPatch: &applicationModels.ApplicationRegistrationPatch{
			WBS: &anyNewWBS,
		},
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	<-responseChannel

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel

	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, anyNewWBS, application.Registration.WBS)

	// Test ConfigBranch
	anyNewConfigBranch := "main2"
	patchRequest = applicationModels.ApplicationRegistrationPatchRequest{
		ApplicationRegistrationPatch: &applicationModels.ApplicationRegistrationPatch{
			ConfigBranch: &anyNewConfigBranch,
		},
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	<-responseChannel

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel

	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, anyNewConfigBranch, application.Registration.ConfigBranch)

	// Test ConfigurationItem
	anyNewConfigurationItem := "ci-patch"
	patchRequest = applicationModels.ApplicationRegistrationPatchRequest{
		ApplicationRegistrationPatch: &applicationModels.ApplicationRegistrationPatch{
			ConfigurationItem: &anyNewConfigurationItem,
		},
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	<-responseChannel

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel

	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, anyNewConfigurationItem, application.Registration.ConfigurationItem)
}

func TestModifyApplication_AbleToUpdateRepository(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _ := setupTest(true, true)

	builder := anApplicationRegistration().
		WithName("any-name").
		WithRepository("https://github.com/Equinor/a-repo")
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", buildApplicationRegistrationRequest(builder.Build(), false))
	<-responseChannel

	// Test
	anyNewRepo := "https://github.com/repo/updated-version"
	patchRequest := applicationModels.ApplicationRegistrationPatchRequest{
		ApplicationRegistrationPatch: &applicationModels.ApplicationRegistrationPatch{
			Repository: &anyNewRepo,
		},
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	<-responseChannel

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response := <-responseChannel

	application := applicationModels.Application{}
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, anyNewRepo, application.Registration.Repository)
}

func TestModifyApplication_ConfigBranchSetToFallbackHack(t *testing.T) {
	// Setup
	appName := "any-name"
	_, controllerTestUtils, _, radixClient, _, _ := setupTest(true, true)
	rr := builders.ARadixRegistration().
		WithName(appName).
		WithConfigurationItem("any").
		WithConfigBranch("")
	radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr.BuildRR(), metav1.CreateOptions{})

	// Test
	anyNewRepo := "https://github.com/repo/updated-version"
	patchRequest := applicationModels.ApplicationRegistrationPatchRequest{
		ApplicationRegistrationPatch: &applicationModels.ApplicationRegistrationPatch{
			Repository: &anyNewRepo,
		},
	}

	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", appName), patchRequest)
	<-responseChannel

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", appName))
	response := <-responseChannel

	application := applicationModels.Application{}
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, applicationconfig.ConfigBranchFallback, application.Registration.ConfigBranch)
}

func TestModifyApplication_IgnoreRequireCIValidationWhenRequiredButCurrentIsEmpty(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, radixClient, _, _ := setupTest(true, true)

	rr, err := anApplicationRegistration().
		WithName("any-name").
		WithConfigurationItem("").
		BuildRR()
	require.NoError(t, err)
	_, err = radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	require.NoError(t, err)

	// Test
	patchRequest := applicationModels.ApplicationRegistrationPatchRequest{
		ApplicationRegistrationPatch: &applicationModels.ApplicationRegistrationPatch{
			ConfigBranch: radixutils.StringPtr("dummyupdate"),
		},
	}

	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestModifyApplication_IgnoreRequireADGroupValidationWhenRequiredButCurrentIsEmpty(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, radixClient, _, _ := setupTest(true, true)

	rr, err := anApplicationRegistration().
		WithName("any-name").
		WithAdGroups(nil).
		BuildRR()
	require.NoError(t, err)
	_, err = radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	require.NoError(t, err)

	// Test
	patchRequest := applicationModels.ApplicationRegistrationPatchRequest{
		ApplicationRegistrationPatch: &applicationModels.ApplicationRegistrationPatch{
			ConfigBranch: radixutils.StringPtr("dummyupdate"),
		},
	}

	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestHandleTriggerPipeline_ForNonMappedAndMappedAndMagicBranchEnvironment_JobIsNotCreatedForUnmapped(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _, _ := setupTest(true, true)
	anyAppName := "any-app"
	configBranch := "magic"

	rr := builders.ARadixRegistration().WithConfigBranch(configBranch).WithAdGroups([]string{"adminGroup"})
	commonTestUtils.ApplyApplication(builders.
		ARadixApplication().
		WithRadixRegistration(rr).
		WithAppName(anyAppName).
		WithEnvironment("dev", "dev").
		WithEnvironment("prod", "release"),
	)

	// Test
	unmappedBranch := "feature"

	parameters := applicationModels.PipelineParametersBuild{Branch: unmappedBranch}
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", anyAppName, v1.BuildDeploy), parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := applicationModels.UnmatchedBranchToEnvironment(unmappedBranch)
	assert.Equal(t, (expectedError.(*radixhttp.Error)).Message, errorResponse.Message)

	// Mapped branch should start job
	parameters = applicationModels.PipelineParametersBuild{Branch: "dev"}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", anyAppName, v1.BuildDeploy), parameters)
	response = <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)

	// Magic branch should start job, even if it is not mapped
	parameters = applicationModels.PipelineParametersBuild{Branch: configBranch}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", anyAppName, v1.BuildDeploy), parameters)
	response = <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestHandleTriggerPipeline_ExistingAndNonExistingApplication_JobIsCreatedForExisting(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _ := setupTest(true, true)

	registerAppParam := buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-app").
			WithConfigBranch("maincfg").
			Build(),
		false,
	)
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", registerAppParam)
	<-responseChannel

	// Test
	const pushCommitID = "4faca8595c5283a9d0f17a623b9255a0d9866a2e"

	parameters := applicationModels.PipelineParametersBuild{Branch: "master", CommitID: pushCommitID}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", "another-app", v1.BuildDeploy), parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusNotFound, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, controllertest.AppNotFoundErrorMsg("another-app"), errorResponse.Message)

	parameters = applicationModels.PipelineParametersBuild{Branch: "", CommitID: pushCommitID}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", "any-app", v1.BuildDeploy), parameters)
	response = <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	expectedError := applicationModels.AppNameAndBranchAreRequiredForStartingPipeline()
	assert.Equal(t, (expectedError.(*radixhttp.Error)).Message, errorResponse.Message)

	parameters = applicationModels.PipelineParametersBuild{Branch: "maincfg", CommitID: pushCommitID}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", "any-app", v1.BuildDeploy), parameters)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	jobSummary := jobModels.JobSummary{}
	controllertest.GetResponseBody(response, &jobSummary)
	assert.Equal(t, "any-app", jobSummary.AppName)
	assert.Equal(t, "maincfg", jobSummary.Branch)
	assert.Equal(t, pushCommitID, jobSummary.CommitID)
}

func TestHandleTriggerPipeline_Deploy_JobHasCorrectParameters(t *testing.T) {
	appName := "an-app"

	type scenario struct {
		name                  string
		params                applicationModels.PipelineParametersDeploy
		expectedToEnvironment string
		expectedImageTagNames map[string]string
	}

	scenarios := []scenario{
		{
			name:                  "only target environment",
			params:                applicationModels.PipelineParametersDeploy{ToEnvironment: "target"},
			expectedToEnvironment: "target",
		},
		{
			name:                  "target environment with image tags",
			params:                applicationModels.PipelineParametersDeploy{ToEnvironment: "target", ImageTagNames: map[string]string{"component1": "tag1", "component2": "tag22"}},
			expectedToEnvironment: "target",
			expectedImageTagNames: map[string]string{"component1": "tag1", "component2": "tag22"},
		},
	}

	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			_, controllerTestUtils, _, radixclient, _, _ := setupTest(true, true)
			registerAppParam := buildApplicationRegistrationRequest(anApplicationRegistration().WithName(appName).Build(), false)
			<-controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", registerAppParam)
			responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", appName, v1.Deploy), ts.params)
			<-responseChannel

			appNamespace := fmt.Sprintf("%s-app", appName)
			jobs, _ := getJobsInNamespace(radixclient, appNamespace)

			assert.Equal(t, ts.expectedToEnvironment, jobs[0].Spec.Deploy.ToEnvironment)
			assert.Equal(t, ts.expectedImageTagNames, jobs[0].Spec.Deploy.ImageTagNames)
		})
	}
}

func TestHandleTriggerPipeline_Promote_JobHasCorrectParameters(t *testing.T) {
	_, controllerTestUtils, _, radixclient, _, _ := setupTest(true, true)

	appName := "an-app"

	parameters := applicationModels.PipelineParametersPromote{
		FromEnvironment: "origin",
		ToEnvironment:   "target",
		DeploymentName:  "a-deployment",
	}
	registerAppParam := buildApplicationRegistrationRequest(anApplicationRegistration().WithName(appName).Build(), false)
	<-controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", registerAppParam)
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", appName, v1.Promote), parameters)
	<-responseChannel

	appNamespace := fmt.Sprintf("%s-app", appName)
	jobs, _ := getJobsInNamespace(radixclient, appNamespace)

	assert.Equal(t, jobs[0].Spec.Promote.FromEnvironment, "origin")
	assert.Equal(t, jobs[0].Spec.Promote.ToEnvironment, "target")
	assert.Equal(t, jobs[0].Spec.Promote.DeploymentName, "a-deployment")
}

func TestIsDeployKeyValid(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _ := setupTest(true, true)
	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName("some-app").
		WithPublicKey("some-public-key").
		WithPrivateKey("some-private-key"))

	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName("some-app-missing-key").
		WithPublicKey("").
		WithPrivateKey(""))

	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName("some-app-missing-repository").
		WithCloneURL(""))

	// Tests
	t.Run("missing rr", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deploykey-valid", "some-nonexisting-app"))
		response := <-responseChannel

		assert.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("missing repository", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deploykey-valid", "some-app-missing-repository"))
		response := <-responseChannel

		assert.Equal(t, http.StatusBadRequest, response.Code)

		errorResponse, _ := controllertest.GetErrorResponse(response)
		assert.Equal(t, "Clone URL is missing", errorResponse.Message)
	})

	t.Run("missing key", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deploykey-valid", "some-app-missing-key"))
		response := <-responseChannel

		assert.Equal(t, http.StatusBadRequest, response.Code)

		errorResponse, _ := controllertest.GetErrorResponse(response)
		assert.Equal(t, "Deploy key is missing", errorResponse.Message)
	})

	t.Run("valid key", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deploykey-valid", "some-app"))
		setStatusOfCloneJob(kubeclient, "some-app-app", true)

		response := <-responseChannel
		assert.Equal(t, http.StatusOK, response.Code)
	})

	t.Run("invalid key", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deploykey-valid", "some-app"))
		setStatusOfCloneJob(kubeclient, "some-app-app", false)

		response := <-responseChannel
		assert.Equal(t, http.StatusBadRequest, response.Code)

		errorResponse, _ := controllertest.GetErrorResponse(response)
		assert.Equal(t, "Deploy key was invalid", errorResponse.Message)
	})
}

func TestDeleteApplication_ApplicationIsDeleted(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _ := setupTest(true, true)

	parameters := buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-name").
			Build(),
		false,
	)

	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	// Test
	responseChannel = controllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s", "any-non-existing"))
	response := <-responseChannel
	assert.Equal(t, http.StatusNotFound, response.Code)

	responseChannel = controllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	// Application should no longer exist
	responseChannel = controllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel
	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestGetApplication_WithAppAlias_ContainsAppAlias(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient, promclient, secretproviderclient := setupTest(true, true)
	utils.ApplyDeploymentWithSync(client, radixclient, promclient, commonTestUtils, secretproviderclient, builders.ARadixDeployment().
		WithAppName("any-app").
		WithEnvironment("prod").
		WithComponents(
			builders.NewDeployComponentBuilder().
				WithName("frontend").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true),
			builders.NewDeployComponentBuilder().
				WithName("backend")))

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-app"))
	response := <-responseChannel

	application := applicationModels.Application{}
	controllertest.GetResponseBody(response, &application)

	assert.NotNil(t, application.AppAlias)
	assert.Equal(t, "frontend", application.AppAlias.ComponentName)
	assert.Equal(t, "prod", application.AppAlias.EnvironmentName)
	assert.Equal(t, fmt.Sprintf("%s.%s", "any-app", appAliasDNSZone), application.AppAlias.URL)
}

func TestListPipeline_ReturnesAvailablePipelines(t *testing.T) {
	supportedPipelines := jobPipeline.GetSupportedPipelines()

	// Setup
	commonTestUtils, controllerTestUtils, _, _, _, _ := setupTest(true, true)
	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName("some-app").
		WithPublicKey("some-public-key").
		WithPrivateKey("some-private-key"))

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/pipelines", "some-app"))
	response := <-responseChannel

	pipelines := make([]string, 0)
	controllertest.GetResponseBody(response, &pipelines)
	assert.Equal(t, len(supportedPipelines), len(pipelines))
}

func TestRegenerateDeployKey_WhenApplicationNotExist_Fail(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _ := setupTest(true, true)

	// Test
	parameters := buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-name").
			WithRepository("https://github.com/Equinor/any-repo").
			Build(),
		false,
	)

	appResponseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-appResponseChannel

	regenerateParameters := &applicationModels.RegenerateDeployKeyAndSecretData{SharedSecret: "new shared secret"}
	appName := "any-non-existing-name"
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/regenerate-deploy-key", appName), regenerateParameters)
	response := <-responseChannel

	deployKeyAndSecret := applicationModels.DeployKeyAndSecret{}
	controllertest.GetResponseBody(response, &deployKeyAndSecret)
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Empty(t, deployKeyAndSecret.PublicDeployKey)
	assert.Empty(t, deployKeyAndSecret.SharedSecret)
}

func TestRegenerateDeployKey_NoSecretInParam_SecretIsReCreated(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeUtil, radixClient, _, _ := setupTest(true, true)
	appName := "any-name"
	rrBuilder := builders.ARadixRegistration().WithName(appName).WithCloneURL("git@github.com:Equinor/my-app.git")

	// Creating RR and syncing it
	utils.ApplyRegistrationWithSync(kubeUtil, radixClient, commonTestUtils, rrBuilder)

	// Check that secret has been created
	firstSecret, err := kubeUtil.CoreV1().Secrets(builders.GetAppNamespace(appName)).Get(context.Background(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(firstSecret.Data[defaults.GitPrivateKeySecretKey]), 1)

	// calling regenerate-deploy-key in order to delete secret
	regenerateParameters := &applicationModels.RegenerateDeployKeyAndSecretData{SharedSecret: "new shared secret"}
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/regenerate-deploy-key", appName), regenerateParameters)
	response := <-responseChannel
	assert.Equal(t, http.StatusNoContent, response.Code)

	// forcing resync of RR
	utils.ApplyRegistrationWithSync(kubeUtil, radixClient, commonTestUtils, rrBuilder)

	// Check that secret has been re-created and is different from first secret
	secondSecret, err := kubeUtil.CoreV1().Secrets(builders.GetAppNamespace(appName)).Get(context.Background(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(secondSecret.Data[defaults.GitPrivateKeySecretKey]), 1)
	assert.NotEqual(t, firstSecret.Data[defaults.GitPrivateKeySecretKey], secondSecret.Data[defaults.GitPrivateKeySecretKey])
}

func TestRegenerateDeployKey_PrivateKeyInParam_SavedPrivateKeyIsEqualToWebParam(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeUtil, radixClient, _, _ := setupTest(true, true)
	appName := "any-name"
	rrBuilder := builders.ARadixRegistration().WithName(appName).WithCloneURL("git@github.com:Equinor/my-app.git")

	// Creating RR and syncing it
	utils.ApplyRegistrationWithSync(kubeUtil, radixClient, commonTestUtils, rrBuilder)

	// make some valid private key
	deployKey, err := builders.GenerateDeployKey()
	assert.NoError(t, err)

	// calling regenerate-deploy-key in order to set secret
	regenerateParameters := &applicationModels.RegenerateDeployKeyAndSecretData{SharedSecret: "new shared secret", PrivateKey: deployKey.PrivateKey}
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/regenerate-deploy-key", appName), regenerateParameters)
	response := <-responseChannel
	assert.Equal(t, http.StatusNoContent, response.Code)

	// forcing resync of RR
	utils.ApplyRegistrationWithSync(kubeUtil, radixClient, commonTestUtils, rrBuilder)

	// Check that secret has been re-created and is equal to the one in the web parameter
	secret, err := kubeUtil.CoreV1().Secrets(builders.GetAppNamespace(appName)).Get(context.Background(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, deployKey.PrivateKey, string(secret.Data[defaults.GitPrivateKeySecretKey]))
}

func TestRegenerateDeployKey_InvalidKeyInParam_ErrorIsReturned(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeUtil, radixClient, _, _ := setupTest(true, true)
	appName := "any-name"
	rrBuilder := builders.ARadixRegistration().WithName(appName).WithCloneURL("git@github.com:Equinor/my-app.git")

	// Creating RR and syncing it
	utils.ApplyRegistrationWithSync(kubeUtil, radixClient, commonTestUtils, rrBuilder)

	// calling regenerate-deploy-key with invalid private key, expecting error
	regenerateParameters := &applicationModels.RegenerateDeployKeyAndSecretData{SharedSecret: "new shared secret", PrivateKey: "invalid key"}
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/regenerate-deploy-key", appName), regenerateParameters)
	response := <-responseChannel
	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func setStatusOfCloneJob(kubeclient kubernetes.Interface, appNamespace string, succeededStatus bool) {
	timeout := time.After(1 * time.Second)
	tick := time.Tick(200 * time.Millisecond)

	for {
		select {
		case <-timeout:
			return

		case <-tick:
			jobs, _ := kubeclient.BatchV1().Jobs(appNamespace).List(context.Background(), metav1.ListOptions{})
			if len(jobs.Items) > 0 {
				job := jobs.Items[0]

				if succeededStatus {
					job.Status.Succeeded = int32(1)
				} else {
					job.Status.Failed = int32(1)
				}

				kubeclient.BatchV1().Jobs(appNamespace).Update(context.Background(), &job, metav1.UpdateOptions{})
			}
		}
	}
}

func createRadixJob(commonTestUtils *commontest.Utils, appName, jobName string, started time.Time) {
	commonTestUtils.ApplyJob(
		builders.ARadixBuildDeployJob().
			WithAppName(appName).
			WithJobName(jobName).
			WithStatus(builders.NewJobStatusBuilder().
				WithCondition(v1.JobSucceeded).
				WithStarted(started.UTC()).
				WithSteps(
					builders.ACloneConfigStep().
						WithCondition(v1.JobSucceeded).
						WithStarted(started.UTC()).
						WithEnded(started.Add(time.Second*time.Duration(100))),
					builders.ARadixPipelineStep().
						WithCondition(v1.JobRunning).
						WithStarted(started.UTC()).
						WithEnded(started.Add(time.Second*time.Duration(100))))))
}

func getJobsInNamespace(radixclient *fake.Clientset, appNamespace string) ([]v1.RadixJob, error) {
	jobs, err := radixclient.RadixV1().RadixJobs(appNamespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return jobs.Items, nil
}

// anApplicationRegistration Constructor for application builder with test values
func anApplicationRegistration() applicationModels.ApplicationRegistrationBuilder {
	return applicationModels.NewApplicationRegistrationBuilder().
		WithName("my-app").
		WithRepository("https://github.com/Equinor/my-app").
		WithSharedSecret("AnySharedSecret").
		WithAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"}).
		WithReaderAdGroups([]string{"40e794dc-244c-4d0a-9f29-55fda1fe3972"}).
		WithCreator("a_test_user@equinor.com").
		WithConfigurationItem("2b0781a7db131784551ea1ea4b9619c9").
		WithConfigBranch("main")
}

func buildApplicationRegistrationRequest(applicationRegistration applicationModels.ApplicationRegistration, acknowledgeWarnings bool) *applicationModels.ApplicationRegistrationRequest {
	return &applicationModels.ApplicationRegistrationRequest{
		ApplicationRegistration: &applicationRegistration,
		AcknowledgeWarnings:     acknowledgeWarnings,
	}
}

type testApplicationHandlerFactory struct {
	config                  ApplicationHandlerConfig
	hasAccessToGetConfigMap hasAccessToGetConfigMapFunc
}

func newTestApplicationHandlerFactory(config ApplicationHandlerConfig, hasAccessToGetConfigMap hasAccessToGetConfigMapFunc) ApplicationHandlerFactory {
	return &testApplicationHandlerFactory{
		config:                  config,
		hasAccessToGetConfigMap: hasAccessToGetConfigMap,
	}
}

// Create creates a new ApplicationHandler
func (f *testApplicationHandlerFactory) Create(accounts models.Accounts) ApplicationHandler {
	return NewApplicationHandler(accounts, f.config, f.hasAccessToGetConfigMap)
}
