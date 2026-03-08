package sharedstate

import "testing"

func TestLoadConfigFromEnvDefaults(t *testing.T) {
	t.Setenv("GO_INIT_ENABLE_SHARED_STATE", "")
	t.Setenv("GO_INIT_SHARED_STATE_REQUIRED", "")
	t.Setenv("GO_INIT_SHARED_STATE_TAG", "")
	t.Setenv("GO_INIT_SHARED_STATE_MOUNT_POINT", "")
	t.Setenv("GO_INIT_SHARED_STATE_FILESYSTEM", "")
	t.Setenv("GO_INIT_SHARED_STATE_MOUNT_OPTIONS", "")

	cfg := LoadConfigFromEnv()
	if !cfg.Enabled {
		t.Fatalf("expected shared state to default enabled")
	}
	if cfg.Required {
		t.Fatalf("expected shared state to default optional")
	}
	if cfg.MountTag != defaultMountTag {
		t.Fatalf("unexpected mount tag: %q", cfg.MountTag)
	}
	if cfg.MountPoint != defaultMountPoint {
		t.Fatalf("unexpected mount point: %q", cfg.MountPoint)
	}
}

func TestBBSRootPrefersMountedShare(t *testing.T) {
	root := BBSRoot(Result{Mounted: true, MountPoint: "/var/lib/go-init/shared"}, "/var/lib/go-init/app")
	if root != "/var/lib/go-init/shared/bbs" {
		t.Fatalf("unexpected root: %q", root)
	}
}

func TestBBSRootFallsBackToLocalStorage(t *testing.T) {
	root := BBSRoot(Result{Mounted: false}, "/var/lib/go-init/app")
	if root != "/var/lib/go-init/app/bbs" {
		t.Fatalf("unexpected root: %q", root)
	}
}
