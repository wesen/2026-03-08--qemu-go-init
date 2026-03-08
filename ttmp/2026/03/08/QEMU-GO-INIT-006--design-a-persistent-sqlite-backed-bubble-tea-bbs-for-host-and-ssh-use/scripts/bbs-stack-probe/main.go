package main

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	_ "modernc.org/sqlite"
)

type probeModel struct{}

func (probeModel) Init() tea.Cmd { return nil }

func (probeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return probeModel{}, tea.Quit
}

func (probeModel) View() string { return "bbs stack probe" }

func main() {
	logger := log.New(os.Stdout, "", 0)

	tempDir, err := os.MkdirTemp("", "bbs-stack-probe-*")
	if err != nil {
		logger.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "probe.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		logger.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			author TEXT NOT NULL,
			body TEXT NOT NULL
		);
		INSERT INTO messages (author, body) VALUES ('probe', 'hello from sqlite');
	`); err != nil {
		logger.Fatalf("seed sqlite: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&count); err != nil {
		logger.Fatalf("count rows: %v", err)
	}

	renderer := lipgloss.NewRenderer(io.Discard)
	program := tea.NewProgram(probeModel{}, tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer())

	fmt.Printf("sqlite rows=%d\n", count)
	fmt.Printf("lipgloss renderer=%T\n", renderer)
	fmt.Printf("bubbletea program=%T\n", program)
	fmt.Println("probe succeeded")
}
