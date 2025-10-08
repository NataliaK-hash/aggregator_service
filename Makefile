BINARY := aggregator
CMD := ./cmd/aggregator
BIN_DIR := ./bin
MIGRATIONS_DIR := ./aggregator/internal/storage/postgres/migrations

.PHONY: all build run test lint clean test-generator bench-generator migrate

# Запускаем всё по умолчанию (сборка + тесты)
all: build test

# Сборка бинарника
build:
	@echo "Building $(BINARY)..."
	cd aggregator && go build -o ../$(BIN_DIR)/$(BINARY) $(CMD)

# Запуск сервиса напрямую
run:
	@echo "Running $(BINARY)..."
	cd aggregator && go run $(CMD)

# Прогон всех тестов
test:
	@echo "Running tests..."
	cd aggregator && go test ./... -v

# Линтер (golangci-lint должен быть установлен локально)
lint:
	@echo "Running linter..."
	cd aggregator && golangci-lint run ./...

# Очистка артефактов
clean:
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)

# Тесты только для генератора
test-generator:
	@echo "Running generator tests..."
	cd aggregator && go test ./internal/generator -race -count=1

# Бенчмарки генератора
bench-generator:
        @echo "Running generator benchmarks..."
        cd aggregator && go test -bench=. ./internal/generator -benchmem -run=^$

# Применение миграций БД через golang-migrate
migrate:
        @if [ -z "$$DB_DSN" ]; then echo "DB_DSN environment variable is required"; exit 1; fi
        @echo "Running migrations..."
        migrate -path $(MIGRATIONS_DIR) -database $$DB_DSN up
