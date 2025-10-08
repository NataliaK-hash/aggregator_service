BINARY := aggregator
CMD := ./cmd/aggregator
BIN_DIR := ./bin
MIGRATIONS_DIR := ./internal/storage/postgres/migrations

.PHONY: all build run test lint clean migrate

all: build test

build:
	@echo "Building $(BINARY)..."
	go build -o $(BIN_DIR)/$(BINARY) $(CMD)

run:
	@echo "Running $(BINARY)..."
	go run $(CMD)

test:
	@echo "Running tests..."
	go test ./...

clean:
	@echo "Cleaning..."
	- rm -rf $(BIN_DIR)

migrate:
	@echo "Running migrations..."
	# команды миграций, если нужны
