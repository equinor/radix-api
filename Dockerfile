FROM golang:alpine3.7 as builder
RUN apk update && apk add git && apk add -y ca-certificates curl
COPY rootfs/radix-api-go /usr/local/bin/radix-api-go
RUN adduser -D -g '' radix-operator

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /usr/local/bin/radix-api-go /usr/local/bin/
USER radix-operator
EXPOSE 3002
ENTRYPOINT ["/usr/local/bin/radix-api-go"]
