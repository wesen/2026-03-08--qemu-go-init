package sharedstate

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/manuel/wesen/qemu-go-init/internal/kmod"
	"golang.org/x/sys/unix"
)

const (
	defaultMountTag     = "hostshare"
	defaultMountPoint   = "/var/lib/go-init/shared"
	defaultFilesystem   = "9p"
	defaultMountOptions = "trans=virtio,version=9p2000.L,cache=none,msize=262144"
)

type Config struct {
	Enabled      bool
	Required     bool
	MountTag     string
	MountPoint   string
	Filesystem   string
	MountOptions string
}

type Result struct {
	Enabled      bool          `json:"enabled"`
	Required     bool          `json:"required"`
	MountTag     string        `json:"mountTag,omitempty"`
	MountPoint   string        `json:"mountPoint,omitempty"`
	Filesystem   string        `json:"filesystem,omitempty"`
	MountOptions string        `json:"mountOptions,omitempty"`
	Mounted      bool          `json:"mounted"`
	Step         string        `json:"step,omitempty"`
	Error        string        `json:"error,omitempty"`
	Directories  []string      `json:"directories,omitempty"`
	Modules      []kmod.Result `json:"modules,omitempty"`
}

func LoadConfigFromEnv() Config {
	return Config{
		Enabled:      boolEnv("GO_INIT_ENABLE_SHARED_STATE", true),
		Required:     boolEnv("GO_INIT_SHARED_STATE_REQUIRED", false),
		MountTag:     stringEnv("GO_INIT_SHARED_STATE_TAG", defaultMountTag),
		MountPoint:   stringEnv("GO_INIT_SHARED_STATE_MOUNT_POINT", defaultMountPoint),
		Filesystem:   stringEnv("GO_INIT_SHARED_STATE_FILESYSTEM", defaultFilesystem),
		MountOptions: stringEnv("GO_INIT_SHARED_STATE_MOUNT_OPTIONS", defaultMountOptions),
	}
}

func Prepare(logger *log.Logger) (Result, error) {
	return prepare(logger, LoadConfigFromEnv())
}

func prepare(logger *log.Logger, cfg Config) (Result, error) {
	result := Result{
		Enabled:      cfg.Enabled,
		Required:     cfg.Required,
		MountTag:     cfg.MountTag,
		MountPoint:   cfg.MountPoint,
		Filesystem:   cfg.Filesystem,
		MountOptions: cfg.MountOptions,
		Step:         "init",
	}
	if !cfg.Enabled {
		result.Step = "disabled"
		logResult(logger, result)
		return result, nil
	}

	result.Modules = kmod.LoadNinePStack(logger)

	if err := os.MkdirAll(cfg.MountPoint, 0o755); err != nil {
		err = fmt.Errorf("create shared-state mount point %s: %w", cfg.MountPoint, err)
		result.Step = "mountpoint"
		result.Error = err.Error()
		logResult(logger, result)
		return result, err
	}

	result.Step = "mount"
	if err := unix.Mount(cfg.MountTag, cfg.MountPoint, cfg.Filesystem, 0, cfg.MountOptions); err != nil {
		err = fmt.Errorf("mount %s on %s as %s: %w", cfg.MountTag, cfg.MountPoint, cfg.Filesystem, err)
		result.Error = err.Error()
		logResult(logger, result)
		return result, err
	}
	result.Mounted = true

	directories, err := ensureDirectories(cfg.MountPoint)
	if err != nil {
		err = fmt.Errorf("prepare shared-state directories under %s: %w", cfg.MountPoint, err)
		result.Step = "directories"
		result.Error = err.Error()
		logResult(logger, result)
		return result, err
	}

	result.Step = "ready"
	result.Directories = directories
	logResult(logger, result)
	return result, nil
}

func BBSRoot(result Result, fallbackRoot string) string {
	if result.Mounted && result.MountPoint != "" {
		return filepath.Join(result.MountPoint, "bbs")
	}
	return filepath.Join(fallbackRoot, "bbs")
}

func ensureDirectories(root string) ([]string, error) {
	directories := []string{
		filepath.Join(root, "bbs"),
		filepath.Join(root, "bbs", "uploads"),
	}

	for _, directory := range directories {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return nil, err
		}
	}

	return directories, nil
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

func logResult(logger *log.Logger, result Result) {
	if logger == nil {
		return
	}

	logger.Printf(
		"sharedstate: enabled=%t required=%t tag=%s mount_point=%s fs=%s mounted=%t step=%s error=%q dirs=%v",
		result.Enabled,
		result.Required,
		result.MountTag,
		result.MountPoint,
		result.Filesystem,
		result.Mounted,
		result.Step,
		result.Error,
		result.Directories,
	)
}
