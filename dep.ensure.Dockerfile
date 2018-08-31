FROM golang:alpine as builder
RUN apk update && apk add git ca-certificates unzip curl
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
RUN adduser -D -g '' appuser
COPY Gopkg.toml Gopkg.lock $GOPATH/src/package/app/
WORKDIR $GOPATH/src/package/app/
RUN dep ensure -vendor-only
COPY ./app/ $GOPATH/src/package/app/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags='-w -s' -o /go/bin/app

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /go/bin/app /go/bin/app
USER appuser
EXPOSE 8080
ENTRYPOINT ["/go/bin/app"]
