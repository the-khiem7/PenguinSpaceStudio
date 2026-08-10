package history

import (
	"context"
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

	record := core.HistoryRecord{
		ID:             "run-1",
		ProviderID:     "fixture.cache",
		PlanID:         "fixture-plan",
		ReclaimedBytes: 3_145_728,
		CreatedAt:      time.Now().UTC(),
	}
	if err := store.Append(context.Background(), record); err != nil {
		t.Fatalf("append record: %v", err)
	}

	records, err := store.List(context.Background(), 10)
	if err != nil {
		t.Fatalf("list records: %v", err)
	}
	if len(records) != 1 || records[0].ID != record.ID || records[0].ReclaimedBytes != record.ReclaimedBytes {
		t.Fatalf("unexpected records: %#v", records)
	}
}
