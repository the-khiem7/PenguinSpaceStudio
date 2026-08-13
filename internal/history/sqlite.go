package history

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	_ "modernc.org/sqlite"
)

const schemaVersion = 2

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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cleanup history migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var currentVersion int
	if err := tx.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&currentVersion); err != nil {
		return fmt.Errorf("read cleanup history schema version: %w", err)
	}
	if currentVersion > schemaVersion {
		return fmt.Errorf("cleanup history schema version %d is newer than supported version %d", currentVersion, schemaVersion)
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS cleanup_history (
			id TEXT PRIMARY KEY,
			provider_id TEXT NOT NULL,
			plan_id TEXT NOT NULL,
			reclaimed_bytes INTEGER NOT NULL CHECK (reclaimed_bytes >= 0),
			reclaimed_kind TEXT NOT NULL DEFAULT 'measured-logical',
			created_at TEXT NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("create cleanup history schema: %w", err)
	}

	hasKind, err := cleanupHistoryHasColumn(ctx, tx, "reclaimed_kind")
	if err != nil {
		return err
	}
	if !hasKind {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE cleanup_history ADD COLUMN reclaimed_kind TEXT NOT NULL DEFAULT 'measured-logical'`); err != nil {
			return fmt.Errorf("add cleanup history measurement kind: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
		return fmt.Errorf("set cleanup history schema version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit cleanup history migration: %w", err)
	}
	return nil
}

func cleanupHistoryHasColumn(ctx context.Context, tx *sql.Tx, column string) (bool, error) {
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(cleanup_history)`)
	if err != nil {
		return false, fmt.Errorf("inspect cleanup history schema: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, fmt.Errorf("scan cleanup history schema: %w", err)
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("read cleanup history schema: %w", err)
	}
	return false, nil
}

func (s *Store) Append(ctx context.Context, record core.HistoryRecord) error {
	if record.ReclaimedBytes > math.MaxInt64 {
		return fmt.Errorf("reclaimed bytes exceed SQLite signed integer range: %d", record.ReclaimedBytes)
	}
	kind := record.ReclaimedKind
	if kind == "" {
		kind = core.MeasurementMeasuredLogical
	}
	if !validMeasurementKind(kind) {
		return fmt.Errorf("unsupported reclaimed measurement kind %q", kind)
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO cleanup_history (id, provider_id, plan_id, reclaimed_bytes, reclaimed_kind, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		record.ID,
		record.ProviderID,
		record.PlanID,
		record.ReclaimedBytes,
		kind,
		record.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("append cleanup history: %w", err)
	}
	return nil
}

func (s *Store) List(ctx context.Context, limit int) ([]core.HistoryRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, provider_id, plan_id, reclaimed_bytes, reclaimed_kind, created_at FROM cleanup_history ORDER BY created_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list cleanup history: %w", err)
	}
	defer rows.Close()

	var records []core.HistoryRecord
	for rows.Next() {
		var record core.HistoryRecord
		var reclaimedKind, createdAt string
		if err := rows.Scan(&record.ID, &record.ProviderID, &record.PlanID, &record.ReclaimedBytes, &reclaimedKind, &createdAt); err != nil {
			return nil, fmt.Errorf("scan cleanup history: %w", err)
		}
		record.ReclaimedKind = core.MeasurementKind(reclaimedKind)
		if !validMeasurementKind(record.ReclaimedKind) {
			return nil, fmt.Errorf("cleanup history contains unsupported reclaimed measurement kind %q", reclaimedKind)
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

func validMeasurementKind(kind core.MeasurementKind) bool {
	switch kind {
	case core.MeasurementMeasuredLogical, core.MeasurementEstimatedLogical, core.MeasurementMeasuredPhysical, core.MeasurementUnavailable:
		return true
	default:
		return false
	}
}

func (s *Store) Close() error {
	return s.db.Close()
}
