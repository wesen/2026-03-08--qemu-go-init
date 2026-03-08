package jsrepl

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dop251/goja"
	"github.com/go-go-golems/bobatea/pkg/eventbus"
	bobarepl "github.com/go-go-golems/bobatea/pkg/repl"
	"github.com/go-go-golems/bobatea/pkg/timeline"
	ggjengine "github.com/go-go-golems/go-go-goja/engine"
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

type evaluator struct {
	runtime *ggjengine.Runtime
}

func newEvaluator() (*evaluator, error) {
	factory, err := ggjengine.NewBuilder().
		WithModules(ggjengine.DefaultRegistryModules()).
		Build()
	if err != nil {
		return nil, fmt.Errorf("build go-go-goja runtime factory: %w", err)
	}

	runtime, err := factory.NewRuntime(context.Background())
	if err != nil {
		return nil, fmt.Errorf("create go-go-goja runtime: %w", err)
	}

	return &evaluator{runtime: runtime}, nil
}

func (e *evaluator) EvaluateStream(ctx context.Context, code string, emit func(bobarepl.Event)) error {
	result, err := e.evaluate(ctx, code)
	if err != nil {
		emit(bobarepl.Event{
			Kind:  bobarepl.EventResultMarkdown,
			Props: map[string]any{"markdown": fmt.Sprintf("Error: %v", err)},
		})
		return nil
	}
	emit(bobarepl.Event{
		Kind:  bobarepl.EventResultMarkdown,
		Props: map[string]any{"markdown": result},
	})
	return nil
}

func (e *evaluator) GetPrompt() string {
	return "js>"
}

func (e *evaluator) GetName() string {
	return "JavaScript"
}

func (e *evaluator) SupportsMultiline() bool {
	return true
}

func (e *evaluator) GetFileExtension() string {
	return ".js"
}

func (e *evaluator) SetVariable(name string, value any) error {
	_, err := e.runtime.Owner.Call(context.Background(), "set-variable:"+name, func(_ context.Context, vm *goja.Runtime) (any, error) {
		return nil, vm.Set(name, value)
	})
	return err
}

func (e *evaluator) LoadScript(ctx context.Context, filename string, content string) error {
	_, err := e.runtime.Owner.Call(ctx, "load-script:"+filename, func(_ context.Context, vm *goja.Runtime) (any, error) {
		_, runErr := vm.RunString(content)
		return nil, runErr
	})
	return err
}

func (e *evaluator) Close() error {
	if e == nil || e.runtime == nil {
		return nil
	}
	return e.runtime.Close(context.Background())
}

func (e *evaluator) evaluate(ctx context.Context, code string) (string, error) {
	value, err := e.runtime.Owner.Call(ctx, "evaluate-js", func(_ context.Context, vm *goja.Runtime) (any, error) {
		result, runErr := vm.RunString(code)
		if runErr != nil {
			return nil, runErr
		}
		if result == nil || goja.IsUndefined(result) {
			return "undefined", nil
		}
		return result.String(), nil
	})
	if err != nil {
		return "", err
	}

	rendered, ok := value.(string)
	if !ok {
		return fmt.Sprintf("%v", value), nil
	}
	return rendered, nil
}

type Surface struct {
	evaluator *evaluator
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
	cfg.Placeholder = "Try: 2 + 2, bbs.listMessages()[0], bbs.post(\"subject\", \"body\")"
	cfg.EnableExternalEditor = false
	cfg.Autocomplete.Enabled = false
	cfg.HelpBar.Enabled = false
	cfg.HelpDrawer.Enabled = false
	cfg.CommandPalette.Enabled = false

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

func installGlobals(evaluator *evaluator, store Store, options Options) error {
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
		if err := evaluator.SetVariable(name, value); err != nil {
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
	if err := evaluator.LoadScript(context.Background(), "bbs-api.js", script); err != nil {
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
