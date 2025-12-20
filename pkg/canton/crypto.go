package canton

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

// ComputeHash implements the Canton-specific hashing logic:
// 1. Prefix with 4-byte BigEndian purpose
// 2. SHA256
// 3. Prefix with 0x12 (SHA256 multicodec) and 0x20 (length)
func ComputeHash(data []byte, purpose int) []byte {
	prefix := make([]byte, 4)
	binary.BigEndian.PutUint32(prefix, uint32(purpose))

	h := sha256.New()
	h.Write(prefix)
	h.Write(data)
	sum := h.Sum(nil)

	// Prefix with 0x12 0x20 (multihash header for SHA256)
	result := append([]byte{0x12, 0x20}, sum...)
	return result
}

type PublicKeyInfo struct {
	KeySpec   string
	Format    string
	PublicKey []byte
}

// InspectPublicKey parses a DER-encoded public key and returns its specification.
func InspectPublicKey(data []byte) (*PublicKeyInfo, error) {
	pub, err := x509.ParsePKIXPublicKey(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %v", err)
	}

	info := &PublicKeyInfo{
		Format:    "CRYPTO_KEY_FORMAT_DER_X509_SUBJECT_PUBLIC_KEY_INFO",
		PublicKey: data,
	}

	switch k := pub.(type) {
	case ed25519.PublicKey:
		info.KeySpec = "SIGNING_KEY_SPEC_EC_CURVE25519"
	case *ecdsa.PublicKey:
		switch k.Curve.Params().Name {
		case "P-256":
			info.KeySpec = "SIGNING_KEY_SPEC_EC_P256"
		case "P-384":
			info.KeySpec = "SIGNING_KEY_SPEC_EC_P384"
		default:
			return nil, fmt.Errorf("unsupported elliptic curve: %s", k.Curve.Params().Name)
		}
	default:
		return nil, fmt.Errorf("unsupported key type: %T", k)
	}

	return info, nil
}

type SignatureMetadata struct {
	Algorithm string
	Format    string
}

// GetSignatureMetadata maps a signature algorithm name to Canton Protobuf enum strings.
func GetSignatureMetadata(algo string) (*SignatureMetadata, error) {
	switch algo {
	case "ed25519":
		return &SignatureMetadata{
			Algorithm: "SIGNING_ALGORITHM_SPEC_ED25519",
			Format:    "SIGNATURE_FORMAT_CONCAT",
		}, nil
	case "ecdsa256":
		return &SignatureMetadata{
			Algorithm: "SIGNING_ALGORITHM_SPEC_EC_DSA_SHA_256",
			Format:    "SIGNATURE_FORMAT_DER",
		}, nil
	case "ecdsa384":
		return &SignatureMetadata{
			Algorithm: "SIGNING_ALGORITHM_SPEC_EC_DSA_SHA_384",
			Format:    "SIGNATURE_FORMAT_DER",
		}, nil
	default:
		return nil, fmt.Errorf("unsupported signature algorithm: %s", algo)
	}
}

// Fingerprint computes the Canton fingerprint for a public key.
// It auto-detects Ed25519 keys to extract the raw 32-byte key material.
func Fingerprint(data []byte) string {
	var keyData []byte

	// Auto-detect key type
	pub, err := x509.ParsePKIXPublicKey(data)
	if err == nil {
		if edPub, ok := pub.(ed25519.PublicKey); ok {
			// For Ed25519, use the raw 32 bytes
			keyData = edPub
		} else {
			// For other parsed keys (RSA/ECDSA), use the full raw input as per script
			keyData = data
		}
	} else {
		// If parsing fails, fall back to the raw input
		keyData = data
	}

	// 12 is the hash purpose for public key fingerprints
	hash := ComputeHash(keyData, 12)
	return hex.EncodeToString(hash)
}
