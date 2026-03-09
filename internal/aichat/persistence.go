package aichat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	"github.com/pkg/errors"
)

const defaultConversationID = "qemu-go-init-bbs"

type persistenceResources struct {
	root            string
	conversationID  string
	turnsDBPath     string
	timelineDBPath  string
	turnStore       chatstore.TurnStore
	timelineStore   chatstore.TimelineStore
	timelineVersion atomic.Uint64
}

func openPersistence(root string, conversationID string) (*persistenceResources, error) {
	root = defaultChatStateRoot(root)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create chat persistence root %s: %w", root, err)
	}

	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		conversationID = defaultConversationID
	}

	turnsDBPath := filepath.Join(root, "turns.db")
	turnsDSN, err := chatstore.SQLiteTurnDSNForFile(turnsDBPath)
	if err != nil {
		return nil, fmt.Errorf("build turns dsn: %w", err)
	}
	turnStore, err := chatstore.NewSQLiteTurnStore(turnsDSN)
	if err != nil {
		return nil, fmt.Errorf("open turns store: %w", err)
	}

	timelineDBPath := filepath.Join(root, "timeline.db")
	timelineDSN, err := chatstore.SQLiteTimelineDSNForFile(timelineDBPath)
	if err != nil {
		_ = turnStore.Close()
		return nil, fmt.Errorf("build timeline dsn: %w", err)
	}
	timelineStore, err := chatstore.NewSQLiteTimelineStore(timelineDSN)
	if err != nil {
		_ = turnStore.Close()
		return nil, fmt.Errorf("open timeline store: %w", err)
	}

	return &persistenceResources{
		root:           root,
		conversationID: conversationID,
		turnsDBPath:    turnsDBPath,
		timelineDBPath: timelineDBPath,
		turnStore:      turnStore,
		timelineStore:  timelineStore,
	}, nil
}

func (p *persistenceResources) Close() error {
	if p == nil {
		return nil
	}
	var err error
	if p.turnStore != nil {
		err = p.turnStore.Close()
	}
	if p.timelineStore != nil {
		if closeErr := p.timelineStore.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
}

func defaultChatStateRoot(root string) string {
	trimmed := strings.TrimSpace(root)
	if trimmed == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(trimmed), "chat")
}

type turnStorePersister struct {
	store  chatstore.TurnStore
	convID string
}

func newTurnStorePersister(store chatstore.TurnStore, convID string) *turnStorePersister {
	if store == nil || strings.TrimSpace(convID) == "" {
		return nil
	}
	return &turnStorePersister{store: store, convID: strings.TrimSpace(convID)}
}

func (p *turnStorePersister) PersistTurn(ctx context.Context, t *turns.Turn) error {
	if p == nil || p.store == nil || t == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if p.convID == "" {
		return errors.New("turn persister: convID is empty")
	}

	sessionID, ok, err := turns.KeyTurnMetaSessionID.Get(t.Metadata)
	if err != nil || !ok || strings.TrimSpace(sessionID) == "" {
		return errors.New("turn persister: sessionID is empty")
	}

	turnID := strings.TrimSpace(t.ID)
	if turnID == "" {
		turnID = "turn"
	}

	payload, err := serde.ToYAML(t, serde.Options{})
	if err != nil {
		return errors.Wrap(err, "turn persister: serialize")
	}

	runtimeKey := ""
	if v, ok, err := turns.KeyTurnMetaRuntime.Get(t.Metadata); err == nil && ok {
		runtimeKey = strings.TrimSpace(runtimeKeyFromMetaValue(v))
	}
	inferenceID := ""
	if v, ok, err := turns.KeyTurnMetaInferenceID.Get(t.Metadata); err == nil && ok {
		inferenceID = strings.TrimSpace(v)
	}

	return p.store.Save(ctx, p.convID, strings.TrimSpace(sessionID), turnID, "final", time.Now().UnixMilli(), string(payload), chatstore.TurnSaveOptions{
		RuntimeKey:  runtimeKey,
		InferenceID: inferenceID,
	})
}

func runtimeKeyFromMetaValue(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		for _, key := range []string{"runtime_key", "key", "slug", "profile", "profile_key"} {
			if raw, ok := t[key]; ok {
				if s, ok := raw.(string); ok {
					return s
				}
			}
		}
	}
	return ""
}
