#!/bin/bash
set -e -o pipefail

# This script builds a Buf image for Daml Interactive Submission protos.
# Inspired by: https://raw.githubusercontent.com/digital-asset/canton/refs/heads/release-line-3.4/community/app/src/pack/examples/08-interactive-submission/setup.sh

ROOT_PATH=$(git rev-parse --show-toplevel)
echo "Working in Canton repo: $ROOT_PATH"

# 1. Path Discovery
# We search for the proto roots to be flexible with different repo layouts (ledger-api vs ledger-api-proto)
COMMUNITY_BASE_ROOT=$(find "$ROOT_PATH" -type d -path "*/community/base/src/main/protobuf" | head -n 1)
LEDGER_API_ROOT=$(find "$ROOT_PATH" -type d -path "*/community/ledger-api*/src/main/protobuf" | head -n 1)

if [ -z "$COMMUNITY_BASE_ROOT" ] || [ -z "$LEDGER_API_ROOT" ]; then
    echo "Error: Could not find proto roots in $ROOT_PATH"
    echo "Community Base Root: ${COMMUNITY_BASE_ROOT:-"NOT FOUND"}"
    echo "Ledger API Root: ${LEDGER_API_ROOT:-"NOT FOUND"}"
    exit 1
fi

# Find value.proto
VAL_PROTO=$(find "$ROOT_PATH" -name "value.proto" | grep "com/daml/ledger/api/v2" | head -n 1)
if [ -z "$VAL_PROTO" ]; then
    echo "Error: Could not locate value.proto. Make sure the repository is initialized/built."
    exit 1
fi

echo "Found Community Base: $COMMUNITY_BASE_ROOT"
echo "Found Ledger API: $LEDGER_API_ROOT"
echo "Found value.proto: $VAL_PROTO"

# 2. Prepare Workspace
WORK_DIR="buf_image_build_work"
rm -rf "$WORK_DIR"
mkdir -p "$WORK_DIR/external/com/daml/ledger/api/v2"

# Copy value.proto to our workspace so it's part of a root
cp "$VAL_PROTO" "$WORK_DIR/external/com/daml/ledger/api/v2/value.proto"

# Download dependencies
download_if_not_exists() {
  local url=$1
  local dest="$WORK_DIR/external/$2"
  if [ ! -f "$dest" ]; then
    echo "Downloading $2..."
    mkdir -p "$(dirname "$dest")"
    curl -sSL "$url" -o "$dest"
  fi
}

download_if_not_exists "https://raw.githubusercontent.com/googleapis/googleapis/3597f7db2191c00b100400991ef96e52d62f5841/google/rpc/status.proto" "google/rpc/status.proto"
download_if_not_exists "https://raw.githubusercontent.com/protocolbuffers/protobuf/407aa2d9319f5db12964540810b446fecc22d419/src/google/protobuf/empty.proto" "google/protobuf/empty.proto"
download_if_not_exists "https://raw.githubusercontent.com/scalapb/ScalaPB/6291978a7ca8b48bd69cc98aa04cb28bc18a44a9/protobuf/scalapb/scalapb.proto" "scalapb/scalapb.proto"
download_if_not_exists "https://raw.githubusercontent.com/googleapis/googleapis/9415ba048aa587b1b2df2b96fc00aa009c831597/google/rpc/error_details.proto" "google/rpc/error_details.proto"

# 3. Create Buf Workspace
# buf build doesn't use -I. We use buf.work.yaml to define multiple roots.
# Since roots in buf.work.yaml must be relative and within the same tree, 
# we'll create symlinks in our work dir to the repo roots.

ln -s "$COMMUNITY_BASE_ROOT" "$WORK_DIR/community_base"
ln -s "$LEDGER_API_ROOT" "$WORK_DIR/ledger_api"

cat <<EOF > "$WORK_DIR/buf.work.yaml"
version: v1
directories:
  - external
  - community_base
  - ledger_api
EOF

# 4. Build Image
echo "Building Buf Image..."
# We run buf build from the workspace directory.
# We use --path to target the specific files we want in the image.
(
  cd "$WORK_DIR"
  buf build \
    --path ledger_api/com/daml/ledger/api/v2/interactive/interactive_submission_service.proto \
    --path ledger_api/com/daml/ledger/api/v2/interactive/transaction/v1/interactive_submission_data.proto \
    --path ledger_api/com/daml/ledger/api/v2/interactive/interactive_submission_common_data.proto \
    --path external/com/daml/ledger/api/v2/value.proto \
    --path ledger_api/com/daml/ledger/api/v2/transaction.proto \
    --path ledger_api/com/daml/ledger/api/v2/commands.proto \
    --path ledger_api/com/daml/ledger/api/v2/completion.proto \
    --path ledger_api/com/daml/ledger/api/v2/event.proto \
    --path ledger_api/com/daml/ledger/api/v2/trace_context.proto \
    --path ledger_api/com/daml/ledger/api/v2/transaction_filter.proto \
    --path ledger_api/com/daml/ledger/api/v2/package_reference.proto \
    --path ledger_api/com/daml/ledger/api/v2/offset_checkpoint.proto \
    --path ledger_api/com/daml/ledger/api/v2/crypto.proto \
    --path community_base/com/digitalasset/canton/protocol/v30/topology.proto \
    --path community_base/com/digitalasset/canton/protocol/v30/synchronizer_parameters.proto \
    --path community_base/com/digitalasset/canton/protocol/v30/traffic_control_parameters.proto \
    --path community_base/com/digitalasset/canton/protocol/v30/sequencing_parameters.proto \
    --path community_base/com/digitalasset/canton/protocol/v30/crypto.proto \
    --path community_base/com/digitalasset/canton/version/v1/untyped_versioned_message.proto \
    --path community_base/com/digitalasset/canton/crypto/v30/crypto.proto \
    --path external/google/rpc/status.proto \
    --path external/google/rpc/error_details.proto \
    --path external/scalapb/scalapb.proto \
    -o ../canton_buf_image.binpb
)

echo "Done! canton_buf_image.binpb is ready."
rm -rf "$WORK_DIR"
