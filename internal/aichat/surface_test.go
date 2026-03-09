package aichat

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/manuel/wesen/qemu-go-init/internal/bbsstore"
)

type stubStore struct {
	messages []bbsstore.Message
}

func (s stubStore) ListMessages(_ context.Context) ([]bbsstore.Message, error) {
	return s.messages, nil
}

func TestResolveRuntimeUsesSharedStateConfig(t *testing.T) {
	t.Setenv("GO_INIT_PINOCCHIO_CONFIG_HOME", "")
	t.Setenv("PINOCCHIO_PROFILE_REGISTRIES", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	root := t.TempDir()
	sharedRoot := filepath.Join(root, "shared-state")
	if err := os.MkdirAll(filepath.Join(sharedRoot, "bbs"), 0o755); err != nil {
		t.Fatalf("mkdir bbs root: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sharedRoot, "pinocchio"), 0o755); err != nil {
		t.Fatalf("mkdir pinocchio root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sharedRoot, "pinocchio", "config.yaml"), []byte("openai-chat:\n  openai-api-key: test-key\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sharedRoot, "pinocchio", "profiles.yaml"), []byte("slug: default\nprofiles:\n  gpt-5-nano:\n    slug: gpt-5-nano\n    runtime:\n      step_settings_patch:\n        ai-chat:\n          ai-api-type: openai-responses\n          ai-engine: gpt-5-nano\n"), 0o644); err != nil {
		t.Fatalf("write profiles: %v", err)
	}

	configHome, registries, profile, err := resolveRuntime(Options{
		StateRoot: filepath.Join(sharedRoot, "bbs"),
	})
	if err != nil {
		t.Fatalf("resolve runtime: %v", err)
	}
	if got, want := configHome, sharedRoot; got != want {
		t.Fatalf("config home = %q, want %q", got, want)
	}
	if got, want := registries, filepath.Join(sharedRoot, "pinocchio", "profiles.yaml"); got != want {
		t.Fatalf("registries = %q, want %q", got, want)
	}
	if got, want := profile, defaultProfileSlug; got != want {
		t.Fatalf("profile = %q, want %q", got, want)
	}
}

func TestNewBuildsSurfaceFromSharedStateConfig(t *testing.T) {
	t.Setenv("GO_INIT_PINOCCHIO_CONFIG_HOME", "")
	t.Setenv("PINOCCHIO_PROFILE_REGISTRIES", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	root := t.TempDir()
	sharedRoot := filepath.Join(root, "shared-state")
	if err := os.MkdirAll(filepath.Join(sharedRoot, "bbs"), 0o755); err != nil {
		t.Fatalf("mkdir bbs root: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sharedRoot, "pinocchio"), 0o755); err != nil {
		t.Fatalf("mkdir pinocchio root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sharedRoot, "pinocchio", "config.yaml"), []byte("openai-chat:\n  openai-api-key: test-key\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sharedRoot, "pinocchio", "profiles.yaml"), []byte("slug: default\nprofiles:\n  gpt-5-nano:\n    slug: gpt-5-nano\n    runtime:\n      step_settings_patch:\n        ai-chat:\n          ai-api-type: openai-responses\n          ai-engine: gpt-5-nano\n"), 0o644); err != nil {
		t.Fatalf("write profiles: %v", err)
	}

	surface, err := New(stubStore{
		messages: []bbsstore.Message{
			{
				ID:        1,
				Author:    "manuel",
				Subject:   "hello",
				Body:      "world",
				CreatedAt: time.Unix(1700000000, 0).UTC(),
			},
		},
	}, Options{
		StateRoot: filepath.Join(sharedRoot, "bbs"),
	})
	if err != nil {
		t.Fatalf("new surface: %v", err)
	}
	defer func() { _ = surface.Close() }()

	if surface.backend == nil {
		t.Fatalf("backend is nil")
	}
	if surface.router == nil {
		t.Fatalf("router is nil")
	}
	if surface.seed == nil {
		t.Fatalf("seed turn is nil")
	}
	if got := surface.View(); got == "" {
		t.Fatalf("view should not be empty")
	}
}
