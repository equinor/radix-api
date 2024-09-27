package cors

import (
	"fmt"

	"github.com/rs/cors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func CreateMiddleware(clusterName, radixDNSZone string) *cors.Cors {

	corsOptions := cors.Options{
		AllowedOrigins: []string{
			"http://localhost:3000",
			"http://localhost:3001",
			"http://127.0.0.1:3000",
			"http://localhost:8000",
			"http://localhost:8086", // For swaggerui testing
			// TODO: We should consider:
			// 1. "https://*.radix.equinor.com"
			// 2. Keep cors rules in ingresses
			fmt.Sprintf("https://console.%s", radixDNSZone),
			getHostName("web", "radix-web-console-qa", clusterName, radixDNSZone),
			getHostName("web", "radix-web-console-prod", clusterName, radixDNSZone),
			getHostName("web", "radix-web-console-dev", clusterName, radixDNSZone),
			// Due to active-cluster
			getActiveClusterHostName("web", "radix-web-console-qa", radixDNSZone),
			getActiveClusterHostName("web", "radix-web-console-prod", radixDNSZone),
			getActiveClusterHostName("web", "radix-web-console-dev", radixDNSZone),
		},
		AllowCredentials: true,
		MaxAge:           600,
		AllowedHeaders:   []string{"Accept", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization"},
		AllowedMethods:   []string{"GET", "PUT", "POST", "OPTIONS", "DELETE", "PATCH"},
	}

	if zerolog.GlobalLevel() <= zerolog.DebugLevel {
		// debugging mode
		corsOptions.Debug = true
		corsLogger := log.Logger.With().Str("pkg", "cors-middleware").Logger()
		corsOptions.Logger = &corsLogger
		// necessary header to allow ajax requests directly from radix-web-console app in browser
		corsOptions.AllowedHeaders = append(corsOptions.AllowedHeaders, "X-Requested-With")
	}

	c := cors.New(corsOptions)

	return c
}

func getActiveClusterHostName(componentName, namespace, radixDNSZone string) string {
	return fmt.Sprintf("https://%s-%s.%s", componentName, namespace, radixDNSZone)
}

func getHostName(componentName, namespace, clustername, radixDNSZone string) string {
	return fmt.Sprintf("https://%s-%s.%s.%s", componentName, namespace, clustername, radixDNSZone)
}
