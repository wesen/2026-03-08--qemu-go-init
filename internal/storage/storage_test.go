package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFromEnvDefaults(t *testing.T) {
	t.Setenv("GO_INIT_ENABLE_STORAGE", "")
	t.Setenv("GO_INIT_STORAGE_DEVICE", "")
	t.Setenv("GO_INIT_STORAGE_MOUNT_POINT", "")
	t.Setenv("GO_INIT_STORAGE_FILESYSTEM", "")
	t.Setenv("GO_INIT_STORAGE_WAIT", "")

	cfg := LoadConfigFromEnv()
	if !cfg.Enabled {
		t.Fatalf("expected storage enabled by default")
	}
	if cfg.Device != defaultDevice {
		t.Fatalf("unexpected device: %q", cfg.Device)
	}
	if cfg.MountPoint != defaultMountPoint {
		t.Fatalf("unexpected mount point: %q", cfg.MountPoint)
	}
	if cfg.Filesystem != defaultFilesystem {
		t.Fatalf("unexpected filesystem: %q", cfg.Filesystem)
	}
	if cfg.WaitFor != defaultWait {
		t.Fatalf("unexpected wait duration: %s", cfg.WaitFor)
	}
}

func TestEnsureStateDirectories(t *testing.T) {
	root := t.TempDir()
	directories, err := ensureStateDirectories(root)
	if err != nil {
		t.Fatalf("ensure directories: %v", err)
	}
	if len(directories) != len(stateDirectories) {
		t.Fatalf("unexpected directory count: %d", len(directories))
	}

	for _, spec := range stateDirectories {
		path := filepath.Join(root, spec.path)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s is not a directory", path)
		}
	}
}
