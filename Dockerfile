FROM golang:alpine3.10 as builder
RUN apk update && \
    apk add bash jq alpine-sdk sed gawk git ca-certificates curl dep && \
    apk add --no-cache gcc musl-dev && \
    go get -u golang.org/x/lint/golint && \
    go get -u github.com/rakyll/statik && \
    # Install go-swagger - 20822585=v0.21.0 - get release id from https://api.github.com/repos/go-swagger/go-swagger/releases
    download_url=$(curl -s https://api.github.com/repos/go-swagger/go-swagger/releases/20822585 | \
    jq -r '.assets[] | select(.name | contains("'"$(uname | tr '[:upper:]' '[:lower:]')"'_amd64")) | .browser_download_url') && \
    curl -o /usr/local/bin/swagger -L'#' "$download_url" && \
    chmod +x /usr/local/bin/swagger

WORKDIR /go/src/github.com/equinor/radix-api/

# get dependencies
COPY Gopkg.toml Gopkg.lock ./
RUN dep ensure -vendor-only

# copy api code
COPY . .

# Generate swagger + add default user
# RUN swagger generate spec -o ./swagger.json --scan-models && \
#     mv swagger.json ./swaggerui_src/swagger.json && \
#     statik -src=./swaggerui_src/ -p swaggerui

# lint and unit tests
RUN golint `go list ./...` && \
    go vet `go list ./...` && \
    CGO_ENABLED=0 GOOS=linux go test `go list ./...`

# Build radix api go project
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -a -installsuffix cgo -o /usr/local/bin/radix-api

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/local/bin/radix-api /usr/local/bin/radix-api
EXPOSE 3001
ENTRYPOINT ["/usr/local/bin/radix-api"]