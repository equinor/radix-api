FROM golang

COPY ./app /go/src/github.com/user/aProject/app
WORKDIR /go/src/github.com/user/aProject/app

RUN go get ./
RUN go build

#RUN go get -u -v github.com/derekparker/delve/cmd/dlv
#EXPOSE 2345

#RUN go get github.com/pilu/fresh
RUN go get github.com/codegangsta/gin

EXPOSE 3000
EXPOSE 3001
# CMD	dlv debug --headless --listen=:2345 --log && fresh
CMD gin run app.go