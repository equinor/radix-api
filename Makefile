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

.PHONY: test
test:
	go test -cover `go list ./...`

mocks:
	mockgen -source ./api/buildstatus/models/buildstatus.go -destination ./api/test/mock/buildstatus_mock.go -package mock
	mockgen -source ./api/deployments/deployment_handler.go -destination ./api/deployments/mock/deployment_handler_mock.go -package mock
	mockgen -source ./api/secrets/secret_handler.go -destination ./api/secrets/mock/secret_handler_mock.go -package mock
	mockgen -source ./api/environments/job_handler.go -destination ./api/environments/mock/job_handler_mock.go -package mock
	mockgen -source ./api/environments/environment_handler.go -destination ./api/environments/mock/environment_handler_mock.go -package mock

build-kaniko:
	docker run --rm -it -v $(CURRENT_FOLDER):/workspace gcr.io/kaniko-project/executor:v0.7.0 --destination=$(DOCKER_REGISTRY)/radix-api-server:3hv6o --snapshotMode=time --cache=true

# This make command is only needed for local testing now
# we also do make swagger inside Dockerfile
.PHONY: swagger
swagger:
	rm -f ./swaggerui_src/swagger.json ./swaggerui/statik.go
	swagger generate spec -o ./swagger.json --scan-models --exclude-deps
	swagger validate ./swagger.json
	mv swagger.json ./swaggerui_src/swagger.json
	statik -src=./swaggerui_src/ -p swaggerui

deploy-api:
	draft up

.PHONY: deploy
deploy:
	# Add and update ACR Helm repo
	az acr helm repo add --name $(CONTAINER_REPO) && helm repo update

	# Download deploy key and other secrets
	az keyvault secret download -f radix-api-radixregistration-values.yaml -n radix-api-radixregistration-values --vault-name radix-boot-dev-vault

	# Install RR
	helm upgrade --install radix-api -f radix-api-radixregistration-values.yaml $(CONTAINER_REPO)/radix-registration
	# Delete secret file to avvoid being checked in
	rm radix-api-radixregistration-values.yaml
	
	# Allow operator to pick up RR. TODO should be handled with waiting for app namespace
	sleep 5	
	
	# Create pipeline
	helm upgrade --install radix-pipeline-api $(CONTAINER_REPO)/radix-pipeline-invocation --set name="radix-api" --set cloneURL="git@github.com:Statoil/radix-api.git" --set cloneBranch="master"

.PHONY: undeploy
undeploy:
	helm delete --purge radix-pipeline-api
	helm delete --purge radix-api

.PHONY: $(BINS)
$(BINS): vendor
	go build -ldflags '$(LDFLAGS)' -o bin/$@ .

.PHONY: docker-build
docker-build: $(addsuffix -image,$(IMAGES))

%-image:
	docker build $(DOCKER_BUILD_FLAGS) -t $(DOCKER_REGISTRY)/$*:$(IMAGE_TAG) .

.PHONY: docker-push
docker-push: $(addsuffix -push,$(IMAGES))

%-push:
	docker push $(DOCKER_REGISTRY)/$*:$(IMAGE_TAG)

HAS_GOMETALINTER := $(shell command -v gometalinter;)
HAS_DEP          := $(shell command -v dep;)
HAS_GIT          := $(shell command -v git;)
HAS_SWAGGER      := $(shell command -v swagger;)
HAS_STATIK 		 := $(shell command -v statik;)

vendor:
ifndef HAS_GIT
	$(error You must install git)
endif
ifndef HAS_DEP
	go get -u github.com/golang/dep/cmd/dep
endif
ifndef HAS_GOMETALINTER
	go get -u github.com/alecthomas/gometalinter
	gometalinter --install
endif
	dep ensure
ifndef HAS_SWAGGER
	go get -u github.com/go-swagger/go-swagger/cmd/swagger
endif

ifndef HAS_STATIK
	go get github.com/rakyll/statik
endif 

.PHONY: bootstrap
bootstrap: vendor

staticcheck:
	staticcheck ./...