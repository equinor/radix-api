FROM alpine:3.7

RUN apk add --no-cache bash openssh-client git

VOLUME /workspace
WORKDIR /workspace

CMD ["/bin/sh", "-c"]