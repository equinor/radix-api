FROM golang

COPY ./app /go/src/github.com/user/aProject/app
WORKDIR /go/src/github.com/user/aProject/app

RUN go get ./
RUN go build

CMD go get github.com/pilu/fresh && \
	fresh; \
	fi
	
EXPOSE 8080
