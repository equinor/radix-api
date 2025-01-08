package applications

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	certfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	applicationModels "github.com/equinor/radix-api/api/applications/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/metrics"
	mock2 "github.com/equinor/radix-api/api/metrics/mock"
	"github.com/equinor/radix-api/api/metrics/prometheus"
	"github.com/equinor/radix-api/api/metrics/prometheus/mock"
	controllertest "github.com/equinor/radix-api/api/test"
	"github.com/equinor/radix-api/api/utils"
	authnmock "github.com/equinor/radix-api/api/utils/token/mock"
	"github.com/equinor/radix-api/internal/config"
	"github.com/equinor/radix-api/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	jobPipeline "github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	clusterName     = "AnyClusterName"
	appAliasDNSZone = "app.dev.radix.equinor.com"
	egressIps       = "0.0.0.0"
	subscriptionId  = "12347718-c8f8-4995-bfbb-02655ff1f89c"
)

func setupTest(t *testing.T, requireAppConfigurationItem, requireAppADGroups bool) (*commontest.Utils, *controllertest.Utils, *kubefake.Clientset, *radixfake.Clientset, *kedafake.Clientset, *prometheusfake.Clientset, *secretproviderfake.Clientset, *certfake.Clientset) {
	return setupTestWithFactory(t, newTestApplicationHandlerFactory(
		config.Config{RequireAppConfigurationItem: requireAppConfigurationItem, RequireAppADGroups: requireAppADGroups},
		func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
			return true, nil
		},
	))
}

func setupTestWithFactory(t *testing.T, handlerFactory ApplicationHandlerFactory) (*commontest.Utils, *controllertest.Utils, *kubefake.Clientset, *radixfake.Clientset, *kedafake.Clientset, *prometheusfake.Clientset, *secretproviderfake.Clientset, *certfake.Clientset) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := radixfake.NewSimpleClientset()
	kedaClient := kedafake.NewSimpleClientset()
	prometheusclient := prometheusfake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	certClient := certfake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient)
	err := commonTestUtils.CreateClusterPrerequisites(clusterName, egressIps, subscriptionId)
	require.NoError(t, err)
	prometheusHandlerMock := createPrometheusHandlerMock(t, nil)

	// controllerTestUtils is used for issuing HTTP request and processing responses
	mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
	controllerTestUtils := controllertest.NewTestUtils(
		kubeclient,
		radixclient,
		kedaClient,
		secretproviderclient,
		certClient,
		mockValidator,
		NewApplicationController(
			func(_ context.Context, _ kubernetes.Interface, _ v1.RadixRegistration) (bool, error) {
				return true, nil
			}, handlerFactory, prometheusHandlerMock),
	)

	return &commonTestUtils, &controllerTestUtils, kubeclient, radixclient, kedaClient, prometheusclient, secretproviderclient, certClient
}

func createPrometheusHandlerMock(t *testing.T, mockHandler *func(handler *mock.MockQueryAPI)) *metrics.Handler {
	ctrl := gomock.NewController(t)

	promQueryApi := mock.NewMockQueryAPI(ctrl)
	promClient := prometheus.NewClient(promQueryApi)

	metricsHandler := metrics.NewHandler(promClient)
	if mockHandler != nil {
		(*mockHandler)(promQueryApi)
	} else {
		promQueryApi.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(&model.Vector{}, nil, nil)
	}
	return metricsHandler
}

func TestGetApplications_HasAccessToSomeRR(t *testing.T) {
	commonTestUtils, _, kubeclient, radixclient, kedaClient, _, secretproviderclient, certClient := setupTest(t, true, true)

	_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithCloneURL("git@github.com:Equinor/my-app.git"))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithCloneURL("git@github.com:Equinor/my-second-app.git").WithAdGroups([]string{"2"}).WithName("my-second-app"))
	require.NoError(t, err)

	t.Run("no access", func(t *testing.T) {
		prometheusHandlerMock := createPrometheusHandlerMock(t, nil)
		mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
		mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
		controllerTestUtils := controllertest.NewTestUtils(
			kubeclient,
			radixclient,
			kedaClient,
			secretproviderclient,
			certClient,
			mockValidator,
			NewApplicationController(
				func(_ context.Context, _ kubernetes.Interface, _ v1.RadixRegistration) (bool, error) {
					return false, nil
				}, newTestApplicationHandlerFactory(config.Config{RequireAppConfigurationItem: true, RequireAppADGroups: true},
					func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
						return true, nil
					}), prometheusHandlerMock))
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications")
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err = controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 0, len(applications))
	})

	t.Run("access to single app", func(t *testing.T) {
		prometheusHandlerMock := createPrometheusHandlerMock(t, nil)
		mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
		mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
		controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient, certClient, mockValidator, NewApplicationController(
			func(_ context.Context, _ kubernetes.Interface, rr v1.RadixRegistration) (bool, error) {
				return rr.GetName() == "my-second-app", nil
			}, newTestApplicationHandlerFactory(config.Config{RequireAppConfigurationItem: true, RequireAppADGroups: true},
				func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
					return true, nil
				}), prometheusHandlerMock))
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications")
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err = controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 1, len(applications))
	})

	t.Run("access to all app", func(t *testing.T) {
		prometheusHandlerMock := createPrometheusHandlerMock(t, nil)
		mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
		mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
		controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient, certClient, mockValidator, NewApplicationController(
			func(_ context.Context, _ kubernetes.Interface, _ v1.RadixRegistration) (bool, error) {
				return true, nil
			}, newTestApplicationHandlerFactory(config.Config{RequireAppConfigurationItem: true, RequireAppADGroups: true},
				func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
					return true, nil
				}), prometheusHandlerMock))
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications")
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err = controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 2, len(applications))
	})
}

func TestGetApplications_WithFilterOnSSHRepo_Filter(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)
	_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithCloneURL("git@github.com:Equinor/my-app.git"))
	require.NoError(t, err)

	// Test
	t.Run("matching repo", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications?sshRepo=%s", url.QueryEscape("git@github.com:Equinor/my-app.git")))
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err = controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 1, len(applications))
	})

	t.Run("not matching repo", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications?sshRepo=%s", url.QueryEscape("git@github.com:Equinor/my-app2.git")))
		response := <-responseChannel

		applications := make([]*applicationModels.ApplicationSummary, 0)
		err = controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 0, len(applications))
	})

	t.Run("no filter", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications")
		response := <-responseChannel

		applications := make([]*applicationModels.ApplicationSummary, 0)
		err = controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 1, len(applications))
	})
}

func TestSearchApplicationsPost(t *testing.T) {
	// Setup
	commonTestUtils, _, kubeclient, radixclient, kedaClient, _, secretproviderclient, certClient := setupTest(t, true, true)
	appNames := []string{"app-1", "app-2"}

	for _, appName := range appNames {
		_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration().WithName(appName))
		require.NoError(t, err)
	}

	app2Job1Started, _ := radixutils.ParseTimestamp("2018-11-12T12:30:14Z")
	err := createRadixJob(commonTestUtils, appNames[1], "app-2-job-1", app2Job1Started)
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		builders.
			ARadixDeployment().
			WithAppName(appNames[1]).
			WithComponent(
				builders.
					NewDeployComponentBuilder(),
			),
	)
	require.NoError(t, err)

	prometheusHandlerMock := createPrometheusHandlerMock(t, nil)
	mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
	controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient, certClient, mockValidator, NewApplicationController(
		func(_ context.Context, _ kubernetes.Interface, _ v1.RadixRegistration) (bool, error) {
			return true, nil
		}, newTestApplicationHandlerFactory(config.Config{RequireAppConfigurationItem: true, RequireAppADGroups: true},
			func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
				return true, nil
			}), prometheusHandlerMock))

	// Tests
	t.Run("search for "+appNames[0], func(t *testing.T) {
		params := applicationModels.ApplicationsSearchRequest{Names: []string{appNames[0]}}
		responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications/_search", &params)
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err := controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 1, len(applications))
		assert.Equal(t, appNames[0], applications[0].Name)
	})

	t.Run("search for both apps", func(t *testing.T) {
		params := applicationModels.ApplicationsSearchRequest{Names: appNames}
		responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications/_search", &params)
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err := controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 2, len(applications))
	})

	t.Run("empty appname list", func(t *testing.T) {
		params := applicationModels.ApplicationsSearchRequest{Names: []string{}}
		responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications/_search", &params)
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err := controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 0, len(applications))
	})

	t.Run("search for "+appNames[1]+" - with includeFields 'LatestJobSummary'", func(t *testing.T) {
		params := applicationModels.ApplicationsSearchRequest{
			Names: []string{appNames[1]},
			IncludeFields: applicationModels.ApplicationSearchIncludeFields{
				LatestJobSummary: true,
			},
		}
		responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications/_search", &params)
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err := controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 1, len(applications))
		assert.Equal(t, appNames[1], applications[0].Name)
		assert.NotNil(t, applications[0].LatestJob)
		assert.Nil(t, applications[0].EnvironmentActiveComponents)
	})

	t.Run("search for "+appNames[1]+" - with includeFields 'EnvironmentActiveComponents'", func(t *testing.T) {
		params := applicationModels.ApplicationsSearchRequest{
			Names: []string{appNames[1]},
			IncludeFields: applicationModels.ApplicationSearchIncludeFields{
				EnvironmentActiveComponents: true,
			},
		}
		responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications/_search", &params)
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err := controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 1, len(applications))
		assert.Equal(t, appNames[1], applications[0].Name)
		assert.Nil(t, applications[0].LatestJob)
		assert.NotNil(t, applications[0].EnvironmentActiveComponents)
	})

	t.Run("search for "+appNames[0]+" - no access", func(t *testing.T) {
		prometheusHandlerMock := createPrometheusHandlerMock(t, nil)
		mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
		mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
		controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient, certClient, mockValidator, NewApplicationController(
			func(_ context.Context, _ kubernetes.Interface, _ v1.RadixRegistration) (bool, error) {
				return false, nil
			}, newTestApplicationHandlerFactory(config.Config{RequireAppConfigurationItem: true, RequireAppADGroups: true},
				func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
					return true, nil
				}), prometheusHandlerMock))
		params := applicationModels.ApplicationsSearchRequest{Names: []string{appNames[0]}}
		responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications/_search", &params)
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err := controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 0, len(applications))
	})
}

func TestSearchApplicationsPost_WithJobs_ShouldOnlyHaveLatest(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _, _, _ := setupTest(t, true, true)
	apps := []applicationModels.Application{
		{Name: "app-1", Jobs: []*jobModels.JobSummary{
			{Name: "app-1-job-1", Started: createTime("2018-11-12T11:45:26Z")},
		}},
		{Name: "app-2", Jobs: []*jobModels.JobSummary{
			{Name: "app-2-job-1", Started: createTime("2018-11-12T12:30:14Z")},
			{Name: "app-2-job-2", Started: createTime("2018-11-20T09:00:00Z")},
			{Name: "app-2-job-3", Started: createTime("2018-11-20T09:00:01Z")},
		}},
		{Name: "app-3"},
	}

	for _, app := range apps {
		commontest.CreateAppNamespace(kubeclient, app.Name)
		_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
			WithName(app.Name))
		require.NoError(t, err)

		for _, job := range app.Jobs {
			if job.Started != nil {
				err = createRadixJob(commonTestUtils, app.Name, job.Name, *job.Started)
				require.NoError(t, err)
			}
		}
	}

	// Test
	params := applicationModels.ApplicationsSearchRequest{
		Names: slice.Reduce(apps, []string{}, func(names []string, app applicationModels.Application) []string { return append(names, app.Name) }),
		IncludeFields: applicationModels.ApplicationSearchIncludeFields{
			LatestJobSummary: true,
		},
	}
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications/_search", &params)
	response := <-responseChannel

	applications := make([]*applicationModels.ApplicationSummary, 0)
	err := controllertest.GetResponseBody(response, &applications)
	require.NoError(t, err)

	for _, application := range applications {
		if app, _ := slice.FindFirst(apps, func(app applicationModels.Application) bool { return strings.EqualFold(application.Name, app.Name) }); app.Jobs != nil {
			assert.NotNil(t, application.LatestJob)
			assert.Equal(t, app.Jobs[len(app.Jobs)-1].Name, application.LatestJob.Name)
		} else {
			assert.Nil(t, application.LatestJob)
		}
	}
}

func TestSearchApplicationsGet(t *testing.T) {
	// Setup
	commonTestUtils, _, kubeclient, radixclient, kedaClient, _, secretproviderclient, certClient := setupTest(t, true, true)
	appNames := []string{"app-1", "app-2"}

	for _, appName := range appNames {
		_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration().WithName(appName))
		require.NoError(t, err)
	}

	app2Job1Started, _ := radixutils.ParseTimestamp("2018-11-12T12:30:14Z")
	err := createRadixJob(commonTestUtils, appNames[1], "app-2-job-1", app2Job1Started)
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		builders.
			ARadixDeployment().
			WithAppName(appNames[1]).
			WithComponent(
				builders.
					NewDeployComponentBuilder(),
			),
	)
	require.NoError(t, err)

	prometheusHandlerMock := createPrometheusHandlerMock(t, nil)
	mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
	controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient, certClient, mockValidator, NewApplicationController(
		func(_ context.Context, _ kubernetes.Interface, _ v1.RadixRegistration) (bool, error) {
			return true, nil
		}, newTestApplicationHandlerFactory(config.Config{RequireAppConfigurationItem: true, RequireAppADGroups: true},
			func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
				return true, nil
			}), prometheusHandlerMock))

	// Tests
	t.Run("search for "+appNames[0], func(t *testing.T) {
		params := "apps=" + appNames[0]
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications/_search?"+params)
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err := controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 1, len(applications))
		assert.Equal(t, appNames[0], applications[0].Name)
	})

	t.Run("search for both apps", func(t *testing.T) {
		params := "apps=" + strings.Join(appNames, ",")
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications/_search?"+params)
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err := controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 2, len(applications))
	})

	t.Run("empty appname list", func(t *testing.T) {
		params := "apps="
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications/_search?"+params)
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err := controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 0, len(applications))
	})

	t.Run("search for "+appNames[1]+" - with includeFields 'LatestJobSummary'", func(t *testing.T) {
		params := []string{"apps=" + appNames[1], "includeLatestJobSummary=true"}
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications/_search?"+strings.Join(params, "&"))
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err := controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 1, len(applications))
		assert.Equal(t, appNames[1], applications[0].Name)
		assert.NotNil(t, applications[0].LatestJob)
		assert.Nil(t, applications[0].EnvironmentActiveComponents)
	})

	t.Run("search for "+appNames[1]+" - with includeFields 'EnvironmentActiveComponents'", func(t *testing.T) {
		params := []string{"apps=" + appNames[1], "includeEnvironmentActiveComponents=true"}
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications/_search?"+strings.Join(params, "&"))
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err := controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 1, len(applications))
		assert.Equal(t, appNames[1], applications[0].Name)
		assert.Nil(t, applications[0].LatestJob)
		assert.NotNil(t, applications[0].EnvironmentActiveComponents)
	})

	t.Run("search for "+appNames[0]+" - no access", func(t *testing.T) {
		prometheusHandlerMock := createPrometheusHandlerMock(t, nil)
		mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
		mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
		controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient, certClient, mockValidator, NewApplicationController(
			func(_ context.Context, _ kubernetes.Interface, _ v1.RadixRegistration) (bool, error) {
				return false, nil
			}, newTestApplicationHandlerFactory(config.Config{RequireAppConfigurationItem: true, RequireAppADGroups: true},
				func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
					return true, nil
				}), prometheusHandlerMock))
		params := "apps=" + appNames[0]
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications/_search?"+params)
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err := controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 0, len(applications))
	})
}

func TestSearchApplicationsGet_WithJobs_ShouldOnlyHaveLatest(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _, _, _ := setupTest(t, true, true)
	apps := []applicationModels.Application{
		{Name: "app-1", Jobs: []*jobModels.JobSummary{
			{Name: "app-1-job-1", Started: createTime("2018-11-12T11:45:26Z")},
		}},
		{Name: "app-2", Jobs: []*jobModels.JobSummary{
			{Name: "app-2-job-1", Started: createTime("2018-11-12T12:30:14Z")},
			{Name: "app-2-job-2", Started: createTime("2018-11-20T09:00:00Z")},
			{Name: "app-2-job-3", Started: createTime("2018-11-20T09:00:01Z")},
		}},
		{Name: "app-3"},
	}

	for _, app := range apps {
		commontest.CreateAppNamespace(kubeclient, app.Name)
		_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
			WithName(app.Name))
		require.NoError(t, err)

		for _, job := range app.Jobs {
			if job.Started != nil {
				err = createRadixJob(commonTestUtils, app.Name, job.Name, *job.Started)
				require.NoError(t, err)
			}
		}
	}

	// Test
	params := []string{
		"apps=" + strings.Join(slice.Reduce(apps, []string{}, func(names []string, app applicationModels.Application) []string { return append(names, app.Name) }), ","),
		"includeLatestJobSummary=true",
	}
	responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications/_search?"+strings.Join(params, "&"))
	response := <-responseChannel

	applications := make([]*applicationModels.ApplicationSummary, 0)
	err := controllertest.GetResponseBody(response, &applications)
	require.NoError(t, err)

	for _, application := range applications {
		if app, _ := slice.FindFirst(apps, func(app applicationModels.Application) bool { return strings.EqualFold(application.Name, app.Name) }); app.Jobs != nil {
			assert.NotNil(t, application.LatestJob)
			assert.Equal(t, app.Jobs[len(app.Jobs)-1].Name, application.LatestJob.Name)
		} else {
			assert.Nil(t, application.LatestJob)
		}
	}
}

func TestCreateApplication_NoName_ValidationError(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)

	// Test
	parameters := buildApplicationRegistrationRequest(
		anApplicationRegistration().WithName("").Build(),
		false,
	)
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, "Error: app name is empty: resource name cannot be empty", errorResponse.Message)
}

func TestCreateApplication_WhenRequiredConfigurationItemIsNotSet_ReturnError(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)

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
	expectedError := radixvalidators.ResourceNameCannotBeEmptyErrorWithMessage("configuration item")
	assert.Equal(t, fmt.Sprintf("Error: %v", expectedError), errorResponse.Message)
}

func TestCreateApplication_WhenOptionalConfigurationItemIsNotSet_ReturnSuccess(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, false, true)

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
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)

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
	expectedError := radixvalidators.ResourceNameCannotBeEmptyErrorWithMessage("AD groups")
	assert.Equal(t, fmt.Sprintf("Error: %v", expectedError), errorResponse.Message)
}

func TestCreateApplication_WhenOptionalAdGroupsIsNotSet_ReturnSuccess(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, false)

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
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)

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
	expectedError := radixvalidators.ResourceNameCannotBeEmptyErrorWithMessage("branch name")
	assert.Equal(t, fmt.Sprintf("Error: %v", expectedError), errorResponse.Message)
}

func TestCreateApplication_WhenConfigBranchIsInvalid_ReturnError(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)

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
	expectedError := radixvalidators.InvalidConfigBranchNameWithMessage(configBranch)
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
			_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)

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
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)

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
	err := controllertest.GetResponseBody(response, &applicationRegistrationUpsertResponse)
	assert.NoError(t, err)
	assert.NotEmpty(t, applicationRegistrationUpsertResponse.Warnings)
	assert.Contains(t, applicationRegistrationUpsertResponse.Warnings, "Repository is used in other application(s)")
}

func TestCreateApplication_DuplicateRepoWithAcknowledgeWarning_ShouldSuccess(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)

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
	err := controllertest.GetResponseBody(response, &applicationRegistrationUpsertResponse)
	require.NoError(t, err)
	assert.Empty(t, applicationRegistrationUpsertResponse.Warnings)
	assert.NotEmpty(t, applicationRegistrationUpsertResponse.ApplicationRegistration)
}

func TestGetApplication_AllFieldsAreSet(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)

	adGroups, adUsers := []string{uuid.New().String()}, []string{uuid.New().String()}
	readerAdGroups, readerAdUsers := []string{uuid.New().String()}, []string{uuid.New().String()}
	parameters := buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-name").
			WithRepository("https://github.com/Equinor/any-repo").
			WithSharedSecret("Any secret").
			WithAdGroups(adGroups).
			WithAdUsers(adUsers).
			WithReaderAdGroups(readerAdGroups).
			WithReaderAdUsers(readerAdUsers).
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
	err := controllertest.GetResponseBody(response, &application)
	assert.NoError(t, err)

	assert.Equal(t, "https://github.com/Equinor/any-repo", application.Registration.Repository)
	assert.Equal(t, "Any secret", application.Registration.SharedSecret)
	assert.Equal(t, adGroups, application.Registration.AdGroups)
	assert.Equal(t, adUsers, application.Registration.AdUsers)
	assert.Equal(t, readerAdGroups, application.Registration.ReaderAdGroups)
	assert.Equal(t, readerAdUsers, application.Registration.ReaderAdUsers)
	assert.Equal(t, "test-principal", application.Registration.Creator)
	assert.Equal(t, "abranch", application.Registration.ConfigBranch)
	assert.Equal(t, "a/custom-radixconfig.yaml", application.Registration.RadixConfigFullName)
	assert.Equal(t, "ci", application.Registration.ConfigurationItem)
}

func TestGetApplication_WithJobs(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _, _, _ := setupTest(t, true, true)
	_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName("any-name"))
	require.NoError(t, err)

	commontest.CreateAppNamespace(kubeclient, "any-name")
	app1Job1Started, _ := radixutils.ParseTimestamp("2018-11-12T11:45:26Z")
	app1Job2Started, _ := radixutils.ParseTimestamp("2018-11-12T12:30:14Z")
	app1Job3Started, _ := radixutils.ParseTimestamp("2018-11-20T09:00:00Z")

	err = createRadixJob(commonTestUtils, "any-name", "any-name-job-1", app1Job1Started)
	require.NoError(t, err)
	err = createRadixJob(commonTestUtils, "any-name", "any-name-job-2", app1Job2Started)
	require.NoError(t, err)
	err = createRadixJob(commonTestUtils, "any-name", "any-name-job-3", app1Job3Started)
	require.NoError(t, err)

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response := <-responseChannel

	application := applicationModels.Application{}
	err = controllertest.GetResponseBody(response, &application)
	require.NoError(t, err)
	assert.Equal(t, 3, len(application.Jobs))
}

func TestGetApplication_WithEnvironments(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, radix, _, _, _, _ := setupTest(t, true, true)

	anyAppName := "any-app"
	anyOrphanedEnvironment := "feature"

	_, err := commonTestUtils.ApplyRegistration(builders.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)

	_, err = commonTestUtils.ApplyApplication(builders.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment("dev", "master").
		WithEnvironment("prod", "release"))
	require.NoError(t, err)

	_, err = commonTestUtils.ApplyDeployment(context.Background(), builders.
		NewDeploymentBuilder().
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithImageTag("someimageindev"))
	require.NoError(t, err)

	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		builders.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyOrphanedEnvironment).
			WithImageTag("someimageinfeature"))
	require.NoError(t, err)

	// Set RE statuses
	devRe, err := radix.RadixV1().RadixEnvironments().Get(context.Background(), builders.GetEnvironmentNamespace(anyAppName, "dev"), metav1.GetOptions{})
	require.NoError(t, err)
	devRe.Status.Reconciled = metav1.Now()
	_, err = radix.RadixV1().RadixEnvironments().UpdateStatus(context.Background(), devRe, metav1.UpdateOptions{})
	require.NoError(t, err)
	prodRe, err := radix.RadixV1().RadixEnvironments().Get(context.Background(), builders.GetEnvironmentNamespace(anyAppName, "prod"), metav1.GetOptions{})
	require.NoError(t, err)
	prodRe.Status.Reconciled = metav1.Time{}
	_, err = radix.RadixV1().RadixEnvironments().UpdateStatus(context.Background(), prodRe, metav1.UpdateOptions{})
	require.NoError(t, err)

	orphanedRe, _ := commonTestUtils.ApplyEnvironment(builders.
		NewEnvironmentBuilder().
		WithAppLabel().
		WithAppName(anyAppName).
		WithEnvironmentName(anyOrphanedEnvironment))
	orphanedRe.Status.Reconciled = metav1.Now()
	orphanedRe.Status.Orphaned = true
	orphanedRe.Status.OrphanedTimestamp = pointers.Ptr(metav1.Now())
	_, err = radix.RadixV1().RadixEnvironments().Update(context.Background(), orphanedRe, metav1.UpdateOptions{})
	require.NoError(t, err)

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", anyAppName))
	response := <-responseChannel

	application := applicationModels.Application{}
	err = controllertest.GetResponseBody(response, &application)
	require.NoError(t, err)
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
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)

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
	err := controllertest.GetResponseBody(response, &registrationUpsertResponse)
	require.NoError(t, err)
	assert.NotEmpty(t, registrationUpsertResponse.Warnings)
}

func TestUpdateApplication_DuplicateRepoWithAcknowledgeWarnings_ShouldSuccess(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)

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
	err := controllertest.GetResponseBody(response, &registrationUpsertResponse)
	require.NoError(t, err)
	assert.Empty(t, registrationUpsertResponse.Warnings)
	assert.NotNil(t, registrationUpsertResponse.ApplicationRegistration)
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestUpdateApplication_MismatchingNameOrNotExists_ShouldFailAsIllegalOperation(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)

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
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)

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
	err := controllertest.GetResponseBody(response, &applicationRegistrationUpsertResponse)
	require.NoError(t, err)
	assert.NotEmpty(t, applicationRegistrationUpsertResponse.ApplicationRegistration)
	assert.Equal(t, newRepository, applicationRegistrationUpsertResponse.ApplicationRegistration.Repository)

	// Test SharedSecret
	newSharedSecret := "Any shared secret"
	builder = builder.
		WithSharedSecret(newSharedSecret)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), buildApplicationRegistrationRequest(builder.Build(), false))
	response = <-responseChannel
	applicationRegistrationUpsertResponse = applicationModels.ApplicationRegistrationUpsertResponse{}
	err = controllertest.GetResponseBody(response, &applicationRegistrationUpsertResponse)
	require.NoError(t, err)
	assert.Equal(t, newSharedSecret, applicationRegistrationUpsertResponse.ApplicationRegistration.SharedSecret)

	// Test WBS
	newWbs := "new.wbs.code"
	builder = builder.
		WithWBS(newWbs)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), buildApplicationRegistrationRequest(builder.Build(), false))
	response = <-responseChannel
	applicationRegistrationUpsertResponse = applicationModels.ApplicationRegistrationUpsertResponse{}
	err = controllertest.GetResponseBody(response, &applicationRegistrationUpsertResponse)
	require.NoError(t, err)
	assert.Equal(t, newWbs, applicationRegistrationUpsertResponse.ApplicationRegistration.WBS)

	// Test ConfigBranch
	newConfigBranch := "newcfgbranch"
	builder = builder.
		WithConfigBranch(newConfigBranch)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), buildApplicationRegistrationRequest(builder.Build(), false))
	response = <-responseChannel
	applicationRegistrationUpsertResponse = applicationModels.ApplicationRegistrationUpsertResponse{}
	err = controllertest.GetResponseBody(response, &applicationRegistrationUpsertResponse)
	require.NoError(t, err)
	assert.Equal(t, newConfigBranch, applicationRegistrationUpsertResponse.ApplicationRegistration.ConfigBranch)

	// Test ConfigurationItem
	newConfigurationItem := "newci"
	builder = builder.
		WithConfigurationItem(newConfigurationItem)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), buildApplicationRegistrationRequest(builder.Build(), false))
	response = <-responseChannel
	applicationRegistrationUpsertResponse = applicationModels.ApplicationRegistrationUpsertResponse{}
	err = controllertest.GetResponseBody(response, &applicationRegistrationUpsertResponse)
	require.NoError(t, err)
	assert.Equal(t, newConfigurationItem, applicationRegistrationUpsertResponse.ApplicationRegistration.ConfigurationItem)
}

func TestModifyApplication_AbleToSetField(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)

	builder := anApplicationRegistration().
		WithName("any-name").
		WithRepository("https://github.com/Equinor/a-repo").
		WithSharedSecret("").
		WithAdGroups([]string{uuid.New().String()}).
		WithAdUsers([]string{uuid.New().String()}).
		WithReaderAdGroups([]string{uuid.New().String()}).
		WithReaderAdUsers([]string{uuid.New().String()}).
		WithOwner("AN_OWNER@equinor.com").
		WithWBS("T.O123A.AZ.45678").
		WithConfigBranch("main1").
		WithConfigurationItem("ci-initial")
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", buildApplicationRegistrationRequest(builder.Build(), false))
	<-responseChannel

	// Test
	anyNewAdGroup := []string{uuid.New().String()}
	anyNewAdUser := []string{uuid.New().String()}
	patchRequest := applicationModels.ApplicationRegistrationPatchRequest{
		ApplicationRegistrationPatch: &applicationModels.ApplicationRegistrationPatch{
			AdGroups: &anyNewAdGroup,
			AdUsers:  &anyNewAdUser,
		},
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel

	application := applicationModels.Application{}
	err := controllertest.GetResponseBody(response, &application)
	require.NoError(t, err)
	assert.Equal(t, anyNewAdGroup, application.Registration.AdGroups)
	assert.Equal(t, anyNewAdUser, application.Registration.AdUsers)
	assert.Equal(t, "AN_OWNER@equinor.com", application.Registration.Owner)
	assert.Equal(t, "T.O123A.AZ.45678", application.Registration.WBS)
	assert.Equal(t, "main1", application.Registration.ConfigBranch)
	assert.Equal(t, "ci-initial", application.Registration.ConfigurationItem)

	// Test
	anyNewReaderAdGroup := []string{uuid.New().String()}
	anyNewReaderAdUser := []string{uuid.New().String()}
	patchRequest = applicationModels.ApplicationRegistrationPatchRequest{
		ApplicationRegistrationPatch: &applicationModels.ApplicationRegistrationPatch{
			ReaderAdGroups: &anyNewReaderAdGroup,
			ReaderAdUsers:  &anyNewReaderAdUser,
		},
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	<-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)
	err = controllertest.GetResponseBody(response, &application)
	require.NoError(t, err)
	assert.Equal(t, anyNewReaderAdGroup, application.Registration.ReaderAdGroups)
	assert.Equal(t, anyNewReaderAdUser, application.Registration.ReaderAdUsers)

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

	err = controllertest.GetResponseBody(response, &application)
	require.NoError(t, err)
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

	err = controllertest.GetResponseBody(response, &application)
	require.NoError(t, err)
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

	err = controllertest.GetResponseBody(response, &application)
	require.NoError(t, err)
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

	err = controllertest.GetResponseBody(response, &application)
	require.NoError(t, err)
	assert.Equal(t, anyNewConfigurationItem, application.Registration.ConfigurationItem)
}

func TestModifyApplication_AbleToUpdateRepository(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)

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
	err := controllertest.GetResponseBody(response, &application)
	require.NoError(t, err)
	assert.Equal(t, anyNewRepo, application.Registration.Repository)
}

func TestModifyApplication_ConfigBranchSetToFallbackHack(t *testing.T) {
	// Setup
	appName := "any-name"
	_, controllerTestUtils, _, radixClient, _, _, _, _ := setupTest(t, true, true)
	rr := builders.ARadixRegistration().
		WithName(appName).
		WithConfigurationItem("any").
		WithConfigBranch("")
	_, err := radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr.BuildRR(), metav1.CreateOptions{})
	require.NoError(t, err)

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
	err = controllertest.GetResponseBody(response, &application)
	require.NoError(t, err)
	assert.Equal(t, applicationconfig.ConfigBranchFallback, application.Registration.ConfigBranch)
}

func TestModifyApplication_IgnoreRequireCIValidationWhenRequiredButCurrentIsEmpty(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, radixClient, _, _, _, _ := setupTest(t, true, true)

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
	_, controllerTestUtils, _, radixClient, _, _, _, _ := setupTest(t, true, true)

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

func TestModifyApplication_UpdateADGroupValidation(t *testing.T) {
	type scenario struct {
		requireAppADGroups   bool
		name                 string
		hasAccessToAdGroups  bool
		expectedResponseCode int
		adminAdGroups        []string
	}
	scenarios := []scenario{
		{
			name:                 "Require ADGroups, has groups and has access to them",
			requireAppADGroups:   true,
			adminAdGroups:        []string{"e654757d-c789-11e8-bbad-045000000001"},
			hasAccessToAdGroups:  true,
			expectedResponseCode: http.StatusOK,
		},
		{
			name:                 "Require ADGroups, has no groups and has access to them",
			requireAppADGroups:   true,
			adminAdGroups:        []string{},
			hasAccessToAdGroups:  true,
			expectedResponseCode: http.StatusBadRequest,
		},
		{
			name:                 "Require ADGroups, has groups and has no access to them",
			requireAppADGroups:   true,
			adminAdGroups:        []string{"e654757d-c789-11e8-bbad-045000000001"},
			hasAccessToAdGroups:  false,
			expectedResponseCode: http.StatusBadRequest,
		},
		{
			name:                 "Require ADGroups, has no groups and has no access to them",
			requireAppADGroups:   true,
			adminAdGroups:        []string{},
			hasAccessToAdGroups:  false,
			expectedResponseCode: http.StatusBadRequest,
		},
		{
			name:                 "Not require ADGroups, has groups and has access to them",
			requireAppADGroups:   false,
			adminAdGroups:        []string{"e654757d-c789-11e8-bbad-045000000001"},
			hasAccessToAdGroups:  true,
			expectedResponseCode: http.StatusOK,
		},
		{
			name:                 "Not require ADGroups, has no groups and has access to them",
			requireAppADGroups:   false,
			adminAdGroups:        []string{},
			hasAccessToAdGroups:  true,
			expectedResponseCode: http.StatusOK,
		},
		{
			name:                 "Not require ADGroups, has groups and has no access to them",
			requireAppADGroups:   false,
			adminAdGroups:        []string{"e654757d-c789-11e8-bbad-045000000001"},
			hasAccessToAdGroups:  false,
			expectedResponseCode: http.StatusBadRequest,
		},
		{
			name:                 "Not require ADGroups, has no groups and has no access to them",
			requireAppADGroups:   false,
			adminAdGroups:        []string{},
			hasAccessToAdGroups:  false,
			expectedResponseCode: http.StatusOK,
		},
	}

	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			_, controllerTestUtils, _, radixClient, _, _, _, _ := setupTestWithFactory(t, newTestApplicationHandlerFactory(
				config.Config{RequireAppConfigurationItem: true, RequireAppADGroups: ts.requireAppADGroups},
				func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
					return ts.hasAccessToAdGroups, nil
				},
			))

			rr, err := anApplicationRegistration().
				WithName("any-name").
				WithAdGroups([]string{"e654757d-c789-11e8-bbad-045007777777"}).
				BuildRR()
			require.NoError(t, err)
			_, err = radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
			require.NoError(t, err)

			// Test
			patchRequest := applicationModels.ApplicationRegistrationPatchRequest{
				ApplicationRegistrationPatch: &applicationModels.ApplicationRegistrationPatch{
					AdGroups: pointers.Ptr(ts.adminAdGroups),
				},
			}

			responseChannel := controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
			response := <-responseChannel
			assert.Equal(t, ts.expectedResponseCode, response.Code, fmt.Sprintf("Expected response code %d but got %d", ts.expectedResponseCode, response.Code))
		})
	}
}

func TestHandleTriggerPipeline_ForNonMappedAndMappedAndMagicBranchEnvironment_JobIsNotCreatedForUnmapped(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)
	anyAppName := "any-app"
	configBranch := "magic"

	rr := builders.ARadixRegistration().WithConfigBranch(configBranch).WithAdGroups([]string{"adminGroup"})
	_, err := commonTestUtils.ApplyApplication(builders.
		ARadixApplication().
		WithRadixRegistration(rr).
		WithAppName(anyAppName).
		WithEnvironment("dev", "dev").
		WithEnvironment("prod", "release"),
	)
	require.NoError(t, err)

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
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)

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
	err := controllertest.GetResponseBody(response, &jobSummary)
	require.NoError(t, err)
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
			_, controllerTestUtils, _, radixclient, _, _, _, _ := setupTest(t, true, true)
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

	const (
		appName         = "an-app"
		commitId        = "475f241c-478b-49da-adfb-3c336aaab8d2"
		fromEnvironment = "origin"
		toEnvironment   = "target"
	)

	type scenario struct {
		name                      string
		existingDeploymentName    string
		requestedDeploymentName   string
		expectedDeploymentName    string
		expectedResponseBodyError string
		expectedResponseCode      int
	}
	scenarios := []scenario{
		{
			name:                    "existing full deployment name",
			existingDeploymentName:  "abc-deployment",
			requestedDeploymentName: "abc-deployment",
			expectedDeploymentName:  "abc-deployment",
			expectedResponseCode:    200,
		},
		{
			name:                    "existing short deployment name",
			existingDeploymentName:  "abc-deployment",
			requestedDeploymentName: "deployment",
			expectedDeploymentName:  "abc-deployment",
			expectedResponseCode:    200,
		},
		{
			name:                      "non existing short deployment name",
			existingDeploymentName:    "abc-deployment",
			requestedDeploymentName:   "other-name",
			expectedDeploymentName:    "",
			expectedResponseBodyError: "invalid or not existing deployment name",
			expectedResponseCode:      400,
		},
	}

	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			commonTestUtils, controllerTestUtils, _, radixclient, _, _, _, _ := setupTest(t, true, true)
			_, err := commonTestUtils.ApplyDeployment(
				context.Background(),
				builders.
					ARadixDeployment().
					WithAppName(appName).
					WithDeploymentName(ts.existingDeploymentName).
					WithEnvironment(fromEnvironment).
					WithLabel(kube.RadixCommitLabel, commitId).
					WithCondition(v1.DeploymentInactive))
			require.NoError(t, err)

			parameters := applicationModels.PipelineParametersPromote{
				FromEnvironment: fromEnvironment,
				ToEnvironment:   toEnvironment,
				DeploymentName:  ts.requestedDeploymentName,
			}

			registerAppParam := buildApplicationRegistrationRequest(anApplicationRegistration().WithName(appName).Build(), false)
			<-controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", registerAppParam)
			responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", appName, v1.Promote), parameters)
			response := <-responseChannel
			assert.Equal(t, ts.expectedResponseCode, response.Code)
			if ts.expectedResponseCode != 200 {
				assert.NotNil(t, response.Body, "Empty respond body")
				type RespondBody struct {
					Type    string `json:"type"`
					Message string `json:"message"`
					Error   string `json:"error"`
				}
				body := RespondBody{}
				err = json.Unmarshal(response.Body.Bytes(), &body)
				require.NoError(t, err)
				require.Equal(t, ts.expectedResponseBodyError, body.Error, "invalid respond error")

			} else {
				appNamespace := fmt.Sprintf("%s-app", appName)
				jobs, err := getJobsInNamespace(radixclient, appNamespace)
				require.NoError(t, err)
				require.Len(t, jobs, 1)
				assert.Equal(t, jobs[0].Spec.Promote.FromEnvironment, fromEnvironment)
				assert.Equal(t, jobs[0].Spec.Promote.ToEnvironment, toEnvironment)
				assert.Equal(t, ts.expectedDeploymentName, jobs[0].Spec.Promote.DeploymentName)
				assert.Equal(t, jobs[0].Spec.Promote.CommitID, commitId)
			}
		})
	}
}

func TestDeleteApplication_ApplicationIsDeleted(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)

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
	commonTestUtils, controllerTestUtils, client, radixclient, kedaClient, promClient, secretproviderclient, certClient := setupTest(t, true, true)
	err := utils.ApplyDeploymentWithSync(client, radixclient, kedaClient, promClient, commonTestUtils, secretproviderclient, certClient, builders.ARadixDeployment().
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
	require.NoError(t, err)

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-app"))
	response := <-responseChannel

	application := applicationModels.Application{}
	err = controllertest.GetResponseBody(response, &application)
	require.NoError(t, err)

	assert.NotNil(t, application.AppAlias)
	assert.Equal(t, "frontend", application.AppAlias.ComponentName)
	assert.Equal(t, "prod", application.AppAlias.EnvironmentName)
	assert.Equal(t, fmt.Sprintf("%s.%s", "any-app", appAliasDNSZone), application.AppAlias.URL)
}

func TestListPipeline_ReturnsAvailablePipelines(t *testing.T) {
	supportedPipelines := jobPipeline.GetSupportedPipelines()

	// Setup
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)
	_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName("some-app").
		WithPublicKey("some-public-key").
		WithPrivateKey("some-private-key"))
	require.NoError(t, err)

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/pipelines", "some-app"))
	response := <-responseChannel

	pipelines := make([]string, 0)
	err = controllertest.GetResponseBody(response, &pipelines)
	require.NoError(t, err)
	assert.Equal(t, len(supportedPipelines), len(pipelines))
}

func TestRegenerateDeployKey_WhenApplicationNotExist_Fail(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)

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
	err := controllertest.GetResponseBody(response, &deployKeyAndSecret)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Empty(t, deployKeyAndSecret.PublicDeployKey)
	assert.Empty(t, deployKeyAndSecret.SharedSecret)
}

func TestRegenerateDeployKey_NoSecretInParam_SecretIsReCreated(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeUtil, radixClient, kedaClient, _, _, _ := setupTest(t, true, true)
	appName := "any-name"
	rrBuilder := builders.ARadixRegistration().WithName(appName).WithCloneURL("git@github.com:Equinor/my-app.git")

	// Creating RR and syncing it
	err := utils.ApplyRegistrationWithSync(kubeUtil, radixClient, kedaClient, commonTestUtils, rrBuilder)
	require.NoError(t, err)

	// Check that secret has been created
	firstSecret, err := kubeUtil.CoreV1().Secrets(builders.GetAppNamespace(appName)).Get(context.Background(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(firstSecret.Data[defaults.GitPrivateKeySecretKey]), 1)

	// calling regenerate-deploy-key in order to delete secret
	regenerateParameters := &applicationModels.RegenerateDeployKeyAndSecretData{SharedSecret: "new shared secret"}
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/regenerate-deploy-key", appName), regenerateParameters)
	response := <-responseChannel
	assert.Equal(t, http.StatusNoContent, response.Code)

	// forcing resync of RR
	err = utils.ApplyRegistrationWithSync(kubeUtil, radixClient, kedaClient, commonTestUtils, rrBuilder)
	require.NoError(t, err)

	// Check that secret has been re-created and is different from first secret
	secondSecret, err := kubeUtil.CoreV1().Secrets(builders.GetAppNamespace(appName)).Get(context.Background(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(secondSecret.Data[defaults.GitPrivateKeySecretKey]), 1)
	assert.NotEqual(t, firstSecret.Data[defaults.GitPrivateKeySecretKey], secondSecret.Data[defaults.GitPrivateKeySecretKey])
}

func TestRegenerateDeployKey_PrivateKeyInParam_SavedPrivateKeyIsEqualToWebParam(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeUtil, radixClient, kedaClient, _, _, _ := setupTest(t, true, true)
	appName := "any-name"
	rrBuilder := builders.ARadixRegistration().WithName(appName).WithCloneURL("git@github.com:Equinor/my-app.git")

	// Creating RR and syncing it
	err := utils.ApplyRegistrationWithSync(kubeUtil, radixClient, kedaClient, commonTestUtils, rrBuilder)
	require.NoError(t, err)

	// make some valid private key
	deployKey, err := builders.GenerateDeployKey()
	assert.NoError(t, err)

	// calling regenerate-deploy-key in order to set secret
	regenerateParameters := &applicationModels.RegenerateDeployKeyAndSecretData{SharedSecret: "new shared secret", PrivateKey: deployKey.PrivateKey}
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/regenerate-deploy-key", appName), regenerateParameters)
	response := <-responseChannel
	assert.Equal(t, http.StatusNoContent, response.Code)

	// forcing resync of RR
	err = utils.ApplyRegistrationWithSync(kubeUtil, radixClient, kedaClient, commonTestUtils, rrBuilder)
	require.NoError(t, err)

	// Check that secret has been re-created and is equal to the one in the web parameter
	secret, err := kubeUtil.CoreV1().Secrets(builders.GetAppNamespace(appName)).Get(context.Background(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, deployKey.PrivateKey, string(secret.Data[defaults.GitPrivateKeySecretKey]))
}

func TestRegenerateDeployKey_InvalidKeyInParam_ErrorIsReturned(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeUtil, radixClient, kedaClient, _, _, _ := setupTest(t, true, true)
	appName := "any-name"
	rrBuilder := builders.ARadixRegistration().WithName(appName).WithCloneURL("git@github.com:Equinor/my-app.git")

	// Creating RR and syncing it
	err := utils.ApplyRegistrationWithSync(kubeUtil, radixClient, kedaClient, commonTestUtils, rrBuilder)
	require.NoError(t, err)

	// calling regenerate-deploy-key with invalid private key, expecting error
	regenerateParameters := &applicationModels.RegenerateDeployKeyAndSecretData{SharedSecret: "new shared secret", PrivateKey: "invalid key"}
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/regenerate-deploy-key", appName), regenerateParameters)
	response := <-responseChannel
	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func Test_GetUsedResources(t *testing.T) {
	const (
		appName1 = "app-1"
	)

	type scenario struct {
		name                       string
		expectedError              error
		queryString                string
		expectedUsedResourcesError error
	}

	scenarios := []scenario{
		{
			name: "Get used resources",
		},
		{
			name:        "Get used resources with arguments",
			queryString: "?environment=prod&component=component1&duration=10d&since=2w",
		},
		{
			name:                       "UsedResources returns an error",
			expectedUsedResourcesError: errors.New("error-123"),
			expectedError:              errors.New("error: error-123"),
		},
	}

	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			commonTestUtils, _, kubeClient, radixClient, kedaClient, _, secretProviderClient, certClient := setupTest(t, true, true)
			_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration().WithName(appName1))
			require.NoError(t, err)

			expectedUtilization := applicationModels.NewPodResourcesUtilizationResponse()
			expectedUtilization.SetCpuReqs("dev", "web", "web-abcd-1", 1)

			cpuReqs := []metrics.LabeledResults{{Value: 1, Namespace: appName1 + "-dev", Component: "web", Pod: "web-abcd-1"}}

			validator := authnmock.NewMockValidatorInterface(ctrl)
			validator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).Times(1).Return(controllertest.NewTestPrincipal(true), nil)

			client := mock2.NewMockClient(ctrl)
			client.EXPECT().GetCpuReqs(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(cpuReqs, nil)
			client.EXPECT().GetCpuAvg(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return([]metrics.LabeledResults{}, nil)
			client.EXPECT().GetMemReqs(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return([]metrics.LabeledResults{}, nil)
			client.EXPECT().GetMemMax(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return([]metrics.LabeledResults{}, ts.expectedError)
			metricsHandler := metrics.NewHandler(client)

			controllerTestUtils := controllertest.NewTestUtils(kubeClient, radixClient, kedaClient, secretProviderClient, certClient, validator,
				NewApplicationController(
					func(_ context.Context, _ kubernetes.Interface, _ v1.RadixRegistration) (bool, error) {
						return true, nil
					},
					newTestApplicationHandlerFactory(
						config.Config{RequireAppConfigurationItem: true, RequireAppADGroups: true},
						func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
							return true, nil
						},
					),
					metricsHandler,
				),
			)

			responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/utilization", appName1))
			response := <-responseChannel
			if ts.expectedError != nil {
				assert.Equal(t, http.StatusBadRequest, response.Code)
				errorResponse, _ := controllertest.GetErrorResponse(response)
				assert.Equal(t, ts.expectedError.Error(), errorResponse.Error())
				return
			}
			assert.Equal(t, http.StatusOK, response.Code)
			actualUtilization := &applicationModels.ReplicaResourcesUtilizationResponse{}
			err = controllertest.GetResponseBody(response, &actualUtilization)
			require.NoError(t, err)
			assert.Equal(t, expectedUtilization, actualUtilization)
		})
	}
}

func createRadixJob(commonTestUtils *commontest.Utils, appName, jobName string, started time.Time) error {
	_, err := commonTestUtils.ApplyJob(
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
	return err
}

func getJobsInNamespace(radixclient *radixfake.Clientset, appNamespace string) ([]v1.RadixJob, error) {
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
	config                  config.Config
	hasAccessToGetConfigMap hasAccessToGetConfigMapFunc
}

func newTestApplicationHandlerFactory(config config.Config, hasAccessToGetConfigMap hasAccessToGetConfigMapFunc) *testApplicationHandlerFactory {
	return &testApplicationHandlerFactory{
		config:                  config,
		hasAccessToGetConfigMap: hasAccessToGetConfigMap,
	}
}

// Create creates a new ApplicationHandler
func (f *testApplicationHandlerFactory) Create(accounts models.Accounts) ApplicationHandler {
	return NewApplicationHandler(accounts, f.config, f.hasAccessToGetConfigMap)
}

func createTime(timestamp string) *time.Time {
	if timestamp == "" {
		return &time.Time{}
	}

	t, _ := time.Parse(time.RFC3339, timestamp)
	return &t
}
