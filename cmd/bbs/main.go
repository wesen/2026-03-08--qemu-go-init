package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/manuel/wesen/qemu-go-init/internal/bbsapp"
	"github.com/manuel/wesen/qemu-go-init/internal/bbsstore"
)

func main() {
	var stateRoot string
	flag.StringVar(&stateRoot, "state-root", defaultStateRoot(), "directory containing the shared BBS state")
	flag.Parse()

	logger := log.New(os.Stderr, "bbs: ", log.LstdFlags|log.Lmicroseconds)
	store, err := bbsstore.Open(stateRoot)
	if err != nil {
		logger.Fatalf("open store: %v", err)
	}
	defer store.Close()

	model, err := bbsapp.New(store, bbsapp.Options{
		Title:     "qemu-go-init bbs",
		Subtitle:  "Host-native Bubble Tea board",
		StateRoot: store.Root(),
	})
	if err != nil {
		logger.Fatalf("create bbs app: %v", err)
	}
	defer func() {
		if closeErr := model.Close(); closeErr != nil {
			logger.Printf("close bbs app: %v", closeErr)
		}
	}()

	program := tea.NewProgram(model, tea.WithAltScreen())
	model.AttachProgram(context.Background(), program)
	if _, err := program.Run(); err != nil {
		logger.Fatalf("run bbs: %v", err)
	}
}

func defaultStateRoot() string {
	return filepath.Join("build", "shared-state", "bbs")
}
