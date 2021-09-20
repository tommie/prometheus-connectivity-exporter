FROM golang:1.16-alpine AS builder

WORKDIR /go/src/app

# This pre-copy is to allow caching even when the source is updated.
COPY go.mod go.sum ./
RUN go mod download

COPY . ./

ENV CGO_ENABLED=0
ENV CI=true

RUN go test ./...
RUN go install ./cmd/...

FROM alpine

COPY --from=builder /go/bin/promcond /usr/local/bin/

ENV HTTP_ADDR=0.0.0.0:9293
ENV CHECKS=

CMD promcond -standalone-log=false "-http-addr=$HTTP_ADDR" $CHECKS
