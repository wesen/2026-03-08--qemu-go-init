package entropy

import (
	"io/fs"
	"log"
	"os"
	"strconv"
	"strings"
)

const (
	entropyAvailPath = "proc/sys/kernel/random/entropy_avail"
	rngAvailablePath = "sys/class/misc/hw_random/rng_available"
	rngCurrentPath   = "sys/class/misc/hw_random/rng_current"
	hwrngDevicePath  = "dev/hwrng"
)

type Result struct {
	EntropyAvail      int      `json:"entropyAvail,omitempty"`
	EntropyAvailKnown bool     `json:"entropyAvailKnown"`
	HWRNGDevice       bool     `json:"hwrngDevice"`
	RNGCurrent        string   `json:"rngCurrent,omitempty"`
	RNGAvailable      []string `json:"rngAvailable,omitempty"`
	VirtioRNGVisible  bool     `json:"virtioRngVisible"`
	Warnings          []string `json:"warnings,omitempty"`
}

func Probe(logger *log.Logger) Result {
	result := ProbeFromFS(os.DirFS("/"))
	if logger != nil {
		logger.Printf(
			"entropy: entropy_avail=%s hwrng=%t rng_current=%q rng_available=%s virtio_rng_visible=%t warnings=%s",
			formatEntropy(result),
			result.HWRNGDevice,
			emptyIfBlank(result.RNGCurrent, "<none>"),
			emptyIfBlank(strings.Join(result.RNGAvailable, ","), "<none>"),
			result.VirtioRNGVisible,
			emptyIfBlank(strings.Join(result.Warnings, ","), "<none>"),
		)
	}
	return result
}

func ProbeFromFS(root fs.FS) Result {
	result := Result{}

	if value, ok := readIntFile(root, entropyAvailPath); ok {
		result.EntropyAvail = value
		result.EntropyAvailKnown = true
	} else {
		result.Warnings = append(result.Warnings, "entropy_avail_unavailable")
	}

	if values, ok := readListFile(root, rngAvailablePath); ok {
		result.RNGAvailable = values
	}

	if value, ok := readStringFile(root, rngCurrentPath); ok {
		result.RNGCurrent = value
	}

	if _, err := fs.Stat(root, hwrngDevicePath); err == nil {
		result.HWRNGDevice = true
	}

	result.VirtioRNGVisible = containsVirtio(result.RNGCurrent) || listContainsVirtio(result.RNGAvailable)
	if !result.HWRNGDevice {
		result.Warnings = append(result.Warnings, "hwrng_device_missing")
	}

	return result
}

func readStringFile(root fs.FS, path string) (string, bool) {
	data, err := fs.ReadFile(root, path)
	if err != nil {
		return "", false
	}
	value := strings.TrimSpace(string(data))
	if value == "" {
		return "", false
	}
	return value, true
}

func readIntFile(root fs.FS, path string) (int, bool) {
	raw, ok := readStringFile(root, path)
	if !ok {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return value, true
}

func readListFile(root fs.FS, path string) ([]string, bool) {
	raw, ok := readStringFile(root, path)
	if !ok {
		return nil, false
	}
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return nil, false
	}
	return fields, true
}

func containsVirtio(value string) bool {
	return strings.Contains(strings.ToLower(value), "virtio")
}

func listContainsVirtio(values []string) bool {
	for _, value := range values {
		if containsVirtio(value) {
			return true
		}
	}
	return false
}

func formatEntropy(result Result) string {
	if !result.EntropyAvailKnown {
		return "unknown"
	}
	return strconv.Itoa(result.EntropyAvail)
}

func emptyIfBlank(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
