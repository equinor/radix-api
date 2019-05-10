FROM alpine:3.7

# TODO: Remove this when done
RUN mkdir /root/.ssh
COPY id_rsa /root/.ssh/id_rsa

RUN apk add --no-cache bash openssh-client git
ENV GIT_SSH_COMMAND="ssh -i /root/.ssh/id_rsa"

VOLUME /workspace
WORKDIR /workspace

RUN mkdir /root/bin
COPY ssh-keyscan.sh /root/bin/ssh-keyscan.sh
RUN chmod +x /root/bin/ssh-keyscan.sh

ENTRYPOINT [ "/root/bin/ssh-keyscan.sh"]
CMD ["-c"]