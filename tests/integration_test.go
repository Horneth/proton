package tests

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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

	// 2. Test template
	out, err := runCLI(binPath, repoRoot, "proto", "template", "TopologyTransaction")
	if err != nil {
		t.Fatalf("proto template failed: %v\nOutput: %s", err, out)
	}
	if !strings.Contains(out, "operation") {
		t.Errorf("template output missing 'operation' field: %s", out)
	}

	// 3. Test generate --set
	out, err = runCLI(binPath, repoRoot, "proto", "generate", "TopologyTransaction",
		"--set", "serial=99",
		"--set", "operation=TOPOLOGY_CHANGE_OP_REMOVE",
		"--versioned", "30",
		"--base64")
	if err != nil {
		t.Fatalf("proto generate failed: %v\nOutput: %s", err, out)
	}
	b64Data := strings.TrimSpace(out)

	// 4. Test decode
	out, err = runCLIWithStdin(binPath, repoRoot, b64Data, "proto", "decode", "TopologyTransaction", "-", "--versioned", "--base64")
	if err != nil {
		t.Fatalf("proto decode failed: %v\nOutput: %s", err, out)
	}
	if !strings.Contains(out, `"serial": 99`) || !strings.Contains(out, "TOPOLOGY_CHANGE_OP_REMOVE") {
		t.Errorf("decoded output incorrect: %s", out)
	}

	// 5. Test specialized prepare
	keyPath := filepath.Join(repoRoot, "ecdsa_pub.der")

	out, err = runCLI(binPath, repoRoot, "canton", "topology", "prepare", "delegation",
		"--root", "--root-key", "@"+keyPath, "--output", filepath.Join(t.TempDir(), "test-prep"))
	if err != nil {
		t.Fatalf("canton prepare failed: %v\nOutput: %s", err, out)
	}
	if !strings.Contains(out, "Transaction written to") {
		t.Errorf("prepare output missing success message: %s", out)
	}
}

func runCLI(bin, dir string, args ...string) (string, error) {
	cmd := exec.Command(bin, args...)
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

func runCLIWithStdin(bin, dir, stdin string, args ...string) (string, error) {
	cmd := exec.Command(bin, args...)
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
