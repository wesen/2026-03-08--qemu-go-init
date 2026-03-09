package logstore

import (
	"context"
	"path/filepath"
	"testing"
)

func TestWriterPersistsStructuredAndPlainLines(t *testing.T) {
	t.Parallel()

	store, err := Open(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	writer := store.Writer("guest")
	if _, err := writer.Write([]byte("{\"level\":\"warn\",\"component\":\"chat\",\"message\":\"hello\"}\nplain text line\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	snapshot, err := store.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snapshot.Rows != 2 {
		t.Fatalf("rows = %d, want 2", snapshot.Rows)
	}

	row := store.db.QueryRow(`SELECT source, level, component, message FROM logs ORDER BY id ASC LIMIT 1`)
	var source, level, component, message string
	if err := row.Scan(&source, &level, &component, &message); err != nil {
		t.Fatalf("scan first row: %v", err)
	}
	if source != "guest" || level != "warn" || component != "chat" || message != "hello" {
		t.Fatalf("unexpected first row: source=%q level=%q component=%q message=%q", source, level, component, message)
	}

	row = store.db.QueryRow(`SELECT message FROM logs ORDER BY id DESC LIMIT 1`)
	if err := row.Scan(&message); err != nil {
		t.Fatalf("scan second row: %v", err)
	}
	if message != "plain text line" {
		t.Fatalf("plain text message = %q, want %q", message, "plain text line")
	}
}

func TestSnapshotHandlesClosedStoreFile(t *testing.T) {
	t.Parallel()

	store, err := Open(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	path := store.path
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	store.db = nil

	snapshot, err := store.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snapshot.Path != path {
		t.Fatalf("path = %q, want %q", snapshot.Path, path)
	}
}
