FROM golang:1.16.5-alpine3.13 as builder
ENV GO111MODULE=on

RUN addgroup -S -g 1000 api-user
RUN adduser -S -u 1000 -G api-user api-user

RUN apk update && \
    apk add bash jq alpine-sdk sed gawk git ca-certificates curl && \
    apk add --no-cache gcc musl-dev && \
    go get -u golang.org/x/lint/golint && \
    go get -u github.com/rakyll/statik && \
    # Install go-swagger - 28704370=v0.25.0 - get release id from https://api.github.com/repos/go-swagger/go-swagger/releases
    download_url=$(curl -s https://api.github.com/repos/go-swagger/go-swagger/releases/28704370 | \
    jq -r '.assets[] | select(.name | contains("'"$(uname | tr '[:upper:]' '[:lower:]')"'_amd64")) | .browser_download_url') && \
    curl -o /usr/local/bin/swagger -L'#' "$download_url" && \
    chmod +x /usr/local/bin/swagger

WORKDIR /go/src/github.com/equinor/radix-api/

# get dependencies
COPY go.mod go.sum ./
RUN go mod download

# copy api code
COPY . .

# Generate swagger
RUN swagger generate spec -o ./swaggerui_src/swagger.json --scan-models && \
    statik -src=./swaggerui_src/ -p swaggerui

# lint and unit tests
RUN golint `go list ./...` && \
    go vet `go list ./...` && \
    CGO_ENABLED=0 GOOS=linux go test `go list ./...`

# Build radix api go project
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -a -installsuffix cgo -o /usr/local/bin/radix-api

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /usr/local/bin/radix-api /usr/local/bin/radix-api

EXPOSE 3001
USER 1000
ENTRYPOINT ["/usr/local/bin/radix-api"]