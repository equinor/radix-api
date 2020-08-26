package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"

	// Controllers
	"github.com/equinor/radix-api/api/admissioncontrollers"
	"github.com/equinor/radix-api/api/applications"
	"github.com/equinor/radix-api/api/buildsecrets"
	"github.com/equinor/radix-api/api/deployments"
	"github.com/equinor/radix-api/api/environments"
	"github.com/equinor/radix-api/api/jobs"
	"github.com/equinor/radix-api/api/privateimagehubs"

	router "github.com/equinor/radix-api/api/router"
	"github.com/equinor/radix-api/models"

	"github.com/equinor/radix-api/api/utils"

	// Force loading of needed authentication library
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	_ "github.com/equinor/radix-api/docs"
)

const clusternameEnvironmentVariable = "RADIX_CLUSTERNAME"

//go:generate swagger generate spec
func main() {
	fs := initializeFlagSet()

	var (
		port                = fs.StringP("port", "p", defaultPort(), "Port where API will be served")
		useOutClusterClient = fs.Bool("useOutClusterClient", true, "In case of testing on local machine you may want to set this to false")
		clusterName         = os.Getenv(clusternameEnvironmentVariable)
		certPath            = os.Getenv("server_cert_path") // "/etc/webhook/certs/cert.pem"
		keyPath             = os.Getenv("server_key_path")  // "/etc/webhook/certs/key.pem"
		httpsPort           = "3003"
	)

	parseFlagsFromArgs(fs)

	errs := make(chan error)
	go func() {
		log.Infof("Api is serving on port %s", *port)
		err := http.ListenAndServe(fmt.Sprintf(":%s", *port), handlers.CombinedLoggingHandler(os.Stdout, router.NewServer(clusterName, utils.NewKubeUtil(*useOutClusterClient), getControllers()...)))
		errs <- err
	}()
	if certPath != "" && keyPath != "" {
		go func() {
			log.Infof("Api is serving on port %s", httpsPort)
			err := http.ListenAndServeTLS(fmt.Sprintf(":%s", httpsPort), certPath, keyPath, router.NewServer(clusterName, utils.NewKubeUtil(*useOutClusterClient), getControllers()...))
			errs <- err
		}()
	} else {
		log.Info("Https support disabled - Env variable server_cert_path and server_key_path is empty.")
	}

	err := <-errs
	if err != nil {
		log.Fatalf("Web api server crached: %v", err)
	}
}

func getControllers() []models.Controller {
	return []models.Controller{
		admissioncontrollers.NewAdmissionController(),
		applications.NewApplicationController(nil),
		deployments.NewDeploymentController(),
		jobs.NewJobController(),
		environments.NewEnvironmentController(),
		privateimagehubs.NewPrivateImageHubController(),
		buildsecrets.NewBuildSecretsController(),
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

func defaultPort() string {
	return "3002"
}
