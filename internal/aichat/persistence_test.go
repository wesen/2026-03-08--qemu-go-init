package aichat

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenPersistenceCreatesSQLiteDatabases(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "shared", "bbs")
	persist, err := openPersistence(root, "conv-test")
	if err != nil {
		t.Fatalf("openPersistence: %v", err)
	}
	t.Cleanup(func() {
		if err := persist.Close(); err != nil {
			t.Fatalf("close persistence: %v", err)
		}
	})

	if got, want := persist.root, filepath.Join(filepath.Dir(root), "chat"); got != want {
		t.Fatalf("root = %q, want %q", got, want)
	}

	for _, path := range []string{persist.turnsDBPath, persist.timelineDBPath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if info.IsDir() {
			t.Fatalf("%s should be a file", path)
		}
	}
}
