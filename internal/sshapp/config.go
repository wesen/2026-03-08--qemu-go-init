package sshapp

import (
	"os"
	"strconv"
	"time"
)

const (
	defaultListenAddr = ":2222"
	defaultHostKey    = "/var/lib/go-init/ssh/ssh_host_ed25519"
	defaultBanner     = "qemu-go-init Wish SSH app"
)

type Config struct {
	Enabled     bool
	ListenAddr  string
	HostKeyPath string
	RequirePTY  bool
	Banner      string
	IdleTimeout time.Duration
	MaxTimeout  time.Duration
}

func LoadConfigFromEnv() Config {
	return Config{
		Enabled:     boolEnv("GO_INIT_ENABLE_SSH", true),
		ListenAddr:  stringEnv("GO_INIT_SSH_ADDR", defaultListenAddr),
		HostKeyPath: stringEnv("GO_INIT_SSH_HOST_KEY_PATH", defaultHostKey),
		RequirePTY:  boolEnv("GO_INIT_SSH_REQUIRE_PTY", true),
		Banner:      stringEnv("GO_INIT_SSH_BANNER", defaultBanner),
		IdleTimeout: durationEnv("GO_INIT_SSH_IDLE_TIMEOUT", 0),
		MaxTimeout:  durationEnv("GO_INIT_SSH_MAX_TIMEOUT", 0),
	}
}

func stringEnv(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func boolEnv(name string, fallback bool) bool {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}

	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func durationEnv(name string, fallback time.Duration) time.Duration {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}
