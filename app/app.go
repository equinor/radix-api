package main

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/spf13/pflag"
	routers "github.com/statoil/radix-api-go/http"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	// Force loading of needed authentication library
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"net/http"
	"os"
)

func main() {
	fs := initializeFlagSet()

	var (
		kubeconfig = fs.String("kubeconfig", defaultKubeConfig(), "Absolute path to the kubeconfig file")
		port       = fs.StringP("port", "p", defaultPort(), "Port where API will be served")
	)

	parseFlagsFromArgs(fs)
	printEnvironment()

	client, radixClient := getKubernetesClient(*kubeconfig)
	apiRouter := routers.NewAPIRouter(client, radixClient)

	logrus.Infof("Api is serving on port %s", *port)
	err := http.ListenAndServe(fmt.Sprintf(":%s", *port), apiRouter.Router)
	if err != nil {
		logrus.Fatalf("Unable to start serving: %v", err)
	}
}

func initializeFlagSet() *pflag.FlagSet {
	// Flag domain.
	fs := pflag.NewFlagSet("default", pflag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "DESCRIPTION\n")
		fmt.Fprintf(os.Stderr, "  radix api-server.\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "FLAGS\n")
		fs.PrintDefaults()
	}
	return fs
}

func printEnvironment() {
	// print env
	env := os.Getenv("APP_ENV")
	if env == "production" {
		log.Println("Running api server in production mode")
	} else {
		log.Println("Running api server in dev mode")
	}
}

func parseFlagsFromArgs(fs *pflag.FlagSet) {
	err := fs.Parse(os.Args[1:])
	switch {
	case err == pflag.ErrHelp:
		os.Exit(0)
	case err != nil:
		fmt.Fprintf(os.Stderr, "Error: %s\n\n", err.Error())
		fs.Usage()
		os.Exit(2)
	}
}

func getKubernetesClient(kubeConfigPath string) (kubernetes.Interface, radixclient.Interface) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			logrus.Fatalf("getClusterConfig InClusterConfig: %v", err)
		}
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		logrus.Fatalf("getClusterConfig k8s client: %v", err)
	}

	radixClient, err := radixclient.NewForConfig(config)
	if err != nil {
		logrus.Fatalf("getClusterConfig radix client: %v", err)
	}

	return client, radixClient
}

func defaultKubeConfig() string {
	return os.Getenv("HOME") + "/.kube/config"
}

func defaultPort() string {
	return "3001"
}
