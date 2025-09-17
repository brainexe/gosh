
.PHONY: build test lint clean all

BINARY_NAME=gosh
BUILD_DIR=build

all: test lint build

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -mod=mod -trimpath -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd

test:
	@echo "Running tests..."
	@go test -mod=mod -v ./...

test-race:
	@echo "Running tests with race detection..."
	@go test -mod=mod -race -v ./...

lint:
	@echo "Running linter..."
	@golangci-lint run --fix

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)

test-integration: build
	@echo "Running integration tests..."
	@./test_integration.sh
