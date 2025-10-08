# ==============================
# Go Aggregator Service Makefile
# ==============================

BINARY := aggregator
CMD := ./app/src/cmd/start
BIN_DIR := ./bin
MIGRATIONS_DIR ?= ./app/resources/db/migrations

.PHONY: all build run test test-integration clean migrate test-flow deps

all: build test

# -------------------------------
# Build binary
# -------------------------------
build:
	@echo "Building aggregator..."
	go build -o $(BIN_DIR)/$(BINARY) $(CMD)

# -------------------------------
# Run tests
# -------------------------------
test-unit:
	@echo "Running unit tests..."
	go test ./app/tests/unit/...

# -------------------------------
# Run application
# -------------------------------
run: migrate
	@echo "Running $(BINARY)..."
	go run $(CMD)

# -------------------------------
# Integration tests
# -------------------------------
test-integration:
	@echo Running integration tests...
	@docker info >nul 2>&1 || (echo Docker not available, skipping tests. && exit 0)
	@go test ./app/tests/integration/...

# -------------------------------
# Clean build artifacts
# -------------------------------
clean:
	@echo "Cleaning..."
	@if [ -d $(BIN_DIR) ]; then rm -rf $(BIN_DIR); fi

# -------------------------------
# Run database migrations
# -------------------------------
migrate:
	@echo "Running migrations..."
	go run ./app/src/cmd/migrate/main.go --dir $(MIGRATIONS_DIR)

# -------------------------------
# Refresh dependencies
# -------------------------------
deps:
	@echo "Syncing dependencies..."
	go mod tidy
	go mod download all

# ------------------------------------------------------------------------------
# Manual E2E test (runs only when triggered manually)
# ------------------------------------------------------------------------------
test-flow:
	@echo 🚀 Running full E2E flow...
	set RUN_E2E=1 && set E2E_FORCE=1 && set E2E_OUTPUT=pretty && go test -v -count=1 ./app/tests/e2e -run ^TestE2E$
