package sshapp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/keygen"
	"github.com/charmbracelet/ssh"
	wish "github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/logging"
	gossh "golang.org/x/crypto/ssh"
)

const authModeNone = "none"

type Snapshot struct {
	Hostname          string
	PID               int
	HTTPAddress       string
	NetworkMethod     string
	NetworkInterface  string
	NetworkAddress    string
	NetworkConfigured bool
	EntropyAvail      int
	EntropyKnown      bool
	VirtioRNGVisible  bool
}

type SnapshotFunc func() Snapshot

type Status struct {
	Enabled        bool   `json:"enabled"`
	Started        bool   `json:"started"`
	ListenAddr     string `json:"listenAddr,omitempty"`
	HostKeyPath    string `json:"hostKeyPath,omitempty"`
	HostKeyPresent bool   `json:"hostKeyPresent"`
	RequirePTY     bool   `json:"requirePty"`
	AuthMode       string `json:"authMode,omitempty"`
	StartedAt      string `json:"startedAt,omitempty"`
	Uptime         string `json:"uptime,omitempty"`
	ActiveSessions uint64 `json:"activeSessions,omitempty"`
	TotalSessions  uint64 `json:"totalSessions,omitempty"`
	Error          string `json:"error,omitempty"`
}

type sessionView interface {
	User() string
	RemoteAddr() net.Addr
	Command() []string
	Pty() (ssh.Pty, <-chan ssh.Window, bool)
}

type Service struct {
	cfg       Config
	logger    *log.Logger
	server    *ssh.Server
	listener  net.Listener
	snapshot  SnapshotFunc
	active    atomic.Uint64
	total     atomic.Uint64
	mu        sync.RWMutex
	started   bool
	startedAt time.Time
	hostKey   bool
	errText   string
}

func Start(logger *log.Logger, cfg Config, snapshot SnapshotFunc) (*Service, error) {
	service := &Service{
		cfg:      cfg,
		logger:   logger,
		snapshot: snapshot,
	}
	if !cfg.Enabled {
		return service, nil
	}

	if err := ensureHostKey(cfg.HostKeyPath); err != nil {
		return nil, fmt.Errorf("ensure ssh host key: %w", err)
	}

	middlewares := []wish.Middleware{logging.Middleware()}
	middlewares = append(middlewares, service.sessionMiddleware())
	if cfg.RequirePTY {
		middlewares = append(middlewares, activeterm.Middleware())
	}

	options := []ssh.Option{
		wish.WithHostKeyPath(cfg.HostKeyPath),
		wish.WithMiddleware(middlewares...),
	}
	if cfg.Banner != "" {
		options = append(options, wish.WithBanner(cfg.Banner))
	}
	if cfg.IdleTimeout > 0 {
		options = append(options, wish.WithIdleTimeout(cfg.IdleTimeout))
	}
	if cfg.MaxTimeout > 0 {
		options = append(options, wish.WithMaxTimeout(cfg.MaxTimeout))
	}

	server, err := wish.NewServer(options...)
	if err != nil {
		return nil, fmt.Errorf("create wish server: %w", err)
	}
	service.server = server
	service.hostKey = fileExists(cfg.HostKeyPath)

	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", cfg.ListenAddr, err)
	}
	service.listener = listener
	service.mu.Lock()
	service.started = true
	service.startedAt = time.Now().UTC()
	service.mu.Unlock()

	go service.serve()
	service.logf("sshapp: listening on %s hostkey=%s require_pty=%t auth=%s",
		listener.Addr().String(),
		cfg.HostKeyPath,
		cfg.RequirePTY,
		authModeNone,
	)

	return service, nil
}

func (s *Service) Status() Status {
	s.mu.RLock()
	started := s.started
	startedAt := s.startedAt
	hostKeyPresent := s.hostKey
	errText := s.errText
	s.mu.RUnlock()

	status := Status{
		Enabled:        s.cfg.Enabled,
		Started:        started,
		HostKeyPath:    s.cfg.HostKeyPath,
		HostKeyPresent: hostKeyPresent,
		RequirePTY:     s.cfg.RequirePTY,
		AuthMode:       authModeNone,
		ActiveSessions: s.active.Load(),
		TotalSessions:  s.total.Load(),
		Error:          errText,
	}
	if s.listener != nil {
		status.ListenAddr = s.listener.Addr().String()
	} else {
		status.ListenAddr = s.cfg.ListenAddr
	}
	if started {
		status.StartedAt = startedAt.Format(time.RFC3339)
		status.Uptime = time.Since(startedAt).Round(time.Second).String()
	}
	return status
}

func (s *Service) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	if ctx == nil {
		ctx = context.Background()
	}
	err := s.server.Shutdown(ctx)
	if err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Service) serve() {
	if s.server == nil || s.listener == nil {
		return
	}
	if err := s.server.Serve(s.listener); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		s.setError(err)
		s.logf("sshapp: serve failed: %v", err)
	}
}

func (s *Service) sessionMiddleware() wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(session ssh.Session) {
			s.active.Add(1)
			s.total.Add(1)
			defer s.active.Add(^uint64(0))

			_, _ = io.WriteString(session, renderSession(session, s.snapshot, s.Status()))
			_ = session.Exit(0)
			if next != nil {
				next(session)
			}
		}
	}
}

func renderSession(session sessionView, snapshotFn SnapshotFunc, status Status) string {
	var snapshot Snapshot
	if snapshotFn != nil {
		snapshot = snapshotFn()
	}

	var builder strings.Builder
	builder.WriteString("qemu-go-init / wish\n")
	builder.WriteString("===================\n")
	builder.WriteString("This guest is a single Go PID 1 runtime.\n")
	builder.WriteString("It exposes an SSH app, not a general shell.\n\n")

	builder.WriteString("Runtime\n")
	builder.WriteString(fmt.Sprintf("  host: %s\n", fallback(snapshot.Hostname, "unknown")))
	builder.WriteString(fmt.Sprintf("  pid: %s\n", renderInt(snapshot.PID)))
	builder.WriteString(fmt.Sprintf("  ssh: %s\n", fallback(status.ListenAddr, "unknown")))
	builder.WriteString(fmt.Sprintf("  http: %s\n", fallback(snapshot.HTTPAddress, "disabled")))

	builder.WriteString("\nNetwork\n")
	if snapshot.NetworkConfigured {
		builder.WriteString(fmt.Sprintf("  interface: %s\n", fallback(snapshot.NetworkInterface, "unknown")))
		builder.WriteString(fmt.Sprintf("  address: %s\n", fallback(snapshot.NetworkAddress, "unknown")))
		builder.WriteString(fmt.Sprintf("  method: %s\n", fallback(snapshot.NetworkMethod, "unknown")))
	} else {
		builder.WriteString(fmt.Sprintf("  status: pending (%s)\n", fallback(snapshot.NetworkMethod, "unknown")))
	}

	builder.WriteString("\nEntropy\n")
	builder.WriteString(fmt.Sprintf("  available: %s\n", renderEntropy(snapshot.EntropyAvail, snapshot.EntropyKnown)))
	builder.WriteString(fmt.Sprintf("  virtio-rng: %s\n", renderBool(snapshot.VirtioRNGVisible)))

	builder.WriteString("\nSession\n")
	builder.WriteString(fmt.Sprintf("  user: %s\n", fallback(session.User(), "unknown")))
	builder.WriteString(fmt.Sprintf("  remote: %s\n", session.RemoteAddr()))
	builder.WriteString(fmt.Sprintf("  pty-required: %s\n", renderBool(status.RequirePTY)))
	builder.WriteString(fmt.Sprintf("  command: %q\n", session.Command()))
	if pty, _, ok := session.Pty(); ok {
		builder.WriteString(fmt.Sprintf("  pty: %s %dx%d\n", pty.Term, pty.Window.Width, pty.Window.Height))
	} else {
		builder.WriteString("  pty: none\n")
	}

	builder.WriteString("\nHost key\n")
	builder.WriteString(fmt.Sprintf("  path: %s\n", fallback(status.HostKeyPath, "unknown")))
	builder.WriteString(fmt.Sprintf("  present: %s\n", renderBool(status.HostKeyPresent)))

	builder.WriteString("\nConnection closed intentionally after status render.\n")
	return builder.String()
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func ensureHostKey(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	if valid, err := hostKeyValid(path); err != nil {
		return err
	} else if valid {
		return nil
	}

	_ = os.Remove(path)
	_ = os.Remove(path + ".pub")

	pair, err := keygen.New(path, keygen.WithKeyType(keygen.Ed25519))
	if err != nil {
		return err
	}
	if err := writeAtomic(path, pair.RawPrivateKey(), 0o600); err != nil {
		return err
	}
	pub := []byte(strings.TrimSpace(pair.AuthorizedKey()) + "\n")
	if err := writeAtomic(path+".pub", pub, 0o600); err != nil {
		return err
	}
	if valid, err := hostKeyValid(path); err != nil {
		return err
	} else if !valid {
		return fmt.Errorf("generated host key at %s is not readable", path)
	}
	return nil
}

func hostKeyValid(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if len(data) == 0 {
		return false, nil
	}
	if _, err := gossh.ParseRawPrivateKey(data); err != nil {
		return false, nil
	}
	return true, nil
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}

	dirHandle, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer dirHandle.Close()
	return dirHandle.Sync()
}

func fallback(value string, alternate string) string {
	if value == "" {
		return alternate
	}
	return value
}

func renderBool(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func renderEntropy(value int, known bool) string {
	if !known {
		return "unknown"
	}
	return strconv.Itoa(value)
}

func renderInt(value int) string {
	if value == 0 {
		return "unknown"
	}
	return strconv.Itoa(value)
}

func (s *Service) setError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil {
		s.errText = ""
		return
	}
	s.errText = err.Error()
}

func (s *Service) logf(format string, args ...any) {
	if s.logger != nil {
		s.logger.Printf(format, args...)
	}
}
