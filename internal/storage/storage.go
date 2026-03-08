package storage

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/sys/unix"
)

const (
	defaultDevice     = "/dev/vda"
	defaultMountPoint = "/var/lib/go-init"
	defaultFilesystem = "ext4"
	defaultWait       = 5 * time.Second
	devicePoll        = 100 * time.Millisecond
)

type Config struct {
	Enabled    bool
	Device     string
	MountPoint string
	Filesystem string
	WaitFor    time.Duration
}

type Result struct {
	Enabled     bool     `json:"enabled"`
	Device      string   `json:"device,omitempty"`
	MountPoint  string   `json:"mountPoint,omitempty"`
	Filesystem  string   `json:"filesystem,omitempty"`
	DeviceReady bool     `json:"deviceReady"`
	Mounted     bool     `json:"mounted"`
	Directories []string `json:"directories,omitempty"`
	Step        string   `json:"step,omitempty"`
	Error       string   `json:"error,omitempty"`
}

type directorySpec struct {
	path string
	mode os.FileMode
}

var stateDirectories = []directorySpec{
	{path: "ssh", mode: 0o700},
	{path: "app", mode: 0o755},
	{path: "log", mode: 0o755},
	{path: "state", mode: 0o755},
}

func LoadConfigFromEnv() Config {
	return Config{
		Enabled:    boolEnv("GO_INIT_ENABLE_STORAGE", true),
		Device:     stringEnv("GO_INIT_STORAGE_DEVICE", defaultDevice),
		MountPoint: stringEnv("GO_INIT_STORAGE_MOUNT_POINT", defaultMountPoint),
		Filesystem: stringEnv("GO_INIT_STORAGE_FILESYSTEM", defaultFilesystem),
		WaitFor:    durationEnv("GO_INIT_STORAGE_WAIT", defaultWait),
	}
}

func Prepare(logger *log.Logger) (Result, error) {
	return prepare(logger, LoadConfigFromEnv())
}

func prepare(logger *log.Logger, cfg Config) (Result, error) {
	result := Result{
		Enabled:    cfg.Enabled,
		Device:     cfg.Device,
		MountPoint: cfg.MountPoint,
		Filesystem: cfg.Filesystem,
		Step:       "init",
	}
	if !cfg.Enabled {
		result.Step = "disabled"
		logResult(logger, result)
		return result, nil
	}

	if err := waitForDevice(cfg.Device, cfg.WaitFor); err != nil {
		err = fmt.Errorf("wait for storage device %s: %w", cfg.Device, err)
		result.Step = "device-wait"
		result.Error = err.Error()
		logResult(logger, result)
		return result, err
	}
	result.DeviceReady = true

	if err := os.MkdirAll(cfg.MountPoint, 0o755); err != nil {
		err = fmt.Errorf("create mount point %s: %w", cfg.MountPoint, err)
		result.Step = "mountpoint"
		result.Error = err.Error()
		logResult(logger, result)
		return result, err
	}

	result.Step = "mount"
	if err := unix.Mount(cfg.Device, cfg.MountPoint, cfg.Filesystem, 0, ""); err != nil {
		err = fmt.Errorf("mount %s on %s as %s: %w", cfg.Device, cfg.MountPoint, cfg.Filesystem, err)
		result.Error = err.Error()
		logResult(logger, result)
		return result, err
	}
	result.Mounted = true

	directories, err := ensureStateDirectories(cfg.MountPoint)
	if err != nil {
		err = fmt.Errorf("prepare state directories under %s: %w", cfg.MountPoint, err)
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

func ensureStateDirectories(root string) ([]string, error) {
	created := make([]string, 0, len(stateDirectories))
	for _, spec := range stateDirectories {
		path := filepath.Join(root, spec.path)
		if err := os.MkdirAll(path, spec.mode); err != nil {
			return nil, err
		}
		if err := os.Chmod(path, spec.mode); err != nil {
			return nil, err
		}
		created = append(created, path)
	}
	return created, nil
}

func waitForDevice(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		info, err := os.Stat(path)
		if err == nil && info.Mode()&os.ModeDevice != 0 {
			return nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return err
			}
			return fmt.Errorf("%s exists but is not a device", path)
		}
		time.Sleep(devicePoll)
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

func logResult(logger *log.Logger, result Result) {
	if logger == nil {
		return
	}
	logger.Printf("storage: enabled=%t device=%s mount_point=%s fs=%s ready=%t mounted=%t step=%s error=%q dirs=%v",
		result.Enabled,
		result.Device,
		result.MountPoint,
		result.Filesystem,
		result.DeviceReady,
		result.Mounted,
		result.Step,
		result.Error,
		result.Directories,
	)
}
