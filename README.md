# Proton: Universal Protobuf & Canton/Daml Toolkit

Proton is a versatile Go-based CLI tool designed for working with Protobuf messages, specifically tailored for Canton and Daml ecosystems. It leverages `buf` libraries to provide generic schema-driven operations alongside specialized logic for Canton topology transactions and Daml transaction hashing.

## Features

- **Generic Protobuf Operations**:
    - `proto template`: Generate JSON templates for any message in a Buf image.
    - `proto generate`: Convert JSON data to Protobuf binary (supports recursive message nesting).
    - `proto decode`: Decode Protobuf binary back to JSON (supports recursive unwrapping).
- **Canton Topology Management**:
    - `canton topology prepare`: Build and serialize complex topology transactions (Namespace Delegations, etc.).
    - `canton topology assemble`: Sign and bundle prepared transactions into `SignedTopologyTransaction` certificates.
    - `canton topology verify`: Verify SHA256-based cryptographic signatures across topology transactions.
- **Daml Interactive Submission**:
    - `daml hash`: Compute deterministic **Daml V2 SHA256 secure hashes** for `PreparedTransaction` messages.
    - `daml decode`: Specialized high-performance decoding for Daml transactions without needing external schema files.
- **Cryptographic Utilities**:
    - `crypto fingerprint`: Compute Canton-compatible fingerprints for public keys.
    - `crypto sign`: Sign arbitrary data using standard algorithms (Ed25519, ECDSA).

## Getting Started

### Prerequisites

- [Go](https://golang.org/doc/install) (1.23+)
- [Buf CLI](https://buf.build/docs/installation)

### Installation & Build

1. **Clone the repository**:
   ```bash
   git clone <repository-url>
   cd buf-lib-poc
   ```

2. **Generate Protobuf Code**:
   Proton uses Go code generated from a Buf image for specialized Daml logic. The Buf image is included in the repository, but you need to generate the Go interfaces:
   ```bash
   make generate
   ```

3. **Build the CLI**:
   ```bash
   make build
   ```
   The binary will be available at `bin/proton`.

### Build Automation

The provided `Makefile` handles common tasks:
- `make generate`: Runs the `buf generate` logic for Daml protos.
- `make build`: Compiles the binary.
- `make test`: Runs unit and integration tests.
- `make clean`: Removes binaries and generated code.

## Usage Guide

### Generic Protobuf Handling
Proton can work with any Protobuf definition via a Buf image:
```bash
# Set your Buf image via environment variable
export PROTO_IMAGE=path/to/image.binpb

# Generate a template
proton proto template MyMessage

# Decode a binary file
proton proto decode MyMessage @data.bin
```

### Daml Transaction Hashing
To compute the secure hash of a `PreparedTransaction`:
```bash
proton daml hash @transaction.bin
```

### Canton Topology Transactions
Example of preparing and assembling a namespace delegation:
```bash
# 1. Prepare
proton canton topology prepare delegation --root-key @root.pub --target-key @new.pub --output my_delegation

# 2. Assemble with signature
proton canton topology assemble --prepared-transaction @my_delegation.prep --signature @sig.bin --signature-algorithm ed25519 --signed-by <fingerprint> --output cert.bin
```

## Development

### Buf Image Generation
If you need to rebuild the consolidated `canton_buf_image.binpb` from the Canton source, use the helper script:
1. Copy `scripts/build_canton_buf_image.sh` to your Canton repository root.
2. Run it to generate a new `canton_buf_image.binpb` containing Daml ledger, Canton topology, crypto, and versioning protos.
3. Copy the resulting image back to the Proton repository root.

### Testing
```bash
make test
```
This runs the full suite of unit tests for core packages (`pkg/patch`, `pkg/engine`, `pkg/daml/hash`) and integration tests for the CLI.

## Standardized Input UX
All file inputs in Proton strictly follow the **`@` prefix convention**:
- Use `@path/to/file` to read content from a file.
- Omit the `@` to treat the input as a literal string or base64.
- Use `-` to read from `stdin`.

---
*Proton - Powering Protobuf Workflows for Canton and Daml.*
