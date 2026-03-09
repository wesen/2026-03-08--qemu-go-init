package aichat

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	bobachat "github.com/go-go-golems/bobatea/pkg/chat"
	"github.com/go-go-golems/geppetto/pkg/events"
	enginefactory "github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/turns"
	pinhelpers "github.com/go-go-golems/pinocchio/pkg/cmds/helpers"
	pinui "github.com/go-go-golems/pinocchio/pkg/ui"
	agentforwarder "github.com/go-go-golems/pinocchio/pkg/ui/forwarders/agent"
	"github.com/manuel/wesen/qemu-go-init/internal/bbsstore"
)

const (
	defaultProfileSlug = "gpt-5-nano"
	defaultTitle       = "qemu-go-init AI chat"
)

type Store interface {
	ListMessages(ctx context.Context) ([]bbsstore.Message, error)
}

type Options struct {
	Title             string
	StateRoot         string
	ProfileSlug       string
	ProfileRegistries string
}

type Surface struct {
	backend *pinui.EngineBackend
	router  *events.EventRouter
	model   tea.Model
	seed    *turns.Turn

	cancel     context.CancelFunc
	attachOnce sync.Once
	closeOnce  sync.Once
}

func New(store Store, options Options) (*Surface, error) {
	details, err := loadRuntimeDetails(context.Background(), options)
	if err != nil {
		return nil, err
	}

	engine, err := enginefactory.NewEngineFromStepSettings(details.resolved.EffectiveStepSettings)
	if err != nil {
		return nil, fmt.Errorf("create pinocchio engine: %w", err)
	}

	router, err := events.NewEventRouter()
	if err != nil {
		return nil, fmt.Errorf("create event router: %w", err)
	}

	sink := middleware.NewWatermillSink(router.Publisher, "chat")
	backend := pinui.NewEngineBackend(engine, sink)
	seed, err := buildSeedTurn(store, options.StateRoot, details.resolved.SystemPrompt)
	if err != nil {
		_ = router.Close()
		return nil, err
	}

	title := strings.TrimSpace(options.Title)
	if title == "" {
		title = defaultTitle
	}

	model := bobachat.InitialModel(
		backend,
		bobachat.WithTitle(title),
	)

	return &Surface{
		backend: backend,
		router:  router,
		model:   model,
		seed:    seed,
	}, nil
}

func (s *Surface) Init() tea.Cmd {
	if s == nil || s.model == nil {
		return nil
	}
	return s.model.Init()
}

func (s *Surface) Update(msg tea.Msg) tea.Cmd {
	if s == nil || s.model == nil {
		return nil
	}
	next, cmd := s.model.Update(msg)
	s.model = next
	return cmd
}

func (s *Surface) View() string {
	if s == nil || s.model == nil {
		return ""
	}
	return s.model.View()
}

func (s *Surface) AttachProgram(ctx context.Context, program *tea.Program) {
	if s == nil || s.backend == nil || s.router == nil || program == nil {
		return
	}
	s.attachOnce.Do(func() {
		if ctx == nil {
			ctx = context.Background()
		}
		runCtx, cancel := context.WithCancel(ctx)
		s.cancel = cancel
		s.backend.AttachProgram(program)
		if s.seed != nil {
			s.backend.SetSeedTurn(s.seed)
		}
		s.router.AddHandler("ui-forward", "chat", agentforwarder.MakeUIForwarder(program))
		go func() {
			_ = s.router.Run(runCtx)
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
		if s.router != nil {
			err = s.router.Close()
		}
	})
	return err
}

func buildSeedTurn(store Store, stateRoot string, profileSystemPrompt string) (*turns.Turn, error) {
	systemParts := []string{
		"You are the resident AI assistant inside a retro SSH BBS.",
		"Keep answers concise, terminal-friendly, and grounded in the current board state when relevant.",
	}
	if trimmed := strings.TrimSpace(profileSystemPrompt); trimmed != "" {
		systemParts = append(systemParts, "", "Profile instructions:", trimmed)
	}
	if trimmed := strings.TrimSpace(stateRoot); trimmed != "" {
		systemParts = append(systemParts, "", fmt.Sprintf("Board state root: %s", trimmed))
	}

	messages, err := store.ListMessages(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list bbs messages for chat seed: %w", err)
	}
	if len(messages) > 0 {
		systemParts = append(systemParts, "", "Current BBS messages:")
		limit := min(8, len(messages))
		for i := 0; i < limit; i++ {
			message := messages[i]
			systemParts = append(systemParts,
				fmt.Sprintf("- %s by %s at %s", message.Subject, message.Author, message.CreatedAt.UTC().Format("2006-01-02 15:04Z")),
				indent(message.Body, "  "),
			)
		}
	}

	seed := &turns.Turn{}
	turns.AppendBlock(seed, turns.NewSystemTextBlock(strings.Join(systemParts, "\n")))
	return seed, nil
}

func resolveRuntime(options Options) (configHome string, profileRegistries string, profileSlug string, err error) {
	configHome = strings.TrimSpace(os.Getenv("GO_INIT_PINOCCHIO_CONFIG_HOME"))
	if configHome == "" {
		configHome = guessConfigHome(options.StateRoot)
	}
	if configHome == "" {
		err = fmt.Errorf("could not find a pinocchio config home for state root %q", options.StateRoot)
		return
	}

	profileRegistries = strings.TrimSpace(options.ProfileRegistries)
	if profileRegistries == "" {
		profileRegistries = strings.TrimSpace(os.Getenv("PINOCCHIO_PROFILE_REGISTRIES"))
	}
	if profileRegistries == "" {
		candidate := filepath.Join(configHome, "pinocchio", "profiles.yaml")
		if fileExists(candidate) {
			profileRegistries = candidate
		}
	}
	if profileRegistries == "" {
		err = fmt.Errorf("could not find a pinocchio profile registry under %s", filepath.Join(configHome, "pinocchio"))
		return
	}

	profileSlug = strings.TrimSpace(options.ProfileSlug)
	if profileSlug == "" {
		profileSlug = defaultProfileSlug
	}
	return
}

func guessConfigHome(stateRoot string) string {
	candidates := []string{}

	if trimmed := strings.TrimSpace(stateRoot); trimmed != "" {
		candidates = append(candidates, filepath.Clean(filepath.Dir(trimmed)))
	}
	if raw := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); raw != "" {
		candidates = append(candidates, filepath.Clean(raw))
	}
	if home, err := os.UserConfigDir(); err == nil && strings.TrimSpace(home) != "" {
		candidates = append(candidates, filepath.Clean(home))
	}
	if currentUser, err := user.Current(); err == nil && currentUser != nil && strings.TrimSpace(currentUser.HomeDir) != "" {
		candidates = append(candidates, filepath.Join(currentUser.HomeDir, ".config"))
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		pinocchioDir := filepath.Join(candidate, "pinocchio")
		if fileExists(filepath.Join(pinocchioDir, "config.yaml")) || fileExists(filepath.Join(pinocchioDir, "profiles.yaml")) {
			return candidate
		}
	}
	return ""
}

var resolveBaseSettingsMu sync.Mutex

func resolveBaseStepSettings(configHome string) (*aisettings.StepSettings, error) {
	resolveBaseSettingsMu.Lock()
	defer resolveBaseSettingsMu.Unlock()

	oldValue, hadOldValue := os.LookupEnv("XDG_CONFIG_HOME")
	if err := os.Setenv("XDG_CONFIG_HOME", configHome); err != nil {
		return nil, fmt.Errorf("set XDG_CONFIG_HOME: %w", err)
	}
	defer func() {
		if hadOldValue {
			_ = os.Setenv("XDG_CONFIG_HOME", oldValue)
			return
		}
		_ = os.Unsetenv("XDG_CONFIG_HOME")
	}()

	settings, _, err := pinhelpers.ResolveBaseStepSettings(nil)
	if err != nil {
		return nil, err
	}
	return settings, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func indent(value string, prefix string) string {
	lines := strings.Split(strings.TrimSpace(value), "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
