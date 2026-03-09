package logstore

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	path      string
	db        *sql.DB
	insertMu  sync.Mutex
	lastError string
}

type Snapshot struct {
	Path      string `json:"path"`
	Exists    bool   `json:"exists"`
	Rows      int64  `json:"rows"`
	LastError string `json:"lastError,omitempty"`
}

func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("log store path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create log db dir: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", path)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open log store: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &Store{path: path, db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Writer(source string) io.Writer {
	return &writer{store: s, source: strings.TrimSpace(source)}
}

func (s *Store) Snapshot(ctx context.Context) (Snapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	snapshot := Snapshot{
		Path:      s.path,
		LastError: s.lastError,
	}
	if info, err := os.Stat(s.path); err == nil && !info.IsDir() {
		snapshot.Exists = true
	}
	if s == nil || s.db == nil {
		return snapshot, nil
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM logs`).Scan(&snapshot.Rows); err != nil {
		return snapshot, fmt.Errorf("count log rows: %w", err)
	}
	return snapshot, nil
}

func (s *Store) Insert(source string, raw string) error {
	if s == nil || s.db == nil {
		return nil
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	entry := parseLine(strings.TrimSpace(source), raw)

	s.insertMu.Lock()
	defer s.insertMu.Unlock()

	_, err := s.db.Exec(`
INSERT INTO logs(ts_rfc3339, source, level, component, message, payload_json)
VALUES(?, ?, ?, ?, ?, ?)
`, entry.Timestamp, entry.Source, entry.Level, entry.Component, entry.Message, entry.PayloadJSON)
	if err != nil {
		s.lastError = err.Error()
		return err
	}
	s.lastError = ""
	return nil
}

func (s *Store) init() error {
	if s == nil || s.db == nil {
		return nil
	}
	statements := []string{
		`PRAGMA busy_timeout = 5000;`,
		`PRAGMA foreign_keys = ON;`,
		`PRAGMA journal_mode = WAL;`,
		`
CREATE TABLE IF NOT EXISTS logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	ts_rfc3339 TEXT NOT NULL,
	source TEXT NOT NULL,
	level TEXT NOT NULL,
	component TEXT NOT NULL DEFAULT '',
	message TEXT NOT NULL,
	payload_json TEXT NOT NULL DEFAULT '{}'
);
`,
		`CREATE INDEX IF NOT EXISTS idx_logs_ts ON logs(id DESC);`,
	}
	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("initialize log store: %w", err)
		}
	}
	return nil
}

type writer struct {
	store  *Store
	source string
	mu     sync.Mutex
	buf    bytes.Buffer
}

func (w *writer) Write(p []byte) (int, error) {
	if w == nil || w.store == nil {
		return len(p), nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	_, _ = w.buf.Write(p)
	for {
		data := w.buf.Bytes()
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			break
		}
		line := string(bytes.TrimSpace(data[:idx]))
		w.buf.Next(idx + 1)
		_ = w.store.Insert(w.source, line)
	}
	return len(p), nil
}

type entry struct {
	Timestamp   string
	Source      string
	Level       string
	Component   string
	Message     string
	PayloadJSON string
}

func parseLine(source string, raw string) entry {
	if source == "" {
		source = "guest"
	}
	result := entry{
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		Source:      source,
		Level:       "info",
		Component:   "",
		Message:     raw,
		PayloadJSON: `{"raw":""}`,
	}

	payload := map[string]any{"raw": raw}
	if decoded := parseJSON(raw); decoded != nil {
		payload = decoded
		if level, ok := decoded["level"].(string); ok && strings.TrimSpace(level) != "" {
			result.Level = strings.TrimSpace(level)
		}
		if component, ok := decoded["component"].(string); ok && strings.TrimSpace(component) != "" {
			result.Component = strings.TrimSpace(component)
		}
		if message, ok := decoded["message"].(string); ok && strings.TrimSpace(message) != "" {
			result.Message = strings.TrimSpace(message)
		}
		if timestamp, ok := decoded["time"].(string); ok && strings.TrimSpace(timestamp) != "" {
			result.Timestamp = strings.TrimSpace(timestamp)
		}
	}
	if marshaled, err := json.Marshal(payload); err == nil {
		result.PayloadJSON = string(marshaled)
	}
	return result
}

func parseJSON(raw string) map[string]any {
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	return payload
}
