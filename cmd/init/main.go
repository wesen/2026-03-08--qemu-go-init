package main

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"

	"github.com/manuel/wesen/qemu-go-init/internal/aichat"
	"github.com/manuel/wesen/qemu-go-init/internal/bbsstore"
	"github.com/manuel/wesen/qemu-go-init/internal/boot"
	"github.com/manuel/wesen/qemu-go-init/internal/entropy"
	"github.com/manuel/wesen/qemu-go-init/internal/kmod"
	"github.com/manuel/wesen/qemu-go-init/internal/logstore"
	"github.com/manuel/wesen/qemu-go-init/internal/networking"
	"github.com/manuel/wesen/qemu-go-init/internal/sharedstate"
	"github.com/manuel/wesen/qemu-go-init/internal/sshapp"
	"github.com/manuel/wesen/qemu-go-init/internal/sshbbs"
	"github.com/manuel/wesen/qemu-go-init/internal/storage"
	"github.com/manuel/wesen/qemu-go-init/internal/webui"
	"github.com/manuel/wesen/qemu-go-init/internal/zlog"
)

func main() {
	zlog.Configure(zerolog.WarnLevel)
	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds|log.LUTC)
	boot.StartChildReaper(logger)

	results := boot.PrepareFilesystem(logger)
	storageResult, err := storage.Prepare(logger)
	if err != nil {
		logger.Printf("fatal: prepare storage: %v", err)
		boot.Halt(logger)
	}
	sharedStateResult, err := sharedstate.Prepare(logger)
	if err != nil {
		logger.Printf("shared state unavailable: %v", err)
	}
	chatRoot := filepath.Join(storageResult.MountPoint, "state", "chat")
	logStore, err := logstore.Open(filepath.Join(chatRoot, "logs.db"))
	if err != nil {
		logger.Printf("fatal: open log store: %v", err)
		boot.Halt(logger)
	}
	defer logStore.Close()
	combinedLogWriter := io.MultiWriter(os.Stdout, logStore.Writer("guest"))
	zlog.ConfigureWithWriter(zerolog.WarnLevel, combinedLogWriter)
	logger.SetOutput(combinedLogWriter)
	moduleResult := kmod.LoadVirtioRNG(logger)
	entropyResult := entropy.Probe(logger)
	if moduleResult.Loaded {
		entropyResult = entropy.WaitForVirtioRNG(logger, 2*time.Second)
	}
	networkResult, err := networking.Configure(logger)
	if err != nil {
		logger.Printf("fatal: configure networking: %v", err)
		boot.Halt(logger)
	}
	bbsRoot := sharedstate.BBSRoot(sharedStateResult, storageResult.MountPoint+"/app")
	store, err := bbsstore.Open(bbsRoot)
	if err != nil {
		logger.Printf("fatal: open bbs store: %v", err)
		boot.Halt(logger)
	}
	defer store.Close()
	configureGuestTLSDefaults()
	chatOptions := aichat.Options{
		Title:         "qemu-go-init AI chat",
		StateRoot:     bbsRoot,
		ChatStateRoot: chatRoot,
	}
	addr := boot.HTTPAddress()
	sshService, err := sshapp.Start(logger, sshapp.LoadConfigFromEnv(), func() sshapp.Snapshot {
		return sshapp.Snapshot{
			Hostname:          hostname(),
			PID:               os.Getpid(),
			HTTPAddress:       addr,
			NetworkMethod:     networkResult.Method,
			NetworkInterface:  networkResult.InterfaceName,
			NetworkAddress:    networkResult.CIDR,
			NetworkConfigured: networkResult.Configured,
			EntropyAvail:      entropyResult.EntropyAvail,
			EntropyKnown:      entropyResult.EntropyAvailKnown,
			VirtioRNGVisible:  entropyResult.VirtioRNGVisible,
		}
	}, sshbbs.Middleware(store, chatRoot))
	if err != nil {
		logger.Printf("fatal: start ssh app: %v", err)
		boot.Halt(logger)
	}
	sshStatus := sshService.Status()

	handler, err := webui.NewHandler(webui.Options{
		ListenAddr:      addr,
		Mounts:          results,
		Storage:         storageResult,
		SharedState:     sharedStateResult,
		Network:         networkResult,
		Entropy:         entropyResult,
		VirtioRNGModule: moduleResult,
		SSHStatus:       sshService.Status,
		AIChatDebug: func(ctx context.Context) (any, error) {
			return aichat.DebugSnapshot(ctx, chatOptions)
		},
		AIChatHTTPSProbe: func(ctx context.Context) (any, error) {
			return aichat.ProbeProviderHTTPS(ctx, chatOptions)
		},
		LogDebug: func(ctx context.Context) (any, error) {
			return logStore.Snapshot(ctx)
		},
	})
	if err != nil {
		logger.Printf("fatal: build handler: %v", err)
		boot.Halt(logger)
	}

	logger.Printf("go init ready http=%s ssh=%s storage=%s shared=%s", addr, sshStatus.ListenAddr, storageResult.MountPoint, sharedStateResult.MountPoint)
	if err := boot.ServeHTTP(addr, handler, logger); err != nil {
		logger.Printf("fatal: serve http: %v", err)
		boot.Halt(logger)
	}
}

func hostname() string {
	value, err := os.Hostname()
	if err != nil {
		return ""
	}
	return value
}

func configureGuestTLSDefaults() {
	const bundlePath = "/etc/ssl/certs/ca-certificates.crt"
	if _, err := os.Stat(bundlePath); err == nil {
		if _, exists := os.LookupEnv("SSL_CERT_FILE"); !exists {
			_ = os.Setenv("SSL_CERT_FILE", bundlePath)
		}
	}
}
