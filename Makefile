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
	@PROTO_IMAGE=$(CURDIR)/$(IMAGE) go test -v ./pkg/...
	@PROTO_IMAGE=$(CURDIR)/$(IMAGE) go test -v ./tests/...

clean:
	@echo "Cleaning up..."
	@rm -rf bin/
	@rm -rf pkg/daml/proto
	@echo "Note: $(IMAGE) is preserved as it is a required source."

# Helper for rebuilding everything from scratch
rebuild: clean generate build test

# Variables for refreshing the Buf image
CANTON_REPO=https://github.com/digital-asset/canton.git
CANTON_VERSION?=main
BUILD_IMAGE_SCRIPT=scripts/build_canton_buf_image.sh

refresh-image:
	@echo "Refreshing Buf image using Canton $(CANTON_VERSION)..."
	@TMP_DIR=$$(mktemp -d); \
	git clone --depth 1 --branch $(CANTON_VERSION) $(CANTON_REPO) "$$TMP_DIR" || git clone $(CANTON_REPO) "$$TMP_DIR" && \
	cd "$$TMP_DIR" && git checkout $(CANTON_VERSION) && \
	cp "$(CURDIR)/$(BUILD_IMAGE_SCRIPT)" "$$TMP_DIR/build_canton_buf_image.sh" && \
	chmod +x "./build_canton_buf_image.sh" && \
	./build_canton_buf_image.sh && \
	cp "canton_buf_image.binpb" "$(CURDIR)/$(IMAGE)" && \
	echo "Image refreshed successfully."
