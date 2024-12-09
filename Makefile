
# Binary name
BINARY_NAME   = gosh

# Build output directory
BUILD_DIR     = build

.PHONY: all build test

all: test lint build

build:
	@echo "Building the application..."
	go build -trimpath -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd

test:
	@echo "Running tests..."
	go test -v ./...

test-race:
	@echo "Running tests..."
	go test -race -v ./...

lint:
	 golangci-lint run --fix
