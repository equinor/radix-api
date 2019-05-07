package utils

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/equinor/radix-api/api/metrics"

	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubeUtil Interface to be mocked in tests
type KubeUtil interface {
	GetOutClusterKubernetesClient(string) (kubernetes.Interface, radixclient.Interface)
	GetOutClusterKubernetesClientWithImpersonation(string, string, string) (kubernetes.Interface, radixclient.Interface, error)
	GetInClusterKubernetesClient() (kubernetes.Interface, radixclient.Interface)
}

type kubeUtil struct {
	useOutClusterClient bool
}

var (
	nrRequests = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "radix_api_k8s_request",
		Help:    "request duration done to k8s api in seconds bucket",
		Buckets: metrics.DefaultBuckets(),
	}, []string{"code", "method"})
)

// NewKubeUtil Constructor
func NewKubeUtil(useOutClusterClient bool) KubeUtil {
	return &kubeUtil{
		useOutClusterClient,
	}
}

// GetOutClusterKubernetesClient Gets a kubernetes client using the bearer token from the radix api client
func (ku *kubeUtil) GetOutClusterKubernetesClient(token string) (kubernetes.Interface, radixclient.Interface) {
	if ku.useOutClusterClient {
		config, _ := getOutClusterClientConfig(token, nil, nil)
		return getKubernetesClientFromConfig(config)
	}

	return ku.GetInClusterKubernetesClient()
}

// GetOutClusterKubernetesClient Gets a kubernetes client using the bearer token from the radix api client
func (ku *kubeUtil) GetOutClusterKubernetesClientWithImpersonation(token, impersonateUser, impersonateGroup string) (kubernetes.Interface, radixclient.Interface, error) {
	if ku.useOutClusterClient {
		config, err := getOutClusterClientConfig(token, &impersonateUser, &impersonateGroup)
		if err != nil {
			return nil, nil, err
		}

		client, radixclient := getKubernetesClientFromConfig(config)
		return client, radixclient, err
	}

	client, radixclient := ku.GetInClusterKubernetesClient()
	return client, radixclient, nil
}

// GetInClusterKubernetesClient Gets a kubernetes client using the config of the running pod
func (ku *kubeUtil) GetInClusterKubernetesClient() (kubernetes.Interface, radixclient.Interface) {
	config := getInClusterClientConfig()
	return getKubernetesClientFromConfig(config)
}

func getOutClusterClientConfig(token string, impersonateUser, impersonateGroup *string) (*restclient.Config, error) {
	host := os.Getenv("K8S_API_HOST")
	if host == "" {
		host = "https://kubernetes.default.svc"
	}

	kubeConfig := &restclient.Config{
		Host:        host,
		BearerToken: token,
		TLSClientConfig: restclient.TLSClientConfig{
			Insecure: true,
		},
	}

	impersonate, err := performImpersonation(impersonateUser, impersonateGroup)
	if err != nil {
		return nil, err
	}

	if impersonate {
		impersonationConfig := restclient.ImpersonationConfig{
			UserName: *impersonateUser,
			Groups:   []string{*impersonateGroup},
		}

		kubeConfig.Impersonate = impersonationConfig
	}

	return addCommonConfigs(kubeConfig), nil
}

func getInClusterClientConfig() *restclient.Config {
	kubeConfigPath := os.Getenv("HOME") + "/.kube/config"
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Fatalf("getClusterConfig InClusterConfig: %v", err)
		}
	}

	return addCommonConfigs(config)
}

func addCommonConfigs(config *restclient.Config) *restclient.Config {
	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return promhttp.InstrumentRoundTripperDuration(nrRequests, rt)
	}
	return config
}

func getKubernetesClientFromConfig(config *restclient.Config) (kubernetes.Interface, radixclient.Interface) {
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("getClusterConfig k8s client: %v", err)
	}

	radixClient, err := radixclient.NewForConfig(config)
	if err != nil {
		log.Fatalf("getClusterConfig radix client: %v", err)
	}

	return client, radixClient
}

func performImpersonation(impersonateUser, impersonateGroup *string) (bool, error) {
	impersonateUserSet := impersonateUser != nil && strings.TrimSpace(*impersonateUser) != ""
	impersonateGroupSet := impersonateGroup != nil && strings.TrimSpace(*impersonateGroup) != ""

	if (impersonateUserSet && !impersonateGroupSet) ||
		(!impersonateUserSet && impersonateGroupSet) {
		return true, errors.New("Impersonation cannot be done without both user and group being set")
	}

	if impersonateUserSet &&
		impersonateGroupSet {
		return true, nil
	}

	return false, nil
}
