// Radix Api Server.
// This is the API Server for the Radix platform.
// Schemes: https, http
// BasePath: /api/v1
// Version: 0.0.15
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

func main() {
	fs := initializeFlagSet()

	var (
		port = fs.StringP("port", "p", defaultPort(), "Port where API will be served")
	)

	parseFlagsFromArgs(fs)

	log.Infof("Api is serving on port %s", *port)
	err := http.ListenAndServe(fmt.Sprintf(":%s", *port), routers.NewServer())
	if err != nil {
		log.Fatalf("Unable to start serving: %v", err)
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
