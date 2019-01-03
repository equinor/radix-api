FROM golang:alpine3.7 as builder
RUN apk update && apk add bash && apk add alpine-sdk && apk add sed && apk add gawk && apk add git && apk add -y ca-certificates curl && \
    curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

RUN mkdir -p /go/src/github.com/statoil/radix-api/vendor/github.com/statoil/radix-operator/pkg
RUN mkdir -p /go/src/github.com/statoil/radix-api/hack

# Install swagger and statik
RUN go get -u github.com/go-swagger/go-swagger/cmd/swagger
RUN go get github.com/rakyll/statik

WORKDIR /go/src/github.com/statoil/radix-api/
COPY Gopkg.toml Gopkg.lock ./
COPY ./hack/removeDependencyToPrivateRepo.sh ./hack/
RUN chmod +x ./hack/removeDependencyToPrivateRepo.sh

# Remove dependeny on operator which is manually added to vendor folder
RUN sed -ri '/### REMOVE IN DOCKERFILE/,/### END REMOVE/d' ./Gopkg.toml
RUN ./hack/removeDependencyToPrivateRepo.sh "github.com/statoil/radix-operator" "[[projects]]" "./Gopkg.lock"

ADD ./vendor/github.com/statoil/radix-operator/pkg/ /vendor/github.com/statoil/radix-operator/pkg
RUN dep ensure -vendor-only

COPY . .
WORKDIR /go/src/github.com/statoil/radix-api/

# Generate swagger
RUN rm -f ./swaggerui_src/swagger.json ./swaggerui/statik.go
RUN swagger generate spec -o ./swagger.json --scan-models
RUN mv swagger.json ./swaggerui_src/swagger.json
RUN statik -src=./swaggerui_src/ -p swaggerui

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -a -installsuffix cgo -o /usr/local/bin/radix-api
# Until cache working together with user command
# https://github.com/GoogleContainerTools/kaniko/issues/477
# RUN adduser -D -g '' radix-api

# FROM scratch
FROM alpine3.7
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /usr/local/bin/radix-api /usr/local/bin/radix-api
RUN adduser -D -g '' radix-api
USER radix-api
EXPOSE 3001
ENTRYPOINT ["/usr/local/bin/radix-api"]