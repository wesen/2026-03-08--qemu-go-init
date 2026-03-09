package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/manuel/wesen/qemu-go-init/internal/logstore"
)

func main() {
	var (
		inputPath = flag.String("input", "", "path to the host-side QEMU log file")
		dbPath    = flag.String("db", "", "path to the sqlite database that should receive imported rows")
		source    = flag.String("source", "qemu-host", "source label to store with imported rows")
	)
	flag.Parse()

	if err := run(*inputPath, *dbPath, *source); err != nil {
		fmt.Fprintf(os.Stderr, "importqemulogs: %v\n", err)
		os.Exit(1)
	}
}

func run(inputPath string, dbPath string, source string) error {
	if strings.TrimSpace(inputPath) == "" {
		return fmt.Errorf("-input is required")
	}
	if strings.TrimSpace(dbPath) == "" {
		return fmt.Errorf("-db is required")
	}

	input, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input log %s: %w", inputPath, err)
	}
	defer input.Close()

	store, err := logstore.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open output db %s: %w", dbPath, err)
	}
	defer store.Close()

	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := store.Insert(source, line); err != nil {
			return fmt.Errorf("insert log line: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan input log: %w", err)
	}
	return nil
}
