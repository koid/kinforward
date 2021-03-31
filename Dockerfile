FROM golang:1.16.2-alpine AS builder
RUN apk add --update git
ENV GOPATH /go
ENV GOOS linux
ENV GOARCH amd64
WORKDIR /go/src/github.com/koid/kinforward
COPY . ./
RUN go build -o kinforward

FROM alpine:latest
RUN apk add --update --no-cache ca-certificates && update-ca-certificates
COPY --from=builder /go/src/github.com/koid/kinforward/kinforward /bin/kinforward
