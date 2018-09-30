// Radix Api Server.
// This is the API Server for the Radix platform.
// Schemes: http, https
// BasePath: /api/v1
// Version: 0.0.1
// Contact: Inge Knudsen <iknu@equinor.com>
//
// Consumes:
// - application/json
//
// Produces:
// - application/json
//
// Security:
// - bearer
//
// SecurityDefinitions:
// bearer:
//   type: apiKey
//   name: Authorization
//   in: header
//
// swagger:meta
package main

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/rs/cors"
	"github.com/spf13/pflag"
	routers "github.com/statoil/radix-api-go/api"

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
	apiRouter := routers.NewServer()

	logrus.Infof("Api is serving on port %s", *port)
	err := http.ListenAndServe(fmt.Sprintf(":%s", *port), getHandler(apiRouter))
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

func getHandler(apiRouter *routers.Server) http.Handler {
	c := cors.New(cors.Options{
		AllowedOrigins: []string{
			"http://localhost:3000", // For socket.io testing
			"http://localhost:3001", // For socket.io testing
			"http://localhost:8086", // For swagger testing
		},
		AllowCredentials: true,
	})

	return c.Handler(apiRouter.Middleware)
}
