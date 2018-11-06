DOCKER_REGISTRY	?= radixdev.azurecr.io

BINS	= radix-api
IMAGES	= radix-api

GIT_TAG		= $(shell git describe --tags --always 2>/dev/null)
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

# This make command is only needed for local testing now
# we also do make swagger inside Dockerfile
.PHONY: swagger
swagger:
	rm -f ./swaggerui_src/swagger.json ./swaggerui/statik.go
	swagger generate spec -o ./swagger.json --scan-models
	mv swagger.json ./swaggerui_src/swagger.json
	statik -src=./swaggerui_src/ -p swaggerui

deploy-gitclone:
	docker build -t radixdev.azurecr.io/gitclone:$(IMAGE_TAG) -f gitclone.Dockerfile .
	docker push radixdev.azurecr.io/gitclone:$(IMAGE_TAG)

deploy-api:
	make docker-build
	make docker-push

.PHONY: deploy
deploy:
	# Add and update ACR Helm repo
	az acr helm repo add --name radixdev && helm repo update

	# Download deploy key and other secrets
	az keyvault secret download -f radix-api-radixregistration-values.yaml -n radix-api-radixregistration-values --vault-name radix-boot-dev-vault

	# Install RR
	helm upgrade --install radix-api -f radix-api-radixregistration-values.yaml radixdev/radix-registration
	# Delete secret file to avvoid being checked in
	rm radix-api-radixregistration-values.yaml
	
	# Allow operator to pick up RR. TODO should be handled with waiting for app namespace
	sleep 5	
	
	# Create pipeline
	helm upgrade --install radix-pipeline-api radixdev/radix-pipeline-invocation --set name="radix-api" --set cloneURL="git@github.com:Statoil/radix-api.git" --set cloneBranch="master"

.PHONY: undeploy
undeploy:
	helm delete --purge radix-pipeline-api
	helm delete --purge radix-api

.PHONY: $(BINS)
$(BINS): vendor
	go build -ldflags '$(LDFLAGS)' -o bin/$@ .

build-docker-bins: $(addsuffix -docker-bin,$(BINS))
%-docker-bin: vendor
	GOOS=linux GOARCH=$(CX_ARCHS) CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o ./rootfs/$* .

.PHONY: docker-build
docker-build: build-docker-bins
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
