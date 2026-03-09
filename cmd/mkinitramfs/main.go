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

type archiveFile struct {
	Path    string
	Data    []byte
	ModTime time.Time
	Mode    fs.FileMode
}

func main() {
	var (
		initPath = flag.String("init-bin", "", "path to the /init binary")
		output   = flag.String("output", "", "path to the initramfs.cpio.gz output file")
		modules  moduleFlags
		files    fileFlags
		fileSets fileSetFlags
	)
	flag.Var(&modules, "module-map", "optional guestPath=hostPath mapping for a kernel module to include in the initramfs")
	flag.Var(&files, "file-map", "optional guestPath=hostPath mapping for a regular file to include in the initramfs")
	flag.Var(&fileSets, "file-map-file", "optional path to a file containing guestPath=hostPath mappings for regular files to include in the initramfs")

	flag.Parse()

	if err := run(*initPath, *output, modules, files, fileSets); err != nil {
		fmt.Fprintf(os.Stderr, "mkinitramfs: %v\n", err)
		os.Exit(1)
	}
}

type moduleFlags []string
type fileFlags []string
type fileSetFlags []string

func (m *moduleFlags) String() string {
	return strings.Join(*m, ",")
}

func (m *moduleFlags) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func (f *fileFlags) String() string {
	return strings.Join(*f, ",")
}

func (f *fileFlags) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func (f *fileSetFlags) String() string {
	return strings.Join(*f, ",")
}

func (f *fileSetFlags) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func run(initPath string, output string, modules moduleFlags, files fileFlags, fileSets fileSetFlags) error {
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

	fileSpecs, err := expandFileSpecs(files, fileSets)
	if err != nil {
		return err
	}

	extras, err := readExtraFiles(modules, fileSpecs)
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

func readExtraFiles(moduleSpecs []string, fileSpecs []string) ([]archiveFile, error) {
	if len(moduleSpecs) == 0 && len(fileSpecs) == 0 {
		return nil, nil
	}

	files := make([]archiveFile, 0, len(moduleSpecs)+len(fileSpecs))
	for _, spec := range moduleSpecs {
		guestPath, hostPath, err := parseModuleSpec(spec)
		if err != nil {
			return nil, err
		}

		data, err := readModuleData(hostPath)
		if err != nil {
			return nil, err
		}

		info, err := os.Stat(hostPath)
		if err != nil {
			return nil, fmt.Errorf("stat module %s: %w", hostPath, err)
		}

		files = append(files, archiveFile{
			Path:    strings.TrimPrefix(guestPath, "/"),
			Data:    data,
			ModTime: info.ModTime().UTC(),
			Mode:    0o644,
		})
	}

	for _, spec := range fileSpecs {
		guestPath, hostPath, err := parseModuleSpec(spec)
		if err != nil {
			return nil, err
		}

		data, err := os.ReadFile(hostPath)
		if err != nil {
			return nil, fmt.Errorf("read file %s: %w", hostPath, err)
		}

		info, err := os.Stat(hostPath)
		if err != nil {
			return nil, fmt.Errorf("stat file %s: %w", hostPath, err)
		}

		files = append(files, archiveFile{
			Path:    strings.TrimPrefix(guestPath, "/"),
			Data:    data,
			ModTime: info.ModTime().UTC(),
			Mode:    info.Mode().Perm(),
		})
	}
	return files, nil
}

func expandFileSpecs(inline []string, fileSets []string) ([]string, error) {
	specs := append([]string{}, inline...)
	for _, path := range fileSets {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read file map file %s: %w", path, err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			specs = append(specs, trimmed)
		}
	}
	return specs, nil
}

func parseModuleSpec(spec string) (string, string, error) {
	parts := strings.SplitN(spec, "=", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid -module-map %q, expected guestPath=hostPath", spec)
	}
	return parts[0], parts[1], nil
}

func readModuleData(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read module %s: %w", path, err)
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
		return nil, fmt.Errorf("decompress module %s: %w", path, err)
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
