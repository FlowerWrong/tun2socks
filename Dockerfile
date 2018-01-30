FROM golang:1.9

WORKDIR /go/src/app
COPY . .

RUN go get -d -v -u ./...
