package jsrepl

import (
	"context"
	"strings"
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

func TestSurfaceExposesAutocompleteAndHelp(t *testing.T) {
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

	ctx := context.Background()

	result, err := surface.evaluator.CompleteInput(ctx, bobarepl.CompletionRequest{
		Input:      "bbs.li",
		CursorByte: len("bbs.li"),
		Reason:     bobarepl.CompletionReasonShortcut,
		Shortcut:   "tab",
	})
	if err != nil {
		t.Fatalf("CompleteInput returned error: %v", err)
	}
	if !result.Show {
		t.Fatalf("expected completion overlay to be shown")
	}
	if !hasSuggestion(result, "listMessages") {
		t.Fatalf("expected listMessages suggestion, got %#v", result.Suggestions)
	}

	helpBar, err := surface.evaluator.GetHelpBar(ctx, bobarepl.HelpBarRequest{
		Input:      "console.log",
		CursorByte: len("console.log"),
		Reason:     bobarepl.HelpBarReasonManual,
	})
	if err != nil {
		t.Fatalf("GetHelpBar returned error: %v", err)
	}
	if !helpBar.Show {
		t.Fatalf("expected help bar to be shown")
	}
	if !strings.Contains(helpBar.Text, "console.log") {
		t.Fatalf("expected console.log help text, got %q", helpBar.Text)
	}

	helpDrawer, err := surface.evaluator.GetHelpDrawer(ctx, bobarepl.HelpDrawerRequest{
		Input:      "bbs.po",
		CursorByte: len("bbs.po"),
		RequestID:  7,
		Trigger:    bobarepl.HelpDrawerTriggerTyping,
	})
	if err != nil {
		t.Fatalf("GetHelpDrawer returned error: %v", err)
	}
	if !helpDrawer.Show {
		t.Fatalf("expected help drawer to be shown")
	}
	if !strings.Contains(helpDrawer.Markdown, "Base expression: `bbs`") || !strings.Contains(helpDrawer.Markdown, "Typed prefix: `po`") {
		t.Fatalf("expected bbs property context in help drawer, got %q", helpDrawer.Markdown)
	}
}

func hasSuggestion(result bobarepl.CompletionResult, label string) bool {
	for _, suggestion := range result.Suggestions {
		if suggestion.Value == label {
			return true
		}
	}
	return false
}
