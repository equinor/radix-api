# Build stage
FROM docker.io/golang:1.22-alpine3.20 as builder
ENV CGO_ENABLED=0 \
    GOOS=linux
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o /build/radix-api

# Final stage, ref https://github.com/GoogleContainerTools/distroless/blob/main/base/README.md for distroless
FROM gcr.io/distroless/static
WORKDIR /app
COPY --from=builder /build/radix-api .
USER 1000
ENTRYPOINT ["/app/radix-api"]
