package kmod

import (
	"errors"
	"log"
	"os"

	"golang.org/x/sys/unix"
)

const VirtioRNGModulePath = "/lib/modules/virtio_rng.ko"

type Result struct {
	Attempted     bool   `json:"attempted"`
	Loaded        bool   `json:"loaded"`
	AlreadyLoaded bool   `json:"alreadyLoaded,omitempty"`
	ModulePath    string `json:"modulePath,omitempty"`
	Step          string `json:"step,omitempty"`
	Error         string `json:"error,omitempty"`
}

type statFunc func(string) (os.FileInfo, error)
type moduleLoader func(string) error

var finitModule = unix.FinitModule

func LoadVirtioRNG(logger *log.Logger) Result {
	return ensureModule(VirtioRNGModulePath, logger, os.Stat, loadModuleFile)
}

func ensureModule(path string, logger *log.Logger, stat statFunc, loader moduleLoader) Result {
	result := Result{
		ModulePath: path,
		Step:       "stat",
	}

	if _, err := stat(path); err != nil {
		result.Error = err.Error()
		result.Step = "missing"
		logResult(logger, result)
		return result
	}

	result.Attempted = true
	result.Step = "load"
	if err := loader(path); err != nil {
		if errors.Is(err, unix.EEXIST) {
			result.Loaded = true
			result.AlreadyLoaded = true
			result.Step = "already-loaded"
			logResult(logger, result)
			return result
		}

		result.Error = err.Error()
		result.Step = "error"
		logResult(logger, result)
		return result
	}

	result.Loaded = true
	result.Step = "loaded"
	logResult(logger, result)
	return result
}

func loadModuleFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return finitModule(int(file.Fd()), "", 0)
}

func logResult(logger *log.Logger, result Result) {
	if logger == nil {
		return
	}

	logger.Printf(
		"kmod: module=%s attempted=%t loaded=%t already_loaded=%t step=%s error=%q",
		result.ModulePath,
		result.Attempted,
		result.Loaded,
		result.AlreadyLoaded,
		result.Step,
		result.Error,
	)
}
