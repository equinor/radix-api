FROM alpine:3.7

USER root
COPY ./rootfs/radix-api /usr/local/bin/radix-api
RUN adduser -D radix-api
RUN chmod u+x /usr/local/bin/radix-api
USER radix-api
CMD ["/usr/local/bin/radix-api"]