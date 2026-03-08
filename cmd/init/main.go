package main

import (
	"log"
	"os"
	"time"

	"github.com/manuel/wesen/qemu-go-init/internal/boot"
	"github.com/manuel/wesen/qemu-go-init/internal/entropy"
	"github.com/manuel/wesen/qemu-go-init/internal/kmod"
	"github.com/manuel/wesen/qemu-go-init/internal/networking"
	"github.com/manuel/wesen/qemu-go-init/internal/webui"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds|log.LUTC)
	boot.StartChildReaper(logger)

	results := boot.PrepareFilesystem(logger)
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

	handler, err := webui.NewHandler(webui.Options{
		ListenAddr:      addr,
		Mounts:          results,
		Network:         networkResult,
		Entropy:         entropyResult,
		VirtioRNGModule: moduleResult,
	})
	if err != nil {
		logger.Printf("fatal: build handler: %v", err)
		boot.Halt(logger)
	}

	logger.Printf("go init ready on %s", addr)
	if err := boot.ServeHTTP(addr, handler, logger); err != nil {
		logger.Printf("fatal: serve http: %v", err)
		boot.Halt(logger)
	}
}
