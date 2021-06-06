FROM golang:latest

ENV GO111MODULE=auto

EXPOSE 8000

RUN mkdir -p /go/src/github.com/sarismet/gjg-sarismet

WORKDIR /go/src/github.com/sarismet/gjg-sarismet

COPY . /go/src/github.com/sarismet/gjg-sarismet

RUN chmod a+x /go/src/github.com/sarismet/gjg-sarismet

RUN go get github.com/go-redis/redis
RUN go get github.com/google/uuid
RUN go get github.com/lib/pq
RUN go get github.com/labstack/echo

CMD ["go","run","/go/src/github.com/sarismet/gjg-sarismet"]


