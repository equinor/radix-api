package main

import (
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strconv"
	"time"

	"github.com/equinor/radix-api/api/secrets"
	"github.com/equinor/radix-api/api/utils/tlsvalidation"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/equinor/radix-api/api/environmentvariables"

	"github.com/equinor/radix-api/api/buildstatus"

	"github.com/spf13/pflag"

	// Controllers
	"github.com/equinor/radix-api/api/alerting"
	"github.com/equinor/radix-api/api/applications"
	"github.com/equinor/radix-api/api/buildsecrets"
	build_models "github.com/equinor/radix-api/api/buildstatus/models"
	"github.com/equinor/radix-api/api/deployments"
	"github.com/equinor/radix-api/api/environments"
	"github.com/equinor/radix-api/api/jobs"
	"github.com/equinor/radix-api/api/privateimagehubs"
	"github.com/equinor/radix-api/api/router"
	"github.com/equinor/radix-api/api/utils"
	_ "github.com/equinor/radix-api/docs"
	"github.com/equinor/radix-api/models"
)

const (
	logLevelEnvironmentVariable    = "LOG_LEVEL"
	logPrettyEnvironmentVariable   = "LOG_PRETTY"
	useProfilerEnvironmentVariable = "USE_PROFILER"
)

//go:generate swagger generate spec
func main() {
	initLogger()
	fs := initializeFlagSet()

	var (
		port                = fs.StringP("port", "p", defaultPort(), "Port where API will be served")
		useOutClusterClient = fs.Bool("useOutClusterClient", true, "In case of testing on local machine you may want to set this to false")
		clusterName         = os.Getenv(defaults.ClusternameEnvironmentVariable)
		certPath            = os.Getenv("server_cert_path") // "/etc/webhook/certs/cert.pem"
		keyPath             = os.Getenv("server_key_path")  // "/etc/webhook/certs/key.pem"
		httpsPort           = "3003"
	)

	parseFlagsFromArgs(fs)

	controllers, err := getControllers()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get controllers")
	}
	errs := make(chan error)

	go func() {
		log.Info().Msgf("Api is serving on port %s", *port)
		err := http.ListenAndServe(fmt.Sprintf(":%s", *port), router.NewServer(clusterName, utils.NewKubeUtil(*useOutClusterClient), controllers...))
		errs <- err
	}()
	if certPath != "" && keyPath != "" {
		go func() {
			log.Info().Msgf("Api is serving on port %s", httpsPort)
			err := http.ListenAndServeTLS(fmt.Sprintf(":%s", httpsPort), certPath, keyPath, router.NewServer(clusterName, utils.NewKubeUtil(*useOutClusterClient), controllers...))
			errs <- err
		}()
	} else {
		log.Info().Msg("Https support disabled - Env variable server_cert_path and server_key_path is empty.")
	}

	if useProfiler, _ := strconv.ParseBool(os.Getenv(useProfilerEnvironmentVariable)); useProfiler {
		go func() {
			log.Info().Msgf("Profiler endpoint is serving on port 7070")
			errs <- http.ListenAndServe("localhost:7070", nil)
		}()
	}

	err = <-errs
	if err != nil {
		log.Fatal().Err(err).Msg("Web api server crashed")
	}
}

func initLogger() {
	logLevelStr := os.Getenv(logLevelEnvironmentVariable)
	if len(logLevelStr) == 0 {
		logLevelStr = zerolog.LevelInfoValue
	}

	logLevel, err := zerolog.ParseLevel(logLevelStr)
	if err != nil {
		logLevel = zerolog.InfoLevel
		log.Warn().Msgf("Invalid log level '%s', fallback to '%s'", logLevelStr, logLevel.String())
	}

	logPretty, _ := strconv.ParseBool(os.Getenv(logPrettyEnvironmentVariable))

	var logWriter io.Writer = os.Stderr
	if logPretty {
		logWriter = &zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.TimeOnly}
	}

	logger := zerolog.New(logWriter).Level(logLevel).With().Timestamp().Logger()

	log.Logger = logger
	zerolog.DefaultContextLogger = &logger
}

func getControllers() ([]models.Controller, error) {
	buildStatus := build_models.NewPipelineBadge()
	applicationHandlerFactory, err := getApplicationHandlerFactory()
	if err != nil {
		return nil, err
	}

	return []models.Controller{
		applications.NewApplicationController(nil, applicationHandlerFactory),
		deployments.NewDeploymentController(),
		jobs.NewJobController(),
		environments.NewEnvironmentController(environments.NewEnvironmentHandlerFactory()),
		environmentvariables.NewEnvVarsController(),
		privateimagehubs.NewPrivateImageHubController(),
		buildsecrets.NewBuildSecretsController(),
		buildstatus.NewBuildStatusController(buildStatus),
		alerting.NewAlertingController(),
		secrets.NewSecretController(tlsvalidation.DefaultValidator()),
	}, nil
}

func getApplicationHandlerFactory() (applications.ApplicationHandlerFactory, error) {
	cfg, err := applications.LoadApplicationHandlerConfig(os.Args[1:])
	if err != nil {
		return nil, err
	}
	return applications.NewApplicationHandlerFactory(cfg), nil
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
