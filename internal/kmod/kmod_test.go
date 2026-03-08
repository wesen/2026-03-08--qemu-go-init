package kmod

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestEnsureModuleLoadsSuccessfully(t *testing.T) {
	dir := t.TempDir()
	modulePath := filepath.Join(dir, "virtio_rng.ko")
	if err := os.WriteFile(modulePath, []byte("module"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	called := false
	result := ensureModule(modulePath, nil, os.Stat, func(path string) error {
		called = true
		if path != modulePath {
			t.Fatalf("got path %q, want %q", path, modulePath)
		}
		return nil
	})

	if !called {
		t.Fatal("expected loader to be called")
	}
	if !result.Attempted || !result.Loaded || result.AlreadyLoaded {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Error != "" || result.Step != "loaded" {
		t.Fatalf("unexpected status: %#v", result)
	}
}

func TestEnsureModuleTreatsEEXISTAsLoaded(t *testing.T) {
	dir := t.TempDir()
	modulePath := filepath.Join(dir, "virtio_rng.ko")
	if err := os.WriteFile(modulePath, []byte("module"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result := ensureModule(modulePath, nil, os.Stat, func(string) error {
		return unix.EEXIST
	})

	if !result.Attempted || !result.Loaded || !result.AlreadyLoaded {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Error != "" || result.Step != "already-loaded" {
		t.Fatalf("unexpected status: %#v", result)
	}
}

func TestEnsureModuleReturnsMissingFile(t *testing.T) {
	modulePath := filepath.Join(t.TempDir(), "missing.ko")

	result := ensureModule(modulePath, nil, os.Stat, func(string) error {
		t.Fatal("loader should not be called for a missing file")
		return nil
	})

	if result.Attempted || result.Loaded || result.AlreadyLoaded {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Step != "missing" {
		t.Fatalf("got step %q, want missing", result.Step)
	}
	if !strings.Contains(result.Error, "no such file") {
		t.Fatalf("expected missing-file error, got %#v", result)
	}
}
