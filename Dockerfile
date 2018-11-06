FROM golang:alpine3.7 as builder
RUN apk update && apk add bash && apk add sed && apk add gawk && apk add git && apk add -y ca-certificates curl && \
    curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

RUN mkdir -p /go/src/github.com/statoil/radix-api/vendor/github.com/statoil/radix-operator/pkg
RUN mkdir -p /go/src/github.com/statoil/radix-api/hack

WORKDIR /go/src/github.com/statoil/radix-api/
COPY Gopkg.toml Gopkg.lock ./
COPY ./hack/removeDependencyToPrivateRepo.sh ./hack/

# Remove dependeny on operator which is manually added to vendor folder
RUN sed -ri '/### REMOVE IN DOCKERFILE/,/### END REMOVE/d' ./Gopkg.toml
RUN ./hack/removeDependencyToPrivateRepo.sh "github.com/statoil/radix-operator" "[[projects]]" "./Gopkg.lock"

#RUN lineNumOfOperator="$(grep -n 'github.com/statoil/radix-operator' ./Gopkg.lock | head -n 1 | cut -d: -f1)"
#RUN cutLockFileFrom="$(expr $lineNumOfOperator - 2)"
#RUN textToCut="$(awk '{ print NR, $1 }' ./Gopkg.lock | awk 'NR=='$cutLockFileFrom',/[[projects]]/')"
#RUN lineNumbersToCut="$(awk '{ print NR, $1 }' ./Gopkg.lock | awk 'NR=='$cutLockFileFrom',/[[projects]]/' | cut -d ' ' -f1)"
#RUN cutLockFileTo= lineNumbersToCut | awk '{print $NF}'

ADD ./vendor/github.com/statoil/radix-operator/pkg/ /vendor/github.com/statoil/radix-operator/pkg
RUN dep ensure -vendor-only
COPY . .
WORKDIR /go/src/github.com/statoil/radix-api/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -a -installsuffix cgo -o /usr/local/bin/radix-api
RUN adduser -D -g '' radix-api

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /usr/local/bin/radix-api /usr/local/bin/radix-api
USER radix-api
EXPOSE 3001
ENTRYPOINT ["/usr/local/bin/radix-api"]