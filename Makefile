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
	go test -v -count=1 ./app/tests/unit/...
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
	@echo "Running integration tests..."
	@docker info > /dev/null 2>&1 || (echo "Docker not available, skipping tests." && exit 0)
	go test -v -count=1 ./app/tests/integration/...
# -------------------------------
# Clean build artifacts
# -------------------------------
clean:
	@echo "Cleaning..."
	@if [ -d $(BIN_DIR) ]; then rm -rf $(BIN_DIR); fi

# -------------------------------
# Run database migrations (cross-platform)
# -------------------------------
UNAME_S := $(shell uname -s)

migrate:
	@echo "Running migrations..."
ifeq ($(OS),Windows_NT)
	@echo "Detected OS: Windows"
	powershell -Command "Start-Sleep -Seconds 3"
	go run ./app/src/cmd/migrate/main.go --dir $(MIGRATIONS_DIR)
else ifeq ($(UNAME_S),Darwin)
	@echo "Detected OS: macOS"
	sleep 3
	go run ./app/src/cmd/migrate/main.go --dir $(MIGRATIONS_DIR)
else ifeq ($(UNAME_S),Linux)
	@echo "Detected OS: Linux"
	sleep 3
	go run ./app/src/cmd/migrate/main.go --dir $(MIGRATIONS_DIR)
else
	@echo "⚠️ Unsupported OS detected: $(UNAME_S)"
endif
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

# -------------------------------------
# Run docker-compose-test (with cleanup)
# -------------------------------------
docker-test:
	@echo "Stopping and removing old containers..."
	docker-compose -f docker-compose-test.yml down --remove-orphans
	@echo "Starting fresh docker-compose-test containers..."
	docker-compose -f docker-compose-test.yml up --build --force-recreate -d
	@echo "✅ docker-compose-test environment is up and running!"

# -------------------------------------
# Run full setup: docker + app
# -------------------------------------
run-full:
	@echo "🚀 Start update dependency"
	$(MAKE) deps
	@echo "🚀 Build project"
	$(MAKE) build
	@echo "🚀 Starting full environment (Docker + App)..."
	$(MAKE) docker-test
	@echo "🚀  Starting application..."
	$(MAKE) run
	@echo "✅ Application is running!"