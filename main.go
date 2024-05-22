package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
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
	defaultPort                    = "3002"
	defaultMetricsPort             = "9090"
	defaultProfilePort             = "7070"
)

//go:generate swagger generate spec
func main() {
	setupLogger()
	fs := initializeFlagSet()

	var (
		port                = fs.StringP("port", "p", defaultPort, "Port where API will be served")
		metricsPort         = fs.String("metrics-port", defaultMetricsPort, "The metrics API server port")
		useOutClusterClient = fs.Bool("useOutClusterClient", true, "In case of testing on local machine you may want to set this to false")
		clusterName         = os.Getenv(defaults.ClusternameEnvironmentVariable)
	)

	parseFlagsFromArgs(fs)

	var servers []*http.Server

	srv, err := initializeServer(*port, clusterName, *useOutClusterClient)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize API server")
	}

	servers = append(servers, srv, initializeMetricsServer(*metricsPort))

	if useProfiler, _ := strconv.ParseBool(os.Getenv(useProfilerEnvironmentVariable)); useProfiler {
		log.Info().Msgf("Initializing profile server on port %s", defaultProfilePort)
		servers = append(servers, &http.Server{Addr: fmt.Sprintf("localhost:%s", defaultProfilePort)})
	}

	startServers(servers...)
	shutdownServersGracefulOnSignal(servers...)
}

func initializeServer(port, clusterName string, useOutClusterClient bool) (*http.Server, error) {
	controllers, err := getControllers()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize controllers: %w", err)
	}
	handler := router.NewAPIHandler(clusterName, utils.NewKubeUtil(useOutClusterClient), controllers...)
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: handler,
	}

	return srv, nil
}

func initializeMetricsServer(port string) *http.Server {
	log.Info().Msgf("Initializing metrics server on port %s", port)
	return &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: router.NewMetricsHandler(),
	}
}

func startServers(servers ...*http.Server) {
	for _, srv := range servers {
		go func() {
			log.Info().Msgf("Starting server on address %s", srv.Addr)
			if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				log.Fatal().Err(err).Msgf("Unable to start server on address %s", srv.Addr)
			}
		}()
	}
}

func shutdownServersGracefulOnSignal(servers ...*http.Server) {
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGTERM, syscall.SIGINT)
	s := <-stopCh
	log.Info().Msgf("Received %v signal", s)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var wg sync.WaitGroup

	for _, srv := range servers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Info().Msgf("Shutting down server on address %s", srv.Addr)
			if err := srv.Shutdown(shutdownCtx); err != nil {
				log.Warn().Err(err).Msgf("shutdown of server on address %s returned an error", srv.Addr)
			}
		}()
	}

	wg.Wait()
}

func setupLogger() {
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
