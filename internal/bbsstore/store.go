package bbsstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const databaseName = "bbs.db"

type Message struct {
	ID        int64     `json:"id"`
	Author    string    `json:"author"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

type CreateMessageParams struct {
	Author  string
	Subject string
	Body    string
}

type Store struct {
	root   string
	dbPath string
	db     *sql.DB
}

func Open(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create bbs root %s: %w", root, err)
	}

	dbPath := filepath.Join(root, databaseName)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db %s: %w", dbPath, err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	store := &Store{
		root:   root,
		dbPath: dbPath,
		db:     db,
	}

	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Root() string {
	return s.root
}

func (s *Store) DBPath() string {
	return s.dbPath
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) ListMessages(ctx context.Context) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, author, subject, body, created_at
FROM messages
ORDER BY id DESC
`)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var message Message
		var createdAt string
		if err := rows.Scan(&message.ID, &message.Author, &message.Subject, &message.Body, &createdAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		message.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse created_at %q: %w", createdAt, err)
		}
		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate messages: %w", err)
	}

	return messages, nil
}

func (s *Store) CreateMessage(ctx context.Context, params CreateMessageParams) (Message, error) {
	author := strings.TrimSpace(params.Author)
	subject := strings.TrimSpace(params.Subject)
	body := strings.TrimSpace(params.Body)

	if author == "" {
		author = "anonymous"
	}
	if subject == "" {
		return Message{}, errors.New("subject is required")
	}
	if body == "" {
		return Message{}, errors.New("body is required")
	}

	message := Message{
		Author:    author,
		Subject:   subject,
		Body:      body,
		CreatedAt: time.Now().UTC(),
	}

	result, err := s.db.ExecContext(ctx, `
INSERT INTO messages(author, subject, body, created_at)
VALUES(?, ?, ?, ?)
`, message.Author, message.Subject, message.Body, message.CreatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return Message{}, fmt.Errorf("insert message: %w", err)
	}

	message.ID, err = result.LastInsertId()
	if err != nil {
		return Message{}, fmt.Errorf("read inserted id: %w", err)
	}

	return message, nil
}

func (s *Store) CountMessages(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count messages: %w", err)
	}
	return count, nil
}

func (s *Store) init() error {
	statements := []string{
		`PRAGMA busy_timeout = 5000;`,
		`PRAGMA foreign_keys = ON;`,
		`PRAGMA journal_mode = DELETE;`,
		`PRAGMA synchronous = NORMAL;`,
		`
CREATE TABLE IF NOT EXISTS messages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	author TEXT NOT NULL,
	subject TEXT NOT NULL,
	body TEXT NOT NULL,
	created_at TEXT NOT NULL
);
`,
		`CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at DESC);`,
	}

	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("initialize bbs store: %w", err)
		}
	}

	count, err := s.CountMessages(context.Background())
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	_, err = s.CreateMessage(context.Background(), CreateMessageParams{
		Author:  "system",
		Subject: "Welcome to qemu-go-init BBS",
		Body:    "This is a tiny shared-state BBS. Post from the host CLI or over SSH and the same SQLite database should back both views.",
	})
	return err
}
