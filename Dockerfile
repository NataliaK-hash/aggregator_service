# syntax=docker/dockerfile:1

FROM golang:1.24 AS builder

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY app ./app

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /workspace/bin/aggregator-service ./app/src/cmd/start

FROM alpine:3.20 AS runtime

RUN apk add --no-cache ca-certificates curl postgresql15-client

WORKDIR /app

COPY --from=builder /workspace/bin/aggregator-service ./aggregator-service
COPY --from=builder /workspace/app/resources ./app/resources

ENV HTTP_PORT=8080 \
    GRPC_PORT=50051 \
    DB_DSN=postgres://aggregator:aggregator@postgres:5432/aggregator?sslmode=disable \
    DB_HOST=postgres \
    DB_PORT=5432 \
    DB_USER=aggregator \
    DB_PASSWORD=aggregator \
    DB_NAME=aggregator

EXPOSE 8080 50051

HEALTHCHECK CMD curl -f http://localhost:8080/healthz || exit 1

CMD ["./aggregator-service"]
