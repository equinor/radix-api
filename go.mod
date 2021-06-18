module github.com/equinor/radix-api

go 1.16

require (
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/equinor/radix-operator v1.13.2
	github.com/evanphx/json-patch v4.9.0+incompatible
	github.com/go-openapi/strfmt v0.20.1
	github.com/golang/gddo v0.0.0-20190301051549-9dbec5838451
	github.com/golang/mock v1.5.0
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/marstr/guid v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus-operator/prometheus-operator v0.44.0
	github.com/prometheus/client_golang v1.9.0
	github.com/rakyll/statik v0.1.7
	github.com/rs/cors v1.7.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/urfave/negroni v1.0.0
	golang.org/x/crypto v0.0.0-20201002170205-7f63de1d35b0
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.19.9
	k8s.io/apimachinery v0.19.9
	k8s.io/client-go v12.0.0+incompatible
)

replace k8s.io/client-go => k8s.io/client-go v0.19.9
