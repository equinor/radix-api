FROM alpine:3.9

RUN apk add --no-cache bash openssh-client git
ENV GIT_SSH_COMMAND="ssh -i /root/.ssh/id_rsa -o UserKnownHostsFile=/root/dynamic_known_host"

VOLUME /workspace
WORKDIR /workspace

RUN mkdir /root/bin
COPY docker-entrypoint.sh /root/bin/docker-entrypoint.sh
RUN chmod +x /root/bin/docker-entrypoint.sh

RUN sh /root/bin/docker-entrypoint.sh
RUN chmod +r /root/dynamic_known_host

CMD ["/bin/sh", "-c"]