package utils

import (
	"net/http"
	"os"

	"github.com/equinor/radix-api/api/metrics"

	"github.com/equinor/radix-api/models"
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
	GetOutClusterKubernetesClientWithImpersonation(string, models.Impersonation) (kubernetes.Interface, radixclient.Interface)
	GetInClusterKubernetesClient() (kubernetes.Interface, radixclient.Interface)
}

type kubeUtil struct {
	useOutClusterClient bool
}

var (
	nrRequests = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "radix_api_k8s_request_duration_seconds",
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
	return ku.GetOutClusterKubernetesClientWithImpersonation(token, models.Impersonation{})
}

// GetOutClusterKubernetesClient Gets a kubernetes client using the bearer token from the radix api client
func (ku *kubeUtil) GetOutClusterKubernetesClientWithImpersonation(token string, impersonation models.Impersonation) (kubernetes.Interface, radixclient.Interface) {
	if ku.useOutClusterClient {
		config := getOutClusterClientConfig(token, impersonation)
		return getKubernetesClientFromConfig(config)
	}

	return ku.GetInClusterKubernetesClient()
}

// GetInClusterKubernetesClient Gets a kubernetes client using the config of the running pod
func (ku *kubeUtil) GetInClusterKubernetesClient() (kubernetes.Interface, radixclient.Interface) {
	config := getInClusterClientConfig()
	return getKubernetesClientFromConfig(config)
}

func getOutClusterClientConfig(token string, impersonation models.Impersonation) *restclient.Config {
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

	if impersonation.PerformImpersonation() {
		impersonationConfig := restclient.ImpersonationConfig{
			UserName: impersonation.User,
			Groups:   []string{impersonation.Group},
		}

		kubeConfig.Impersonate = impersonationConfig
	}

	return addCommonConfigs(kubeConfig)
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
