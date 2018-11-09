package utils

import (
	"os"

	log "github.com/Sirupsen/logrus"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubeUtil Interface to be mocked in tests
type KubeUtil interface {
	GetOutClusterKubernetesClient(string) (kubernetes.Interface, radixclient.Interface)
	GetInClusterKubernetesClient() (kubernetes.Interface, radixclient.Interface)
}

type kubeUtil struct {
}

// NewKubeUtil Constructor
func NewKubeUtil() KubeUtil {
	return &kubeUtil{}
}

// GetOutClusterKubernetesClient Gets a kubernetes client using the bearer token from the radix api client
func (ku *kubeUtil) GetOutClusterKubernetesClient(token string) (kubernetes.Interface, radixclient.Interface) {
	config := getOutClusterClientConfig(token)
	return getKubernetesClientFromConfig(config)
}

// GetInClusterKubernetesClient Gets a kubernetes client using the config of the running pod
func (ku *kubeUtil) GetInClusterKubernetesClient() (kubernetes.Interface, radixclient.Interface) {
	config := getInClusterClientConfig()
	return getKubernetesClientFromConfig(config)
}

func getOutClusterClientConfig(token string) *restclient.Config {
	log.Info("Using out of cluster config")
	kubeConfig := &restclient.Config{
		Host:        "https://kubernetes.default.svc",
		BearerToken: token,
		TLSClientConfig: restclient.TLSClientConfig{
			Insecure: true,
		},
	}

	return kubeConfig
}

func getInClusterClientConfig() *restclient.Config {
	log.Info("Using in cluster config")
	kubeConfigPath := os.Getenv("HOME") + "/.kube/config"
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Fatalf("getClusterConfig InClusterConfig: %v", err)
		}
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
