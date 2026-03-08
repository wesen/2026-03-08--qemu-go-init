package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

func TestWriteArchiveIncludesVirtioRNGModule(t *testing.T) {
	var compressed bytes.Buffer
	moduleData := []byte("virtio-rng-module")
	err := writeArchive(&compressed, time.Unix(1_741_398_400, 0).UTC(), []byte("init"), []archiveFile{{
		Path:    virtioRNGModuleGuestPath,
		Data:    moduleData,
		ModTime: time.Unix(1_741_398_401, 0).UTC(),
		Mode:    0o644,
	}})
	if err != nil {
		t.Fatalf("writeArchive: %v", err)
	}

	zr, err := gzip.NewReader(bytes.NewReader(compressed.Bytes()))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer zr.Close()

	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	entries, err := parseArchive(data)
	if err != nil {
		t.Fatalf("parseArchive: %v", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
		if entry.Name == virtioRNGModuleGuestPath && string(entry.Data) != string(moduleData) {
			t.Fatalf("got module data %q, want %q", entry.Data, moduleData)
		}
	}

	for _, required := range []string{"lib", "lib/modules", virtioRNGModuleGuestPath} {
		if !containsName(names, required) {
			t.Fatalf("archive missing %s: %v", required, names)
		}
	}
}

func TestReadModuleDataDecompressesZstd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "virtio_rng.ko.zst")

	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	compressed := encoder.EncodeAll([]byte("ELF-module"), nil)
	encoder.Close()

	if err := os.WriteFile(path, compressed, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	data, err := readModuleData(path)
	if err != nil {
		t.Fatalf("readModuleData: %v", err)
	}

	if string(data) != "ELF-module" {
		t.Fatalf("got %q, want %q", data, "ELF-module")
	}
}

func TestParentDirectories(t *testing.T) {
	got := parentDirectories("/lib/modules/virtio_rng.ko.zst")
	want := []string{"lib", "lib/modules"}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func containsName(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

type parsedEntry struct {
	Name string
	Data []byte
}

func parseArchive(data []byte) ([]parsedEntry, error) {
	var entries []parsedEntry
	offset := 0

	for {
		if offset+110 > len(data) {
			return nil, fmt.Errorf("short header at %d", offset)
		}

		header := data[offset : offset+110]
		offset += 110
		if string(header[:6]) != "070701" {
			return nil, fmt.Errorf("unexpected magic %q", header[:6])
		}

		fileSize, err := parseHex(header[54:62])
		if err != nil {
			return nil, err
		}
		nameSize, err := parseHex(header[94:102])
		if err != nil {
			return nil, err
		}

		if offset+int(nameSize) > len(data) {
			return nil, fmt.Errorf("short name at %d", offset)
		}

		nameBytes := data[offset : offset+int(nameSize)]
		offset += int(nameSize)
		offset += (4 - (offset % 4)) % 4

		name := string(nameBytes[:len(nameBytes)-1])
		if offset+int(fileSize) > len(data) {
			return nil, fmt.Errorf("short file data for %s", name)
		}

		fileData := append([]byte(nil), data[offset:offset+int(fileSize)]...)
		offset += int(fileSize)
		offset += (4 - (offset % 4)) % 4

		entries = append(entries, parsedEntry{Name: name, Data: fileData})
		if name == "TRAILER!!!" {
			return entries, nil
		}
	}
}

func parseHex(data []byte) (uint32, error) {
	value, err := strconv.ParseUint(string(data), 16, 32)
	if err != nil {
		return 0, err
	}
	return uint32(value), nil
}
