package canton

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
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
