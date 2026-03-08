package sshapp

import (
	"context"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/ssh"
)

type stubSession struct {
	user    string
	command []string
	remote  net.Addr
	pty     bool
}

func (s stubSession) User() string {
	return s.user
}

func (s stubSession) RemoteAddr() net.Addr {
	return s.remote
}

func (s stubSession) Command() []string {
	return s.command
}

func (s stubSession) Pty() (ssh.Pty, <-chan ssh.Window, bool) {
	ch := make(chan ssh.Window)
	close(ch)
	return ssh.Pty{}, ch, s.pty
}

func TestLoadConfigFromEnvDefaults(t *testing.T) {
	t.Setenv("GO_INIT_ENABLE_SSH", "")
	t.Setenv("GO_INIT_SSH_ADDR", "")
	t.Setenv("GO_INIT_SSH_HOST_KEY_PATH", "")
	t.Setenv("GO_INIT_SSH_REQUIRE_PTY", "")
	t.Setenv("GO_INIT_SSH_BANNER", "")

	cfg := LoadConfigFromEnv()
	if !cfg.Enabled {
		t.Fatalf("expected ssh to be enabled by default")
	}
	if cfg.ListenAddr != defaultListenAddr {
		t.Fatalf("unexpected listen addr: %q", cfg.ListenAddr)
	}
	if cfg.HostKeyPath != defaultHostKey {
		t.Fatalf("unexpected host key path: %q", cfg.HostKeyPath)
	}
	if !cfg.RequirePTY {
		t.Fatalf("expected PTY requirement to default to true")
	}
}

func TestLoadConfigFromEnvOverrides(t *testing.T) {
	t.Setenv("GO_INIT_ENABLE_SSH", "false")
	t.Setenv("GO_INIT_SSH_ADDR", "127.0.0.1:2022")
	t.Setenv("GO_INIT_SSH_HOST_KEY_PATH", "/tmp/test-host-key")
	t.Setenv("GO_INIT_SSH_REQUIRE_PTY", "false")
	t.Setenv("GO_INIT_SSH_BANNER", "custom banner")

	cfg := LoadConfigFromEnv()
	if cfg.Enabled {
		t.Fatalf("expected ssh to be disabled")
	}
	if cfg.ListenAddr != "127.0.0.1:2022" {
		t.Fatalf("unexpected listen addr: %q", cfg.ListenAddr)
	}
	if cfg.HostKeyPath != "/tmp/test-host-key" {
		t.Fatalf("unexpected host key path: %q", cfg.HostKeyPath)
	}
	if cfg.RequirePTY {
		t.Fatalf("expected PTY requirement to be false")
	}
	if cfg.Banner != "custom banner" {
		t.Fatalf("unexpected banner: %q", cfg.Banner)
	}
}

func TestRenderSession(t *testing.T) {
	body := renderSession(stubSession{
		user:    "tester",
		command: []string{"status"},
		remote:  &net.IPAddr{IP: net.ParseIP("127.0.0.1")},
	}, func() Snapshot {
		return Snapshot{
			Hostname:          "guest",
			PID:               1,
			HTTPAddress:       ":8080",
			NetworkMethod:     "userspace-dhcp",
			NetworkInterface:  "eth0",
			NetworkAddress:    "10.0.2.15/24",
			NetworkConfigured: true,
			EntropyAvail:      256,
			EntropyKnown:      true,
			VirtioRNGVisible:  true,
		}
	}, Status{
		ListenAddr:     ":2222",
		HostKeyPath:    "/var/lib/go-init/ssh/ssh_host_ed25519",
		HostKeyPresent: true,
		RequirePTY:     true,
	})

	for _, expected := range []string{
		"qemu-go-init / wish",
		"host: guest",
		"http: :8080",
		"address: 10.0.2.15/24",
		"virtio-rng: yes",
		`command: ["status"]`,
		"path: /var/lib/go-init/ssh/ssh_host_ed25519",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected body to contain %q\n%s", expected, body)
		}
	}
}

func TestStartAndShutdown(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	hostKeyPath := filepath.Join(tempDir, "ssh_host_ed25519")
	service, err := Start(log.New(os.Stdout, "", 0), Config{
		Enabled:     true,
		ListenAddr:  "127.0.0.1:0",
		HostKeyPath: hostKeyPath,
		RequirePTY:  false,
		Banner:      "",
	}, func() Snapshot {
		return Snapshot{Hostname: "guest", PID: 1}
	})
	if err != nil {
		t.Fatalf("start ssh service: %v", err)
	}
	t.Cleanup(func() {
		_ = service.Shutdown(context.Background())
	})

	status := service.Status()
	if !status.Started {
		t.Fatalf("expected started status")
	}
	if !fileExists(hostKeyPath) {
		t.Fatalf("expected host key to exist at %s", hostKeyPath)
	}
	if status.ListenAddr == "" {
		t.Fatalf("expected listen addr")
	}
}
