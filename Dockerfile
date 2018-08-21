FROM golang:alpine as builder
RUN apk update && apk add git && apk add ca-certificates
RUN adduser -D -g '' appuser
COPY ./app/ $GOPATH/src/mypackage/myapp/
WORKDIR $GOPATH/src/mypackage/myapp/
RUN go get -d -v
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags='-w -s' -o /go/bin/app

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /go/bin/app /go/bin/app
USER appuser
EXPOSE 8080
ENTRYPOINT ["/go/bin/app"]
