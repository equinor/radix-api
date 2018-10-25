// Radix Api Server.
// This is the API Server for the Radix platform.
// Schemes: https, http
// BasePath: /api/v1
// Version: 0.0.26
// Contact: https://equinor.slack.com/messages/CBKM6N2JY
//
// Consumes:
// - application/json
//
// Produces:
// - application/json
//
// SecurityDefinitions:
//   bearer:
//     type: apiKey
//     name: Authorization
//     in: header
//
// Security:
// - bearer:
//
// swagger:meta
package main

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/pflag"
	routers "github.com/statoil/radix-api/api"

	// Force loading of needed authentication library

	"net/http"
	"os"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const clusternameEnvironmentVariable = "radix-clustername"

func main() {
	fs := initializeFlagSet()

	var (
		port        = fs.StringP("port", "p", defaultPort(), "Port where API will be served")
		clusterName = os.Getenv(clusternameEnvironmentVariable)
		certPath    = os.Getenv("server_cert_path") // "/etc/webhook/certs/cert.pem"
		keyPath     = os.Getenv("server_key_path")  // "/etc/webhook/certs/key.pem"
		httpsPort   = "3000"
	)

	parseFlagsFromArgs(fs)

	errs := make(chan error)
	go func() {
		log.Infof("Api is serving on port %s", *port)
		err := http.ListenAndServe(fmt.Sprintf(":%s", *port), routers.NewServer(clusterName))
		errs <- err
	}()
	if certPath != "" && keyPath != "" {
		go func() {
			log.Infof("Api is serving on port %s", httpsPort)
			err := http.ListenAndServeTLS(fmt.Sprintf(":%s", httpsPort), certPath, keyPath, routers.NewServer(clusterName))
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
