package utils

import (
	"net/http"
	"os"

	"github.com/equinor/radix-api/api/metrics"
	radixmodels "github.com/equinor/radix-common/models"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	secretproviderclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
)

type RestClientConfigOption func(*restclient.Config)

func WithQPS(qps float32) RestClientConfigOption {
	return func(cfg *restclient.Config) {
		cfg.QPS = qps
	}
}

func WithBurst(burst int) RestClientConfigOption {
	return func(cfg *restclient.Config) {
		cfg.Burst = burst
	}
}

// KubeUtil Interface to be mocked in tests
type KubeUtil interface {
	GetOutClusterKubernetesClient(string, ...RestClientConfigOption) (kubernetes.Interface, radixclient.Interface, secretproviderclient.Interface, tektonclient.Interface)
	GetOutClusterKubernetesClientWithImpersonation(string, radixmodels.Impersonation, ...RestClientConfigOption) (kubernetes.Interface, radixclient.Interface, secretproviderclient.Interface, tektonclient.Interface)
	GetInClusterKubernetesClient(...RestClientConfigOption) (kubernetes.Interface, radixclient.Interface, secretproviderclient.Interface, tektonclient.Interface)
	IsUseOutClusterClient() bool
}

type kubeUtil struct {
	useOutClusterClient bool
}

func (ku *kubeUtil) IsUseOutClusterClient() bool {
	return ku.useOutClusterClient
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
func (ku *kubeUtil) GetOutClusterKubernetesClient(token string, options ...RestClientConfigOption) (kubernetes.Interface, radixclient.Interface, secretproviderclient.Interface, tektonclient.Interface) {
	return ku.GetOutClusterKubernetesClientWithImpersonation(token, radixmodels.Impersonation{}, options...)
}

// GetOutClusterKubernetesClientWithImpersonation Gets a kubernetes client using the bearer token from the radix api client
func (ku *kubeUtil) GetOutClusterKubernetesClientWithImpersonation(token string, impersonation radixmodels.Impersonation, options ...RestClientConfigOption) (kubernetes.Interface, radixclient.Interface, secretproviderclient.Interface, tektonclient.Interface) {
	if ku.useOutClusterClient {
		config := getOutClusterClientConfig(token, impersonation, options)
		return getKubernetesClientFromConfig(config)
	}

	return ku.GetInClusterKubernetesClient(options...)
}

// GetInClusterKubernetesClient Gets a kubernetes client using the config of the running pod
func (ku *kubeUtil) GetInClusterKubernetesClient(options ...RestClientConfigOption) (kubernetes.Interface, radixclient.Interface, secretproviderclient.Interface, tektonclient.Interface) {
	config := getInClusterClientConfig(options)
	return getKubernetesClientFromConfig(config)
}

func getOutClusterClientConfig(token string, impersonation radixmodels.Impersonation, options []RestClientConfigOption) *restclient.Config {
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

	return addCommonConfigs(kubeConfig, options)
}

func getInClusterClientConfig(options []RestClientConfigOption) *restclient.Config {
	kubeConfigPath := os.Getenv("HOME") + "/.kube/config"
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		config, err = restclient.InClusterConfig()
		if err != nil {
			log.Fatalf("getClusterConfig InClusterConfig: %v", err)
		}
	}

	return addCommonConfigs(config, options)
}

func addCommonConfigs(config *restclient.Config, options []RestClientConfigOption) *restclient.Config {
	for _, opt := range options {
		opt(config)
	}

	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return promhttp.InstrumentRoundTripperDuration(nrRequests, rt)
	}
	return config
}

func getKubernetesClientFromConfig(config *restclient.Config) (kubernetes.Interface, radixclient.Interface, secretproviderclient.Interface, tektonclient.Interface) {
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("getClusterConfig k8s client: %v", err)
	}

	radixClient, err := radixclient.NewForConfig(config)
	if err != nil {
		log.Fatalf("getClusterConfig radix client: %v", err)
	}

	secretProviderClient, err := secretproviderclient.NewForConfig(config)
	if err != nil {
		log.Fatalf("getClusterConfig secret provider client client: %v", err)
	}

	tektonClient, err := tektonclient.NewForConfig(config)
	if err != nil {
		log.Fatalf("getClusterConfig Tekton client client: %v", err)
	}
	return client, radixClient, secretProviderClient, tektonClient
}
