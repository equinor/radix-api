package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/equinor/radix-api/api/secrets"

	"github.com/equinor/radix-api/api/environmentvariables"

	"github.com/equinor/radix-api/api/buildstatus"

	"github.com/gorilla/handlers"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"

	// Controllers
	"github.com/equinor/radix-api/api/admissioncontrollers"
	"github.com/equinor/radix-api/api/alerting"
	"github.com/equinor/radix-api/api/applications"
	"github.com/equinor/radix-api/api/buildsecrets"
	"github.com/equinor/radix-api/api/deployments"
	"github.com/equinor/radix-api/api/environments"
	"github.com/equinor/radix-api/api/jobs"
	"github.com/equinor/radix-api/api/privateimagehubs"

	build_models "github.com/equinor/radix-api/api/buildstatus/models"

	router "github.com/equinor/radix-api/api/router"
	"github.com/equinor/radix-api/models"

	"github.com/equinor/radix-api/api/utils"

	_ "github.com/equinor/radix-api/docs"
)

const clusternameEnvironmentVariable = "RADIX_CLUSTERNAME"

//go:generate swagger generate spec
func main() {
	switch os.Getenv("LOG_LEVEL") {
	case "DEBUG":
		log.SetLevel(log.DebugLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}

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
		log.Fatalf("Web api server crashed: %v", err)
	}
}

func getControllers() []models.Controller {
	buildStatus := build_models.NewPipelineBadge()
	return []models.Controller{
		admissioncontrollers.NewAdmissionController(),
		applications.NewApplicationController(nil),
		deployments.NewDeploymentController(),
		jobs.NewJobController(),
		environments.NewEnvironmentController(),
		environmentvariables.NewEnvVarsController(),
		privateimagehubs.NewPrivateImageHubController(),
		buildsecrets.NewBuildSecretsController(),
		buildstatus.NewBuildStatusController(buildStatus),
		alerting.NewAlertingController(),
		secrets.NewSecretController(),
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
