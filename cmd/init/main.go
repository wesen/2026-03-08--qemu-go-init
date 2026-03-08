package main

import (
	"log"
	"os"
	"time"

	"github.com/manuel/wesen/qemu-go-init/internal/boot"
	"github.com/manuel/wesen/qemu-go-init/internal/entropy"
	"github.com/manuel/wesen/qemu-go-init/internal/kmod"
	"github.com/manuel/wesen/qemu-go-init/internal/networking"
	"github.com/manuel/wesen/qemu-go-init/internal/sshapp"
	"github.com/manuel/wesen/qemu-go-init/internal/storage"
	"github.com/manuel/wesen/qemu-go-init/internal/webui"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds|log.LUTC)
	boot.StartChildReaper(logger)

	results := boot.PrepareFilesystem(logger)
	storageResult, err := storage.Prepare(logger)
	if err != nil {
		logger.Printf("fatal: prepare storage: %v", err)
		boot.Halt(logger)
	}
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
	})
	if err != nil {
		logger.Printf("fatal: start ssh app: %v", err)
		boot.Halt(logger)
	}
	sshStatus := sshService.Status()

	handler, err := webui.NewHandler(webui.Options{
		ListenAddr:      addr,
		Mounts:          results,
		Storage:         storageResult,
		Network:         networkResult,
		Entropy:         entropyResult,
		VirtioRNGModule: moduleResult,
		SSHStatus:       sshService.Status,
	})
	if err != nil {
		logger.Printf("fatal: build handler: %v", err)
		boot.Halt(logger)
	}

	logger.Printf("go init ready http=%s ssh=%s storage=%s", addr, sshStatus.ListenAddr, storageResult.MountPoint)
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
