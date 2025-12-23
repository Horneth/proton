package tests

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const testConfigJSON = `{
    "aliases": {
        "SignedTopologyTransaction": "com.digitalasset.canton.protocol.v30.SignedTopologyTransaction",
        "TopologyTransaction": "com.digitalasset.canton.protocol.v30.TopologyTransaction",
        "SigningPublicKey": "com.digitalasset.canton.crypto.v30.SigningPublicKey",
	"PreparedTransaction": "com.daml.ledger.api.v2.interactive.PreparedTransaction"
    },
    "mappings": [
        {
            "type": "com.digitalasset.canton.protocol.v30.SignedTopologyTransaction",
            "field": "transaction",
            "target_type": "com.digitalasset.canton.protocol.v30.TopologyTransaction",
            "versioned": true,
            "default_version": 30
        }
    ]
}
`

func setupTestConfig(t *testing.T) string {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(configPath, []byte(testConfigJSON), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
	return configPath
}

func TestCLI_BasicFlow(t *testing.T) {
	imagePath := os.Getenv("PROTO_IMAGE")
	if imagePath == "" {
		t.Skip("PROTO_IMAGE not set")
	}

	// 1. Compile the binary
	repoRoot, _ := filepath.Abs("../")
	binPath := filepath.Join(repoRoot, "bin", "proton")

	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/proton")
	buildCmd.Dir = repoRoot
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build binary: %v", err)
	}

	// 1.5. Prepare config
	configPath := setupTestConfig(t)

	// 2. Test template
	out, err := runCLI(configPath, binPath, repoRoot, "proto", "template", "TopologyTransaction")
	if err != nil {
		t.Fatalf("proto template failed: %v\nOutput: %s", err, out)
	}
	if !strings.Contains(out, "operation") {
		t.Errorf("template output missing 'operation' field: %s", out)
	}

	// 3. Test generate --set
	out, err = runCLI(configPath, binPath, repoRoot, "proto", "generate", "TopologyTransaction",
		"--set", "serial=99",
		"--set", "operation=TOPOLOGY_CHANGE_OP_REMOVE",
		"--versioned", "30",
		"--base64")
	if err != nil {
		t.Fatalf("proto generate failed: %v\nOutput: %s", err, out)
	}
	b64Data := strings.TrimSpace(out)

	// 4. Test decode
	out, err = runCLIWithStdin(configPath, binPath, repoRoot, b64Data, "proto", "decode", "TopologyTransaction", "-", "--versioned", "--base64")
	if err != nil {
		t.Fatalf("proto decode failed: %v\nOutput: %s", err, out)
	}
	if !strings.Contains(out, `"serial": 99`) || !strings.Contains(out, "TOPOLOGY_CHANGE_OP_REMOVE") {
		t.Errorf("decoded output incorrect: %s", out)
	}

	// 5. Test specialized prepare
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "test_pub.der")

	// Generate a dummy ECDSA key for the test
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pubBytes, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	os.WriteFile(keyPath, pubBytes, 0644)

	out, err = runCLI(configPath, binPath, repoRoot, "canton", "topology", "prepare", "delegation",
		"--root", "--root-key", "@"+keyPath, "--output", filepath.Join(tempDir, "test-prep"))
	if err != nil {
		t.Fatalf("canton prepare failed: %v\nOutput: %s", err, out)
	}
	if !strings.Contains(out, "Transaction written to") {
		t.Errorf("prepare output missing success message: %s", out)
	}
}

func TestCLI_VerifySignature(t *testing.T) {
	imagePath := os.Getenv("PROTO_IMAGE")
	if imagePath == "" {
		t.Skip("PROTO_IMAGE not set")
	}

	repoRoot, _ := filepath.Abs("../")
	binPath := filepath.Join(repoRoot, "bin", "proton")

	// 0. Compile the binary to ensure we have latest changes
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/proton")
	buildCmd.Dir = repoRoot
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build binary: %v", err)
	}

	configPath := setupTestConfig(t)
	tmpDir := t.TempDir()

	// 1. Generate an Ed25519 key pair
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i)
	}
	privKey := ed25519.NewKeyFromSeed(seed)
	pubKey := privKey.Public().(ed25519.PublicKey)

	pubKeyDer, _ := x509.MarshalPKIXPublicKey(pubKey)

	privPath := filepath.Join(tmpDir, "test.priv")
	pubPath := filepath.Join(tmpDir, "test.pub")
	os.WriteFile(privPath, privKey, 0644)
	os.WriteFile(pubPath, pubKeyDer, 0644)

	// 2. Prepare Transaction
	prepPrefix := filepath.Join(tmpDir, "tx")
	_, err := runCLI(configPath, binPath, repoRoot, "canton", "topology", "prepare", "delegation",
		"--root", "--root-key", "@"+pubPath, "--output", prepPrefix)
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	// 3. Sign the Hash
	hashPath := prepPrefix + ".hash"
	sig, err := runCLI(configPath, binPath, repoRoot, "crypto", "sign", "@"+privPath, "@"+hashPath, "--algo", "ed25519")
	if err != nil {
		t.Fatalf("sign failed: %v\nOutput: %s", err, sig)
	}
	sig = strings.TrimSpace(sig)

	// 4. Assemble
	fp, _ := runCLI(configPath, binPath, repoRoot, "crypto", "fingerprint", "@"+pubPath)
	fp = strings.TrimSpace(fp)

	certPath := filepath.Join(tmpDir, "tx.cert")
	assembleOut, err := runCLI(configPath, binPath, repoRoot, "canton", "topology", "assemble",
		"--prepared-transaction", "@"+prepPrefix+".prep",
		"--signature", sig,
		"--signature-algorithm", "ed25519",
		"--signed-by", fp,
		"--output", certPath)
	t.Logf("Assemble Output:\n%s", assembleOut)
	if err != nil {
		t.Fatalf("assemble failed: %v", err)
	}

	// 5. Verify
	out, err := runCLI(configPath, binPath, repoRoot, "canton", "topology", "verify",
		"--input", "@"+certPath,
		"--public-key", "@"+pubPath)
	if err != nil {
		t.Fatalf("verify failed: %v\nOutput: %s", err, out)
	}
	if !strings.Contains(out, "SUCCESS: Signature is valid") {
		t.Errorf("verification output missing success message: %s", out)
	}

	// 6. Test failure with wrong key
	wrongSeed := make([]byte, 32)
	wrongSeed[0] = 0xFF
	wrongPriv := ed25519.NewKeyFromSeed(wrongSeed)
	wrongPub := wrongPriv.Public().(ed25519.PublicKey)
	wrongPubDer, _ := x509.MarshalPKIXPublicKey(wrongPub)
	wrongPubPath := filepath.Join(tmpDir, "wrong.pub")
	os.WriteFile(wrongPubPath, wrongPubDer, 0644)

	_, err = runCLI(configPath, binPath, repoRoot, "canton", "topology", "verify",
		"--input", "@"+certPath,
		"--public-key", "@"+wrongPubPath)

	// Should fail (exit 1)
	if err == nil {
		t.Errorf("expected verify to fail with wrong key, but it succeeded")
	}
}

func TestCLI_VerifySignature_ECDSA(t *testing.T) {
	imagePath := os.Getenv("PROTO_IMAGE")
	if imagePath == "" {
		t.Skip("PROTO_IMAGE not set")
	}

	repoRoot, _ := filepath.Abs("../")
	binPath := filepath.Join(repoRoot, "bin", "proton")

	// Build binary
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/proton")
	buildCmd.Dir = repoRoot
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build binary: %v", err)
	}

	configPath := setupTestConfig(t)
	tmpDir := t.TempDir()

	// 1. Generate ECDSA P-256 key pair
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	pubKeyDer, _ := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	privKeyDer, _ := x509.MarshalECPrivateKey(privKey) // Using EC private key format for simplicity

	privPath := filepath.Join(tmpDir, "test.priv")
	pubPath := filepath.Join(tmpDir, "test.pub")
	os.WriteFile(privPath, privKeyDer, 0644)
	os.WriteFile(pubPath, pubKeyDer, 0644)

	// 2. Prepare Transaction
	prepPrefix := filepath.Join(tmpDir, "tx")
	_, err = runCLI(configPath, binPath, repoRoot, "canton", "topology", "prepare", "delegation",
		"--root", "--root-key", "@"+pubPath, "--output", prepPrefix)
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	// 3. Sign the Hash (Hash path contains 34-byte multihash)
	hashPath := prepPrefix + ".hash"
	sig, err := runCLI(configPath, binPath, repoRoot, "crypto", "sign", "@"+privPath, "@"+hashPath, "--algo", "ecdsa256")
	if err != nil {
		t.Fatalf("sign failed: %v\nOutput: %s", err, sig)
	}
	sig = strings.TrimSpace(sig)

	// 4. Assemble
	fp, _ := runCLI(configPath, binPath, repoRoot, "crypto", "fingerprint", "@"+pubPath)
	fp = strings.TrimSpace(fp)

	certPath := filepath.Join(tmpDir, "tx.cert")
	assembleOut, err := runCLI(configPath, binPath, repoRoot, "canton", "topology", "assemble",
		"--prepared-transaction", "@"+prepPrefix+".prep",
		"--signature", sig,
		"--signature-algorithm", "ecdsa256",
		"--signed-by", fp,
		"--output", certPath)
	t.Logf("Assemble Output:\n%s", assembleOut)
	if err != nil {
		t.Fatalf("assemble failed: %v", err)
	}

	// 5. Verify
	out, err := runCLI(configPath, binPath, repoRoot, "canton", "topology", "verify",
		"--input", "@"+certPath,
		"--public-key", "@"+pubPath)
	if err != nil {
		t.Fatalf("verify failed: %v\nOutput: %s", err, out)
	}
	if !strings.Contains(out, "SUCCESS: Signature is valid") {
		t.Errorf("verification output missing success message: %s", out)
	}
}

func runCLI(configPath, bin, dir string, args ...string) (string, error) {
	fullArgs := append([]string{"--config", configPath}, args...)
	cmd := exec.Command(bin, fullArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PROTO_IMAGE="+os.Getenv("PROTO_IMAGE"))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return stderr.String(), err
	}
	return stdout.String(), nil
}

func runCLIWithStdin(configPath, bin, dir, stdin string, args ...string) (string, error) {
	fullArgs := append([]string{"--config", configPath}, args...)
	cmd := exec.Command(bin, fullArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PROTO_IMAGE="+os.Getenv("PROTO_IMAGE"))
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return stderr.String(), err
	}
	return stdout.String(), nil
}
