FROM golang:1.21-alpine AS builder

WORKDIR /go/src/github.com/guillembonet/pi-metrics
ADD . .
RUN go build -o build/pi-metrics .

FROM alpine:3.18

RUN apk update && apk upgrade

COPY --from=builder /go/src/github.com/guillembonet/pi-metrics/build/pi-metrics /usr/bin/pi-metrics

ENTRYPOINT ["/usr/bin/pi-metrics"]
