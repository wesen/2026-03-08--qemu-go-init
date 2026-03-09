package jsrepl

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/bobatea/pkg/eventbus"
	bobarepl "github.com/go-go-golems/bobatea/pkg/repl"
	"github.com/go-go-golems/bobatea/pkg/timeline"
	jsadapter "github.com/go-go-golems/go-go-goja/pkg/repl/adapters/bobatea"
	jseval "github.com/go-go-golems/go-go-goja/pkg/repl/evaluators/javascript"
	"github.com/manuel/wesen/qemu-go-init/internal/bbsstore"
)

type Store interface {
	ListMessages(ctx context.Context) ([]bbsstore.Message, error)
	CreateMessage(ctx context.Context, params bbsstore.CreateMessageParams) (bbsstore.Message, error)
}

type Options struct {
	StateRoot     string
	DefaultAuthor string
}

type messageRecord struct {
	ID        int64  `json:"id"`
	Author    string `json:"author"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
}

func newEvaluator() (*jsadapter.JavaScriptEvaluator, error) {
	cfg := jseval.DefaultConfig()
	cfg.EnableConsoleLog = false
	return jsadapter.NewJavaScriptEvaluator(cfg)
}

type Surface struct {
	evaluator *jsadapter.JavaScriptEvaluator
	bus       *eventbus.Bus
	model     *bobarepl.Model

	cancel     context.CancelFunc
	attachOnce sync.Once
	closeOnce  sync.Once
}

func New(store Store, options Options) (*Surface, error) {
	coreEvaluator, err := newEvaluator()
	if err != nil {
		return nil, err
	}

	bus, err := eventbus.NewInMemoryBus()
	if err != nil {
		_ = coreEvaluator.Close()
		return nil, fmt.Errorf("create bobatea event bus: %w", err)
	}
	bobarepl.RegisterReplToTimelineTransformer(bus)

	cfg := bobarepl.DefaultConfig()
	cfg.Title = "qemu-go-init JavaScript REPL"
	cfg.Placeholder = "Try: 2 + 2, bbs.listMessages()[0], bbs.post(\"subject\", \"body\") | Tab completion | Alt+H docs | Ctrl+P palette"
	cfg.EnableExternalEditor = false
	cfg.Autocomplete.TriggerKeys = []string{"tab"}
	cfg.Autocomplete.AcceptKeys = []string{"enter", "tab"}
	cfg.Autocomplete.FocusToggleKey = "ctrl+t"
	cfg.HelpDrawer.PrefetchWhenHidden = true

	if err := installGlobals(coreEvaluator, store, options); err != nil {
		_ = coreEvaluator.Close()
		return nil, err
	}

	return &Surface{
		evaluator: coreEvaluator,
		bus:       bus,
		model:     bobarepl.NewModel(coreEvaluator, cfg, bus.Publisher),
	}, nil
}

func (s *Surface) Model() *bobarepl.Model {
	return s.model
}

func (s *Surface) AttachProgram(ctx context.Context, program *tea.Program) {
	if s == nil || s.bus == nil || program == nil {
		return
	}
	s.attachOnce.Do(func() {
		if ctx == nil {
			ctx = context.Background()
		}
		runCtx, cancel := context.WithCancel(ctx)
		s.cancel = cancel
		timeline.RegisterUIForwarder(s.bus, program)
		go func() {
			_ = s.bus.Run(runCtx)
		}()
	})
}

func (s *Surface) Close() error {
	if s == nil {
		return nil
	}

	var err error
	s.closeOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		if s.evaluator != nil {
			err = s.evaluator.Close()
		}
	})
	return err
}

func installGlobals(evaluator *jsadapter.JavaScriptEvaluator, store Store, options Options) error {
	defaultAuthor := strings.TrimSpace(options.DefaultAuthor)
	if defaultAuthor == "" {
		defaultAuthor = "anonymous"
	}

	helpJSON, err := marshalJSON(map[string]any{
		"stateRoot":    "Shared BBS state root",
		"listMessages": "List all current board messages",
		"post":         "post(subject, body) using the default session author",
		"postAs":       "postAs(author, subject, body)",
	})
	if err != nil {
		return fmt.Errorf("marshal bbs help payload: %w", err)
	}

	globals := map[string]any{
		"__bbsStateRoot": options.StateRoot,
		"__bbsHelpJSON":  helpJSON,
		"__bbsListMessagesJSON": func() (string, error) {
			records, err := listMessages(store)
			if err != nil {
				return "", err
			}
			return marshalJSON(records)
		},
		"__bbsPostJSON": func(subject string, body string) (string, error) {
			record, err := createMessage(store, defaultAuthor, subject, body)
			if err != nil {
				return "", err
			}
			return marshalJSON(record)
		},
		"__bbsPostAsJSON": func(author string, subject string, body string) (string, error) {
			record, err := createMessage(store, author, subject, body)
			if err != nil {
				return "", err
			}
			return marshalJSON(record)
		},
	}
	for name, value := range globals {
		if err := evaluator.Core().SetVariable(name, value); err != nil {
			return fmt.Errorf("install javascript global %s: %w", name, err)
		}
	}

	const script = `
globalThis.bbs = {
  stateRoot: __bbsStateRoot,
  help() {
    return JSON.parse(__bbsHelpJSON);
  },
  listMessages() {
    return JSON.parse(__bbsListMessagesJSON());
  },
  post(subject, body) {
    return JSON.parse(__bbsPostJSON(subject, body));
  },
  postAs(author, subject, body) {
    return JSON.parse(__bbsPostAsJSON(author, subject, body));
  }
};
`
	if err := evaluator.Core().LoadScript(context.Background(), "bbs-api.js", script); err != nil {
		return fmt.Errorf("load bbs javascript api shim: %w", err)
	}

	return nil
}

func listMessages(store Store) ([]messageRecord, error) {
	messages, err := store.ListMessages(context.Background())
	if err != nil {
		return nil, err
	}
	records := make([]messageRecord, 0, len(messages))
	for _, message := range messages {
		records = append(records, toMessageRecord(message))
	}
	return records, nil
}

func createMessage(store Store, author string, subject string, body string) (messageRecord, error) {
	message, err := store.CreateMessage(context.Background(), bbsstore.CreateMessageParams{
		Author:  author,
		Subject: subject,
		Body:    body,
	})
	if err != nil {
		return messageRecord{}, err
	}
	return toMessageRecord(message), nil
}

func toMessageRecord(message bbsstore.Message) messageRecord {
	return messageRecord{
		ID:        message.ID,
		Author:    message.Author,
		Subject:   message.Subject,
		Body:      message.Body,
		CreatedAt: message.CreatedAt.Format("2006-01-02T15:04:05.000000000Z07:00"),
	}
}

func marshalJSON(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}
