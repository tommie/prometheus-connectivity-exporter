FROM golang:1.16-alpine AS builder

WORKDIR /go/src/app
COPY . ./

ENV CGO_ENABLED=0

RUN go mod download
RUN go test ./...
RUN go install ./cmd/...


FROM alpine

COPY --from=builder /go/bin/promcond /usr/local/bin/

ENV HTTP_ADDR=0.0.0.0:9293

CMD promcond -standalone-log=false "-http-addr=$HTTP_ADDR"