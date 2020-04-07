module github.com/equinor/radix-api

go 1.13

require (
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/equinor/radix-operator v1.5.9
	github.com/evanphx/json-patch v4.5.0+incompatible
	github.com/golang/gddo v0.0.0-20190301051549-9dbec5838451
	github.com/gorilla/mux v1.7.0
	github.com/graphql-go/graphql v0.7.7
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.1.0
	github.com/rakyll/statik v0.1.6
	github.com/rs/cors v1.6.0
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.4.0
	github.com/urfave/negroni v1.0.0
	golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.0.0-20191016225839-816a9b7df678
	k8s.io/apimachinery v0.0.0-20191020214737-6c8691705fc5
	k8s.io/client-go v12.0.0+incompatible
)

replace (
	github.com/prometheus/prometheus => github.com/prometheus/prometheus v0.0.0-20190818123050-43acd0e2e93f
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
)
