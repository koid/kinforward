FROM golang:1.10.3-alpine AS builder
RUN apk add --update git
RUN go get -u github.com/golang/dep/cmd/dep
ENV GOPATH /go
ENV GOOS linux
ENV GOARCH amd64
WORKDIR /go/src/github.com/koid/kinforward
COPY . ./
RUN dep ensure
RUN go build -o kinforward

FROM alpine:latest
RUN apk add --update --no-cache ca-certificates && update-ca-certificates
COPY --from=builder /go/src/github.com/koid/kinforward/kinforward /bin/kinforward
