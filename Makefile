ENVIRONMENT ?= dev

CONTAINER_REPO ?= radix$(ENVIRONMENT)
DOCKER_REGISTRY	?= $(CONTAINER_REPO).azurecr.io

BINS	= radix-api
IMAGES	= radix-api

GIT_TAG		= $(shell git describe --tags --always 2>/dev/null)
CURRENT_FOLDER = $(shell pwd)
VERSION		?= ${GIT_TAG}
IMAGE_TAG 	?= ${VERSION}
LDFLAGS		+= -s -w

CX_OSES		= linux windows
CX_ARCHS	= amd64

.PHONY: build
build: $(BINS)

.PHONY: mocks
mocks: bootstrap
	mockgen -source ./api/buildstatus/models/buildstatus.go -destination ./api/test/mock/buildstatus_mock.go -package mock
	mockgen -source ./api/deployments/deployment_handler.go -destination ./api/deployments/mock/deployment_handler_mock.go -package mock
	mockgen -source ./api/metrics/prometheus_handler.go -destination ./api/metrics/mock/prometheus_handler_mock.go -package mock
	mockgen -source ./api/metrics/prometheus_client.go -destination ./api/metrics/mock/prometheus_client_mock.go -package mock
	mockgen -source ./api/environments/job_handler.go -destination ./api/environments/mock/job_handler_mock.go -package mock
	mockgen -source ./api/environments/environment_handler.go -destination ./api/environments/mock/environment_handler_mock.go -package mock
	mockgen -source ./api/utils/tlsvalidation/interface.go -destination ./api/utils/tlsvalidation/mock/tls_secret_validator_mock.go -package mock
	mockgen -source ./api/events/event_handler.go -destination ./api/events/mock/event_handler_mock.go -package mock
	mockgen -source ./api/environmentvariables/env_vars_handler.go -destination ./api/environmentvariables/env_vars_handler_mock.go -package environmentvariables
	mockgen -source ./api/environmentvariables/env_vars_handler_factory.go -destination ./api/environmentvariables/env_vars_handler_factory_mock.go -package environmentvariables

.PHONY: test
test:
	go test -cover `go list ./...`

.PHONY: lint
lint: bootstrap
	golangci-lint run --max-same-issues 0 --timeout 10m

build-kaniko:
	docker run --rm -it -v $(CURRENT_FOLDER):/workspace gcr.io/kaniko-project/executor:latest --destination=$(DOCKER_REGISTRY)/radix-api-server:3hv6o --snapshotMode=time --cache=true

# This make command is only needed for local testing now
# we also do make swagger inside Dockerfile
.PHONY: swagger
swagger: SHELL:=/bin/bash
swagger: bootstrap
	swagger generate spec -o ./swaggerui/html/swagger.json --scan-models --exclude-deps
	swagger validate ./swaggerui/html/swagger.json

.PHONY: $(BINS)
$(BINS): bootstrap
	go build -ldflags '$(LDFLAGS)' -o bin/$@ .

.PHONY: docker-build
docker-build: $(addsuffix -image,$(IMAGES))

%-image:
	docker build $(DOCKER_BUILD_FLAGS) -t $(DOCKER_REGISTRY)/$*-server:$(IMAGE_TAG) .

.PHONY: docker-push
docker-push: $(addsuffix -push,$(IMAGES))

%-push:
	az acr login --name $(CONTAINER_REPO)
	docker push $(DOCKER_REGISTRY)/$*-server:$(IMAGE_TAG)

.PHONY: deploy
deploy: $(addsuffix -image,$(IMAGES)) $(addsuffix -push,$(IMAGES))

.PHONY: generate
generate: mocks swagger

.PHONY: verify-generate
verify-generate: generate
	git diff --exit-code

HAS_SWAGGER       := $(shell command -v swagger;)
HAS_GOLANGCI_LINT := $(shell command -v golangci-lint;)
HAS_MOCKGEN       := $(shell command -v mockgen;)

bootstrap:
ifndef HAS_SWAGGER
	go install github.com/go-swagger/go-swagger/cmd/swagger@v0.31.0
endif
ifndef HAS_GOLANGCI_LINT
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.58.2
endif
ifndef HAS_MOCKGEN
	go install github.com/golang/mock/mockgen@v1.6.0
endif
