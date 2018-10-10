FROM alpine:3.7

RUN adduser -D radix-api
USER radix-api
COPY ./rootfs/radix-api /usr/local/bin/radix-api
ENTRYPOINT ["/usr/local/bin/radix-api"]