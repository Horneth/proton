#!/bin/bash
set -e

export PATH=$PATH:$(go env GOPATH)/bin

IMAGE="canton_buf_image.binpb"

if [ ! -f "$IMAGE" ]; then
    echo "Error: $IMAGE not found."
    echo "Please build it from the Canton repository using the command provided in the instructions,"
    echo "and copy it to the root of this repository."
    exit 1
fi

echo "Generating Go code from $IMAGE..."
mkdir -p pkg/daml/proto
buf generate "$IMAGE" --template buf.gen.daml.yaml

# Remove conflicting standard Google protos (they are handled via genproto overrides)
echo "Cleaning up standard Google protos..."
rm -rf pkg/daml/proto/google

echo "Done."
