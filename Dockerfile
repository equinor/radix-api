FROM golang:alpine3.7 as builder
RUN apk update && \
    apk add bash alpine-sdk sed gawk git ca-certificates curl && \
    apk add --no-cache gcc musl-dev && \
    go get -u golang.org/x/lint/golint && \
    curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh && \
    go get -u github.com/go-swagger/go-swagger/cmd/swagger github.com/rakyll/statik

WORKDIR /go/src/github.com/equinor/radix-api/

COPY Gopkg.toml Gopkg.lock ./
RUN dep ensure -vendor-only

COPY . .

# Generate swagger + add default user
RUN rm -f ./swaggerui_src/swagger.json ./swaggerui/statik.go && \
    swagger generate spec -o ./swagger.json --scan-models && \
    mv swagger.json ./swaggerui_src/swagger.json && \
    statik -src=./swaggerui_src/ -p swaggerui

RUN golint `go list ./...` && \
    go vet `go list ./...` && \
    CGO_ENABLED=0 GOOS=linux go test ./...

# Build radix api go project
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -a -installsuffix cgo -o /usr/local/bin/radix-api
# Until cache working together with user command
# https://github.com/GoogleContainerTools/kaniko/issues/477

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/local/bin/radix-api /usr/local/bin/radix-api
EXPOSE 3001
ENTRYPOINT ["/usr/local/bin/radix-api"]