FROM alpine:3.6

RUN adduser -D radix-operator
USER radix-operator
COPY rootfs/radix-api-go /usr/local/bin/radix-api-go
EXPOSE 3002
ENTRYPOINT ["/usr/local/bin/radix-api-go"]
