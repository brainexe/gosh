
.PHONY: build test lint clean all test-coverage

BINARY_NAME=gosh
BUILD_DIR=build

all: test lint build

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 go build -mod=mod -trimpath -ldflags="-s -w -extldflags '-static'" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd

test:
	@echo "Running tests..."
	@go test -mod=mod ./...

test-race:
	@echo "Running tests with race detection..."
	@go test -mod=mod -race -v ./...

test-coverage:
	@echo "Running tests with coverage..."
	@mkdir -p $(BUILD_DIR)
	@go test -mod=mod -coverprofile=$(BUILD_DIR)/coverage.out ./...
	@go tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/cover.html

lint:
	@echo "Running linter..."
	@golangci-lint run --fix

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)

test-integration: build
	@echo "Running integration tests..."
	@./test_integration.sh
