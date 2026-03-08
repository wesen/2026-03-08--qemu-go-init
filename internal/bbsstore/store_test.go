package bbsstore

import (
	"context"
	"testing"
)

func TestOpenSeedsWelcomeMessage(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	messages, err := store.ListMessages(context.Background())
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(messages) == 0 {
		t.Fatalf("expected seeded welcome message")
	}
}

func TestCreateMessage(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	message, err := store.CreateMessage(context.Background(), CreateMessageParams{
		Author:  "tester",
		Subject: "hello",
		Body:    "world",
	})
	if err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}
	if message.ID == 0 {
		t.Fatalf("expected inserted id")
	}

	messages, err := store.ListMessages(context.Background())
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if messages[0].Subject != "hello" {
		t.Fatalf("expected newest message first, got %q", messages[0].Subject)
	}
}
