package utils

import (
	"os"

	"github.com/Sirupsen/logrus"

	logger "github.com/Sirupsen/logrus"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// GetKubernetesClient Gets a kubernetes client using the bearer token from the radix api client
func GetKubernetesClient(token string) (kubernetes.Interface, radixclient.Interface) {
	logrus.Info("Get kubernetes client")
	config := getInClusterClientConfig()
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatalf("getClusterConfig k8s client: %v", err)
	}

	radixClient, err := radixclient.NewForConfig(config)
	if err != nil {
		logger.Fatalf("getClusterConfig radix client: %v", err)
	}

	return client, radixClient
}

func getOutClusterClientConfig(token string) *restclient.Config {
	logrus.Info("Using out of cluster config")
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
	logrus.Info("Using in cluster config")
	kubeConfigPath := os.Getenv("HOME") + "/.kube/config"
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			logger.Fatalf("getClusterConfig InClusterConfig: %v", err)
		}
	}

	return config
}
