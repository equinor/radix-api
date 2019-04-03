package utils

import (
	"os"
	"strings"

	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubeUtil Interface to be mocked in tests
type KubeUtil interface {
	GetOutClusterKubernetesClient(string) (kubernetes.Interface, radixclient.Interface)
	GetOutClusterKubernetesClientWithImpersonation(string, string, string) (kubernetes.Interface, radixclient.Interface)
	GetInClusterKubernetesClient() (kubernetes.Interface, radixclient.Interface)
}

type kubeUtil struct {
	useOutClusterClient bool
}

// NewKubeUtil Constructor
func NewKubeUtil(useOutClusterClient bool) KubeUtil {
	return &kubeUtil{
		useOutClusterClient,
	}
}

// GetOutClusterKubernetesClient Gets a kubernetes client using the bearer token from the radix api client
func (ku *kubeUtil) GetOutClusterKubernetesClient(token string) (kubernetes.Interface, radixclient.Interface) {
	if ku.useOutClusterClient {
		config := getOutClusterClientConfig(token, nil, nil)
		return getKubernetesClientFromConfig(config)
	}

	return ku.GetInClusterKubernetesClient()
}

// GetOutClusterKubernetesClient Gets a kubernetes client using the bearer token from the radix api client
func (ku *kubeUtil) GetOutClusterKubernetesClientWithImpersonation(token, impersonateUser, impersonateGroup string) (kubernetes.Interface, radixclient.Interface) {
	if ku.useOutClusterClient {
		config := getOutClusterClientConfig(token, &impersonateUser, &impersonateGroup)
		return getKubernetesClientFromConfig(config)
	}

	return ku.GetInClusterKubernetesClient()
}

// GetInClusterKubernetesClient Gets a kubernetes client using the config of the running pod
func (ku *kubeUtil) GetInClusterKubernetesClient() (kubernetes.Interface, radixclient.Interface) {
	config := getInClusterClientConfig()
	return getKubernetesClientFromConfig(config)
}

func getOutClusterClientConfig(token string, impersonateUser, impersonateGroup *string) *restclient.Config {
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

	if impersonateUser != nil &&
		impersonateGroup != nil &&
		strings.TrimSpace(*impersonateUser) != "" &&
		strings.TrimSpace(*impersonateGroup) != "" {

		impersonationConfig := restclient.ImpersonationConfig{
			UserName: *impersonateUser,
			Groups:   []string{*impersonateGroup},
		}

		kubeConfig.Impersonate = impersonationConfig
	}

	return kubeConfig
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
