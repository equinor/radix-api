FROM alpine:3.7 as builder
RUN apk update && apk add git && apk add -y ca-certificates curl
RUN adduser -D -g '' radix-api
COPY ./rootfs/radix-api /usr/local/bin/radix-api

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /usr/local/bin/radix-api /usr/local/bin/
USER radix-api
EXPOSE 3002
ENTRYPOINT ["/usr/local/bin/radix-api"]
