package entropy

import (
	"testing"
	"testing/fstest"
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
