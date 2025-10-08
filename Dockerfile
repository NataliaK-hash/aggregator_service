# syntax=docker/dockerfile:1

FROM golang:1.21 AS builder

WORKDIR /app

COPY go.mod go.sum ./
COPY github.com ./github.com
COPY internal ./internal
COPY cmd ./cmd
COPY api ./api

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/aggregator-service ./cmd/aggregator

FROM alpine:3.20 AS runtime

RUN apk add --no-cache ca-certificates curl postgresql15-client

WORKDIR /app

COPY --from=builder /app/bin/aggregator-service ./aggregator-service
COPY api ./api

ENV HTTP_PORT=8080 \
    GRPC_PORT=50051 \
    DB_DSN=postgres://aggregator:aggregator@postgres:5432/aggregator?sslmode=disable \
    DB_HOST=postgres \
    DB_PORT=5432 \
    DB_USER=aggregator \
    DB_PASSWORD=aggregator \
    DB_NAME=aggregator

EXPOSE 8080 50051

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 CMD curl -f http://localhost:${HTTP_PORT}/healthz || exit 1

CMD ["./aggregator-service"]
