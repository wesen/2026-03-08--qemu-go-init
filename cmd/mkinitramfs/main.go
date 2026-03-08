package main

import (
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/manuel/wesen/qemu-go-init/internal/initramfs"
)

const virtioRNGModuleGuestPath = "lib/modules/virtio_rng.ko"

type archiveFile struct {
	Path    string
	Data    []byte
	ModTime time.Time
	Mode    fs.FileMode
}

func main() {
	var (
		initPath           = flag.String("init-bin", "", "path to the statically linked /init binary")
		output             = flag.String("output", "", "path to the initramfs.cpio.gz output file")
		virtioRNGModuleSrc = flag.String("virtio-rng-module-src", "", "optional path to a virtio_rng kernel module to include in the initramfs")
	)

	flag.Parse()

	if err := run(*initPath, *output, *virtioRNGModuleSrc); err != nil {
		fmt.Fprintf(os.Stderr, "mkinitramfs: %v\n", err)
		os.Exit(1)
	}
}

func run(initPath string, output string, virtioRNGModuleSrc string) error {
	if initPath == "" {
		return fmt.Errorf("-init-bin is required")
	}
	if output == "" {
		return fmt.Errorf("-output is required")
	}

	initData, err := os.ReadFile(initPath)
	if err != nil {
		return fmt.Errorf("read init binary: %w", err)
	}

	info, err := os.Stat(initPath)
	if err != nil {
		return fmt.Errorf("stat init binary: %w", err)
	}

	extras, err := readExtraFiles(virtioRNGModuleSrc)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	file, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer file.Close()

	if err := writeArchive(file, info.ModTime().UTC(), initData, extras); err != nil {
		return err
	}

	return nil
}

func readExtraFiles(virtioRNGModuleSrc string) ([]archiveFile, error) {
	if virtioRNGModuleSrc == "" {
		return nil, nil
	}

	data, err := readModuleData(virtioRNGModuleSrc)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(virtioRNGModuleSrc)
	if err != nil {
		return nil, fmt.Errorf("stat virtio-rng module: %w", err)
	}

	return []archiveFile{{
		Path:    virtioRNGModuleGuestPath,
		Data:    data,
		ModTime: info.ModTime().UTC(),
		Mode:    0o644,
	}}, nil
}

func readModuleData(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read virtio-rng module: %w", err)
	}
	if !strings.HasSuffix(path, ".zst") {
		return data, nil
	}

	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return nil, fmt.Errorf("create zstd decoder: %w", err)
	}
	defer decoder.Close()

	decoded, err := decoder.DecodeAll(data, nil)
	if err != nil {
		return nil, fmt.Errorf("decompress virtio-rng module: %w", err)
	}
	return decoded, nil
}

func writeArchive(w io.Writer, modTime time.Time, initData []byte, extras []archiveFile) error {
	zw, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("create gzip writer: %w", err)
	}

	archive := initramfs.NewWriter(zw)

	if err := archive.AddDirectory("dev", 0o755, modTime); err != nil {
		return fmt.Errorf("add dev directory: %w", err)
	}
	if err := archive.AddDirectory("proc", 0o755, modTime); err != nil {
		return fmt.Errorf("add proc directory: %w", err)
	}
	if err := archive.AddDirectory("sys", 0o755, modTime); err != nil {
		return fmt.Errorf("add sys directory: %w", err)
	}
	if err := addExtraDirectories(archive, extras); err != nil {
		return err
	}
	if err := archive.AddCharDevice("dev/console", 0o600, modTime, 5, 1); err != nil {
		return fmt.Errorf("add /dev/console: %w", err)
	}
	if err := archive.AddCharDevice("dev/null", 0o666, modTime, 1, 3); err != nil {
		return fmt.Errorf("add /dev/null: %w", err)
	}
	if err := archive.AddFile("init", 0o755, modTime, initData); err != nil {
		return fmt.Errorf("add /init: %w", err)
	}
	for _, extra := range extras {
		if err := archive.AddFile(extra.Path, extra.Mode, extra.ModTime, extra.Data); err != nil {
			return fmt.Errorf("add %s: %w", extra.Path, err)
		}
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("close archive: %w", err)
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("close gzip stream: %w", err)
	}

	return nil
}

func addExtraDirectories(archive *initramfs.Writer, extras []archiveFile) error {
	seen := map[string]struct{}{
		"dev":  {},
		"proc": {},
		"sys":  {},
	}

	for _, extra := range extras {
		for _, dir := range parentDirectories(extra.Path) {
			if _, ok := seen[dir]; ok {
				continue
			}
			if err := archive.AddDirectory(dir, 0o755, extra.ModTime); err != nil {
				return fmt.Errorf("add %s directory: %w", dir, err)
			}
			seen[dir] = struct{}{}
		}
	}

	return nil
}

func parentDirectories(path string) []string {
	clean := filepath.Clean(strings.TrimPrefix(path, "/"))
	if clean == "." || clean == "" {
		return nil
	}

	dir := filepath.Dir(clean)
	if dir == "." {
		return nil
	}

	parts := strings.Split(dir, string(filepath.Separator))
	result := make([]string, 0, len(parts))
	for i := range parts {
		result = append(result, filepath.Join(parts[:i+1]...))
	}
	return result
}
