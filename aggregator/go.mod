module aggregator

go 1.24.0

require (
        github.com/DATA-DOG/go-sqlmock v1.0.0
        github.com/google/uuid v1.6.0
        github.com/google/wire v0.7.0
        github.com/jackc/pgx/v5 v5.0.0
        github.com/testcontainers/testcontainers-go v0.0.0
        go.uber.org/fx v1.24.0
)

require (
go.uber.org/dig v1.19.0
go.uber.org/multierr v1.11.0
go.uber.org/zap v1.27.0
golang.org/x/sys v0.36.0
)

replace github.com/jackc/pgx/v5 => ./third_party/github.com/jackc/pgx/v5

replace github.com/DATA-DOG/go-sqlmock => ./third_party/github.com/DATA-DOG/go-sqlmock

replace github.com/testcontainers/testcontainers-go => ./third_party/github.com/testcontainers/testcontainers-go

replace github.com/google/uuid => ./internal/uuidstub

replace github.com/google/wire => ./third_party/github.com/google/wire
