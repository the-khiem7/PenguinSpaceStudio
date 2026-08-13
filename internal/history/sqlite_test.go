package history

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
)

func TestStoreMigratesAndPersistsHistory(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "penguinspace.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	recordsToWrite := []core.HistoryRecord{
		{
			ID:             "run-measured",
			ProviderID:     "fixture.cache",
			PlanID:         "fixture-plan",
			ReclaimedBytes: 3_145_728,
			ReclaimedKind:  core.MeasurementMeasuredLogical,
			CreatedAt:      time.Now().UTC().Add(-time.Second),
		},
		{
			ID:            "run-unavailable",
			ProviderID:    core.DockerNetworkRemovalProviderID,
			PlanID:        "network-plan",
			ReclaimedKind: core.MeasurementUnavailable,
			CreatedAt:     time.Now().UTC(),
		},
	}
	for _, record := range recordsToWrite {
		if err := store.Append(context.Background(), record); err != nil {
			t.Fatalf("append record: %v", err)
		}
	}

	records, err := store.List(context.Background(), 10)
	if err != nil {
		t.Fatalf("list records: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("record count = %d, want 2", len(records))
	}
	if records[0].ID != "run-unavailable" || records[0].ReclaimedBytes != 0 || records[0].ReclaimedKind != core.MeasurementUnavailable {
		t.Fatalf("unexpected unavailable record: %#v", records[0])
	}
	if records[1].ID != "run-measured" || records[1].ReclaimedBytes != 3_145_728 || records[1].ReclaimedKind != core.MeasurementMeasuredLogical {
		t.Fatalf("unexpected measured record: %#v", records[1])
	}
}

func TestStoreMigratesVersionOneHistoryKind(t *testing.T) {
	path := filepath.Join(t.TempDir(), "penguinspace.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open v1 database: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE cleanup_history (
			id TEXT PRIMARY KEY,
			provider_id TEXT NOT NULL,
			plan_id TEXT NOT NULL,
			reclaimed_bytes INTEGER NOT NULL CHECK (reclaimed_bytes >= 0),
			created_at TEXT NOT NULL
		);
		INSERT INTO cleanup_history (id, provider_id, plan_id, reclaimed_bytes, created_at)
		VALUES ('legacy', 'fixture.cache', 'legacy-plan', 4096, '2026-08-13T00:00:00Z');
		PRAGMA user_version = 1;
	`)
	if err != nil {
		_ = db.Close()
		t.Fatalf("create v1 database: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close v1 database: %v", err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatalf("migrate v1 store: %v", err)
	}
	defer store.Close()

	records, err := store.List(context.Background(), 10)
	if err != nil {
		t.Fatalf("list migrated records: %v", err)
	}
	if len(records) != 1 || records[0].ReclaimedKind != core.MeasurementMeasuredLogical || records[0].ReclaimedBytes != 4096 {
		t.Fatalf("unexpected migrated record: %#v", records)
	}
}

func TestStoreRejectsFutureSchemaWithoutDowngrade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "penguinspace.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open future database: %v", err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 99`); err != nil {
		_ = db.Close()
		t.Fatalf("set future version: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close future database: %v", err)
	}

	if store, err := Open(path); err == nil {
		_ = store.Close()
		t.Fatal("expected future schema version to be rejected")
	}

	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("reopen future database: %v", err)
	}
	defer db.Close()
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("read future version: %v", err)
	}
	if version != 99 {
		t.Fatalf("schema version changed to %d, want 99", version)
	}
}
