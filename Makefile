.PHONY: all build generate test clean

BINARY_NAME=bin/proton
DAML_GEN_SCRIPT=scripts/generate_daml.sh
IMAGE=canton_buf_image.binpb

all: generate build test

generate:
	@echo "Generating Daml Protobuf code..."
	@./$(DAML_GEN_SCRIPT)

build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BINARY_NAME) ./cmd/proton

test:
	@echo "Running tests..."
	@go test -v ./pkg/...
	@go test -v ./tests/...

clean:
	@echo "Cleaning up..."
	@rm -rf bin/
	@rm -rf pkg/daml/proto
	@echo "Note: $(IMAGE) is preserved as it is a required source."

# Helper for rebuilding everything from scratch
rebuild: clean generate build test
