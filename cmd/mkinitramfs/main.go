package main

import (
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/manuel/wesen/qemu-go-init/internal/initramfs"
)

func main() {
	var (
		initPath = flag.String("init-bin", "", "path to the statically linked /init binary")
		output   = flag.String("output", "", "path to the initramfs.cpio.gz output file")
	)

	flag.Parse()

	if err := run(*initPath, *output); err != nil {
		fmt.Fprintf(os.Stderr, "mkinitramfs: %v\n", err)
		os.Exit(1)
	}
}

func run(initPath string, output string) error {
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

	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	file, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer file.Close()

	if err := writeArchive(file, info.ModTime().UTC(), initData); err != nil {
		return err
	}

	return nil
}

func writeArchive(w io.Writer, modTime time.Time, initData []byte) error {
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
	if err := archive.AddCharDevice("dev/console", 0o600, modTime, 5, 1); err != nil {
		return fmt.Errorf("add /dev/console: %w", err)
	}
	if err := archive.AddCharDevice("dev/null", 0o666, modTime, 1, 3); err != nil {
		return fmt.Errorf("add /dev/null: %w", err)
	}
	if err := archive.AddFile("init", 0o755, modTime, initData); err != nil {
		return fmt.Errorf("add /init: %w", err)
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("close archive: %w", err)
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("close gzip stream: %w", err)
	}

	return nil
}
