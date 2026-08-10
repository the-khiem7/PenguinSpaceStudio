package history

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"math"
	"time"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	store := &Store{db: db}
	if err := store.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS cleanup_history (
			id TEXT PRIMARY KEY,
			provider_id TEXT NOT NULL,
			plan_id TEXT NOT NULL,
			reclaimed_bytes INTEGER NOT NULL CHECK (reclaimed_bytes >= 0),
			created_at TEXT NOT NULL
		);
		PRAGMA user_version = 1;
	`)
	if err != nil {
		return fmt.Errorf("migrate cleanup history: %w", err)
	}
	return nil
}

func (s *Store) Append(ctx context.Context, record core.HistoryRecord) error {
	if record.ReclaimedBytes > math.MaxInt64 {
		return fmt.Errorf("reclaimed bytes exceed SQLite signed integer range: %d", record.ReclaimedBytes)
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO cleanup_history (id, provider_id, plan_id, reclaimed_bytes, created_at) VALUES (?, ?, ?, ?, ?)`,
		record.ID,
		record.ProviderID,
		record.PlanID,
		record.ReclaimedBytes,
		record.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("append cleanup history: %w", err)
	}
	return nil
}

func (s *Store) List(ctx context.Context, limit int) ([]core.HistoryRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, provider_id, plan_id, reclaimed_bytes, created_at FROM cleanup_history ORDER BY created_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list cleanup history: %w", err)
	}
	defer rows.Close()

	var records []core.HistoryRecord
	for rows.Next() {
		var record core.HistoryRecord
		var createdAt string
		if err := rows.Scan(&record.ID, &record.ProviderID, &record.PlanID, &record.ReclaimedBytes, &createdAt); err != nil {
			return nil, fmt.Errorf("scan cleanup history: %w", err)
		}
		parsed, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse cleanup history timestamp: %w", err)
		}
		record.CreatedAt = parsed
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) Close() error {
	return s.db.Close()
}
