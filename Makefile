# Makefile for Proton Toolkit

.PHONY: all build clean cross-compile

# Binary names
BIN_NAME=proton

# Build directory
BUILD_DIR=bin

all: build

build:
	@echo "Building proton binary..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BIN_NAME) ./cmd/proton
	@echo "Done. Binary is in $(BUILD_DIR)/"

test: build
	@if [ -z "$$PROTO_IMAGE" ]; then \
		echo "ERROR: PROTO_IMAGE must be set for engine and integration tests."; \
		exit 1; \
	fi
	@echo "Running all tests..."
	go test -v ./...

clean:
	@echo "Cleaning up..."
	rm -rf $(BUILD_DIR)

cross-compile:
	@echo "Cross-compiling for Linux and Windows..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BIN_NAME)-linux-amd64 ./cmd/proton
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BIN_NAME)-windows-amd64.exe ./cmd/proton
	@echo "Cross-compilation complete."
