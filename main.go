package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/equinor/radix-api/api/metrics"
	"github.com/equinor/radix-api/api/secrets"
	"github.com/equinor/radix-api/api/utils/tlsvalidation"
	token "github.com/equinor/radix-api/api/utils/token"
	"github.com/equinor/radix-api/internal/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/equinor/radix-api/api/environmentvariables"

	"github.com/equinor/radix-api/api/buildstatus"

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

//go:generate swagger generate spec
func main() {
	c := config.MustParse()
	setupLogger(c.LogLevel, c.LogPrettyPrint)

	servers := []*http.Server{
		initializeServer(c),
		initializeMetricsServer(c),
	}

	if c.UseProfiler {
		log.Info().Msgf("Initializing profile server on port %d", c.ProfilePort)
		servers = append(servers, &http.Server{Addr: fmt.Sprintf("localhost:%d", c.ProfilePort)})
	}

	startServers(servers...)
	shutdownServersGracefulOnSignal(servers...)
}

func initializeServer(c config.Config) *http.Server {
	jwtValidator := initializeTokenValidator(c)
	controllers, err := getControllers(c)
	if err != nil {
		log.Fatal().Err(err).Msgf("failed to initialize controllers: %v", err)
	}

	handler := router.NewAPIHandler(jwtValidator, utils.NewKubeUtil(), controllers...)
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", c.Port),
		Handler: handler,
	}

	return srv
}

func initializeTokenValidator(c config.Config) token.ValidatorInterface {
	issuerUrl, err := url.Parse(c.OidcIssuer)
	if err != nil {
		log.Fatal().Err(err).Msg("Error parsing issuer url")
	}

	// Set up the validator.
	// jwtValidator, err := token.NewValidator(issuerUrl, c.OidcAudience)
	jwtValidator, err := token.NewUncheckedValidator(issuerUrl, c.OidcAudience)
	if err != nil {
		log.Fatal().Err(err).Msg("Error creating JWT validator")
	}
	return jwtValidator
}

func initializeMetricsServer(c config.Config) *http.Server {
	log.Info().Msgf("Initializing metrics server on port %d", c.MetricsPort)
	return &http.Server{
		Addr:    fmt.Sprintf(":%d", c.MetricsPort),
		Handler: router.NewMetricsHandler(),
	}
}

func startServers(servers ...*http.Server) {
	for _, srv := range servers {
		srv := srv
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
		srv := srv
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

func setupLogger(logLevelStr string, prettyPrint bool) {
	if len(logLevelStr) == 0 {
		logLevelStr = zerolog.LevelInfoValue
	}

	logLevel, err := zerolog.ParseLevel(logLevelStr)
	if err != nil {
		logLevel = zerolog.InfoLevel
		log.Warn().Msgf("Invalid log level '%s', fallback to '%s'", logLevelStr, logLevel.String())
	}

	var logWriter io.Writer = os.Stderr
	if prettyPrint {
		logWriter = &zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.TimeOnly}
	}

	logger := zerolog.New(logWriter).Level(logLevel).With().Timestamp().Logger()

	log.Logger = logger
	zerolog.DefaultContextLogger = &logger
}

func getControllers(config config.Config) ([]models.Controller, error) {
	buildStatus := build_models.NewPipelineBadge()
	applicatinoFactory := applications.NewApplicationHandlerFactory(config)
	prometheusClient, err := metrics.NewPrometheusClient(config.PrometheusUrl)
	if err != nil {
		return nil, err
	}
	prometheusHandler := metrics.NewPrometheusHandler(prometheusClient)
	return []models.Controller{
		applications.NewApplicationController(nil, applicatinoFactory, prometheusHandler),
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
