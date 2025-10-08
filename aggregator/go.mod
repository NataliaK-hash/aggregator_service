module aggregator

go 1.24.0

require (
        github.com/DATA-DOG/go-sqlmock v1.0.0
        github.com/google/uuid v1.6.0
        github.com/google/wire v0.7.0
        github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
        github.com/jackc/pgx/v5 v5.0.0
        github.com/prometheus/client_golang v1.19.1
        github.com/testcontainers/testcontainers-go v0.0.0
        go.uber.org/fx v1.24.0
        google.golang.org/grpc v1.67.1
        google.golang.org/protobuf v0.0.0
)

require (
        go.uber.org/dig v1.19.0 // indirect
        go.uber.org/multierr v1.11.0 // indirect
        go.uber.org/zap v1.27.0 // indirect
        golang.org/x/sys v0.36.0 // indirect
)

replace github.com/jackc/pgx/v5 => ./third_party/github.com/jackc/pgx/v5

replace github.com/DATA-DOG/go-sqlmock => ./third_party/github.com/DATA-DOG/go-sqlmock

replace github.com/testcontainers/testcontainers-go => ./third_party/github.com/testcontainers/testcontainers-go

replace github.com/google/uuid => ./internal/uuidstub

replace github.com/google/wire => ./third_party/github.com/google/wire

replace github.com/grpc-ecosystem/go-grpc-prometheus => ./third_party/github.com/grpc-ecosystem/go-grpc-prometheus

replace github.com/prometheus/client_golang => ./third_party/github.com/prometheus/client_golang

replace google.golang.org/grpc => ./third_party/google.golang.org/grpc

replace google.golang.org/protobuf => ./third_party/google.golang.org/protobuf
