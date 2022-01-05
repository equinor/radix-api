module github.com/equinor/radix-api

go 1.16

require (
	github.com/equinor/radix-common v1.1.8
	github.com/equinor/radix-job-scheduler v1.3.1
	github.com/equinor/radix-operator v1.16.10-0.20220105121243-00f9881d12b4
	github.com/evanphx/json-patch v4.11.0+incompatible
	github.com/go-openapi/strfmt v0.20.1
	github.com/golang-jwt/jwt/v4 v4.1.0
	github.com/golang/mock v1.5.0
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/marstr/guid v1.1.0
	github.com/prometheus-operator/prometheus-operator v0.44.0
	github.com/prometheus/client_golang v1.11.0
	github.com/rakyll/statik v0.1.7
	github.com/rs/cors v1.7.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/urfave/negroni v1.0.0
	golang.org/x/crypto v0.0.0-20210220033148-5ea612d1eb83
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.22.4
	k8s.io/apimachinery v0.22.4
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/secrets-store-csi-driver v1.0.0
)

//github.com/equinor/radix-operator => /home/user1/go/src/github.com/equinor/radix-operator
replace (
	github.com/equinor/radix-operator => /home/user1/go/src/github.com/equinor/radix-operator
	k8s.io/client-go => k8s.io/client-go v0.22.4
)
