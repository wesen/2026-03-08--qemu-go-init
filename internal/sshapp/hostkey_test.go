package sshapp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureHostKeyCreatesValidKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ssh_host_ed25519")
	if err := ensureHostKey(path); err != nil {
		t.Fatalf("ensure host key: %v", err)
	}

	valid, err := hostKeyValid(path)
	if err != nil {
		t.Fatalf("validate host key: %v", err)
	}
	if !valid {
		t.Fatalf("expected generated host key to be valid")
	}

	pubInfo, err := os.Stat(path + ".pub")
	if err != nil {
		t.Fatalf("stat public key: %v", err)
	}
	if pubInfo.Size() == 0 {
		t.Fatalf("expected non-empty public key")
	}
}

func TestEnsureHostKeyRepairsZeroByteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ssh_host_ed25519")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("write zero-byte private key: %v", err)
	}
	if err := os.WriteFile(path+".pub", nil, 0o600); err != nil {
		t.Fatalf("write zero-byte public key: %v", err)
	}

	if err := ensureHostKey(path); err != nil {
		t.Fatalf("repair host key: %v", err)
	}

	valid, err := hostKeyValid(path)
	if err != nil {
		t.Fatalf("validate repaired host key: %v", err)
	}
	if !valid {
		t.Fatalf("expected repaired host key to be valid")
	}
}
