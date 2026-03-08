package jsrepl

import (
	"context"
	"testing"
	"time"

	bobarepl "github.com/go-go-golems/bobatea/pkg/repl"
	"github.com/manuel/wesen/qemu-go-init/internal/bbsstore"
)

type stubStore struct {
	messages []bbsstore.Message
	nextID   int64
}

func (s *stubStore) ListMessages(context.Context) ([]bbsstore.Message, error) {
	out := make([]bbsstore.Message, len(s.messages))
	copy(out, s.messages)
	return out, nil
}

func (s *stubStore) CreateMessage(_ context.Context, params bbsstore.CreateMessageParams) (bbsstore.Message, error) {
	s.nextID++
	message := bbsstore.Message{
		ID:        s.nextID,
		Author:    params.Author,
		Subject:   params.Subject,
		Body:      params.Body,
		CreatedAt: time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC),
	}
	s.messages = append([]bbsstore.Message{message}, s.messages...)
	return message, nil
}

func TestSurfaceInstallsBBSAPI(t *testing.T) {
	store := &stubStore{
		nextID: 1,
		messages: []bbsstore.Message{{
			ID:        1,
			Author:    "system",
			Subject:   "welcome",
			Body:      "hello",
			CreatedAt: time.Date(2026, time.March, 8, 11, 0, 0, 0, time.UTC),
		}},
	}

	surface, err := New(store, Options{
		StateRoot:     "/var/lib/go-init/shared/bbs",
		DefaultAuthor: "tester",
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer func() {
		_ = surface.Close()
	}()

	var outputs []string
	err = surface.evaluator.EvaluateStream(context.Background(), "bbs.listMessages()[0].subject", func(event bobarepl.Event) {
		if event.Kind == bobarepl.EventResultMarkdown {
			if markdown, ok := event.Props["markdown"].(string); ok {
				outputs = append(outputs, markdown)
			}
		}
	})
	if err != nil {
		t.Fatalf("EvaluateStream returned error: %v", err)
	}
	if len(outputs) != 1 || outputs[0] != "welcome" {
		t.Fatalf("expected welcome output, got %v", outputs)
	}

	outputs = outputs[:0]
	err = surface.evaluator.EvaluateStream(context.Background(), "bbs.post('hello from js', 'body text').author", func(event bobarepl.Event) {
		if event.Kind == bobarepl.EventResultMarkdown {
			if markdown, ok := event.Props["markdown"].(string); ok {
				outputs = append(outputs, markdown)
			}
		}
	})
	if err != nil {
		t.Fatalf("EvaluateStream returned error for post: %v", err)
	}
	if len(outputs) != 1 || outputs[0] != "tester" {
		t.Fatalf("expected tester output, got %v", outputs)
	}

	if len(store.messages) != 2 {
		t.Fatalf("expected message to be created, got %d messages", len(store.messages))
	}
}
