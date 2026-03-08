package entropy

import (
	"testing"
	"testing/fstest"
	"time"
)

func TestProbeFromFSWithVirtioRNG(t *testing.T) {
	root := fstest.MapFS{
		entropyAvailPath: &fstest.MapFile{Data: []byte("256\n")},
		rngAvailablePath: &fstest.MapFile{Data: []byte("virtio-rng.0 tpm-rng\n")},
		rngCurrentPath:   &fstest.MapFile{Data: []byte("virtio-rng.0\n")},
		hwrngDevicePath:  &fstest.MapFile{},
	}

	result := ProbeFromFS(root)

	if !result.EntropyAvailKnown || result.EntropyAvail != 256 {
		t.Fatalf("unexpected entropy result: %#v", result)
	}
	if !result.HWRNGDevice {
		t.Fatal("expected /dev/hwrng to be present")
	}
	if !result.VirtioRNGVisible {
		t.Fatal("expected virtio rng visibility")
	}
	if got, want := result.RNGCurrent, "virtio-rng.0"; got != want {
		t.Fatalf("rng current = %q, want %q", got, want)
	}
}

func TestProbeFromFSMissingOptionalFiles(t *testing.T) {
	root := fstest.MapFS{}

	result := ProbeFromFS(root)

	if result.EntropyAvailKnown {
		t.Fatal("did not expect entropy_avail to be known")
	}
	if result.HWRNGDevice {
		t.Fatal("did not expect /dev/hwrng to exist")
	}
	if len(result.Warnings) < 2 {
		t.Fatalf("expected warnings for missing files, got %#v", result.Warnings)
	}
}

func TestWaitForVirtioRNGStopsWhenVisible(t *testing.T) {
	probes := 0
	result := waitForVirtioRNG(100*time.Millisecond, 0, func() Result {
		probes++
		if probes < 3 {
			return Result{HWRNGDevice: true}
		}
		return Result{
			HWRNGDevice:      true,
			RNGCurrent:       "virtio-rng.0",
			VirtioRNGVisible: true,
		}
	})

	if !result.VirtioRNGVisible {
		t.Fatalf("expected virtio-rng to become visible: %#v", result)
	}
	if probes != 3 {
		t.Fatalf("got %d probes, want 3", probes)
	}
}
