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

.PHONY: swagger
swagger:
	rm -f ./swaggerui_src/swagger.json ./swaggerui/statik.go
	swagger generate spec -o ./swagger.json --scan-models
	mv swagger.json ./swaggerui_src/swagger.json
	statik -src=./swaggerui_src/ -p swaggerui

.PHONY: deploy
deploy:
	# Download deploy key + webhook shared secret
	az keyvault secret download -f ./charts/api-server/values.yaml -n radix-api-registration --vault-name radix-boot-dev-vault
	# Install RR referring to the downloaded secrets
	helm install -n radix-api-server ./charts/api-server/
	# Delete secret file to avvoid being checked in
	rm ./charts/api-server/values.yaml
	# Allow operator to pick up RR. TODO should be handled with waiting for app namespace
	sleep 5
	# Create pipeline job
	helm install -n radix-api-init-deploy ./charts/init-deploy/		

.PHONY: undeploy
undeploy:
	helm delete --purge radix-api-init-deploy
	helm delete --purge radix-api-server

.PHONY: $(BINS)
$(BINS): vendor
	go build -ldflags '$(LDFLAGS)' -o bin/$@ .

build-docker-bins: $(addsuffix -docker-bin,$(BINS))
%-docker-bin: vendor
	make swagger
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