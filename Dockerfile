FROM golang:1.25.4-alpine3.22 AS builder

RUN apk add --no-cache git gcc musl-dev ca-certificates

WORKDIR /app

COPY src/go.mod src/go.sum /app/
RUN go mod download

COPY src/ /app/
RUN go build -o /app/sniproxy -ldflags="-s -w -X main.version=${VERSION}" .


FROM alpine:3.22.2

COPY --from=builder /app/sniproxy /usr/local/bin/sniproxy

WORKDIR /sniproxy
ENTRYPOINT ["/usr/local/bin/sniproxy"]
CMD ["-c", "/sniproxy/config.yaml"]
