package initramfs

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"
	"time"
)

func TestWriterCreatesNewcArchive(t *testing.T) {
	timestamp := time.Unix(1_741_398_400, 0).UTC()

	var buf bytes.Buffer
	writer := NewWriter(&buf)

	if err := writer.AddDirectory("dev", 0o755, timestamp); err != nil {
		t.Fatalf("AddDirectory: %v", err)
	}
	if err := writer.AddCharDevice("dev/console", 0o600, timestamp, 5, 1); err != nil {
		t.Fatalf("AddCharDevice: %v", err)
	}
	if err := writer.AddFile("init", 0o755, timestamp, []byte("hello")); err != nil {
		t.Fatalf("AddFile: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	entries, err := parseArchive(buf.Bytes())
	if err != nil {
		t.Fatalf("parseArchive: %v", err)
	}

	if len(entries) != 4 {
		t.Fatalf("got %d entries, want 4", len(entries))
	}

	if entries[0].Name != "dev" || entries[0].Mode != 0o040755 {
		t.Fatalf("unexpected directory entry: %#v", entries[0])
	}
	if entries[1].Name != "dev/console" || entries[1].Mode != 0o020600 || entries[1].RDevMajor != 5 || entries[1].RDevMinor != 1 {
		t.Fatalf("unexpected char device entry: %#v", entries[1])
	}
	if entries[2].Name != "init" || string(entries[2].Data) != "hello" {
		t.Fatalf("unexpected file entry: %#v", entries[2])
	}
	if entries[3].Name != trailer {
		t.Fatalf("unexpected trailer entry: %#v", entries[3])
	}
}

type parsedEntry struct {
	Name      string
	Mode      uint32
	FileSize  uint32
	RDevMajor uint32
	RDevMinor uint32
	Data      []byte
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

		if string(header[:6]) != newcMagic {
			return nil, fmt.Errorf("unexpected magic %q", header[:6])
		}

		mode, err := parseHex(header[14:22])
		if err != nil {
			return nil, err
		}
		fileSize, err := parseHex(header[54:62])
		if err != nil {
			return nil, err
		}
		rdevMajor, err := parseHex(header[78:86])
		if err != nil {
			return nil, err
		}
		rdevMinor, err := parseHex(header[86:94])
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

		entry := parsedEntry{
			Name:      name,
			Mode:      mode,
			FileSize:  fileSize,
			RDevMajor: rdevMajor,
			RDevMinor: rdevMinor,
			Data:      fileData,
		}
		entries = append(entries, entry)
		if name == trailer {
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
