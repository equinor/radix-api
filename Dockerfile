FROM alpine:3.6

RUN adduser -D '' radix-api-go
COPY rootfs/radix-api-go /usr/local/bin/radix-api-go
EXPOSE 3002
USER radix-api-go
ENTRYPOINT ["/usr/local/bin/radix-api-go"]
