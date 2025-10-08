BINARY := aggregator
CMD := ./cmd/aggregator

.PHONY: all build run test lint wire clean

all: build

wire:
	cd internal/app && wire

build: wire
	go build -o bin/$(BINARY) $(CMD)

run:
	go run $(CMD)

test:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin
