package core

import (
	"context"
	"testing"
)

type memoryHistory struct {
	records []HistoryRecord
}

func (m *memoryHistory) Append(_ context.Context, record HistoryRecord) error {
	m.records = append(m.records, record)
	return nil
}

func TestFixtureProviderRequiresConfirmation(t *testing.T) {
	provider := NewFixtureProvider()
	detection, err := provider.Detect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	scan, err := provider.Scan(context.Background(), detection)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := provider.Plan(scan)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := provider.Execute(context.Background(), plan, false); err == nil {
		t.Fatal("expected an unconfirmed plan to be rejected")
	}
}

func TestFixtureScenarioPreservesLifecycleAndExactBytes(t *testing.T) {
	history := &memoryHistory{}
	orchestrator := NewOrchestrator(NewFixtureProvider(), history)

	scenario, err := orchestrator.RunFixtureScenario(context.Background())
	if err != nil {
		t.Fatalf("run scenario: %v", err)
	}
	if !scenario.Execution.Executed || scenario.Execution.Destructive {
		t.Fatalf("unexpected execution result: %#v", scenario.Execution)
	}
	if scenario.Verification.ReclaimedActual.Bytes != fixtureBytes {
		t.Fatalf("reclaimed bytes = %d, want %d", scenario.Verification.ReclaimedActual.Bytes, fixtureBytes)
	}
	if scenario.Verification.MeasuredAfter.Bytes != 0 {
		t.Fatalf("bytes after = %d, want 0", scenario.Verification.MeasuredAfter.Bytes)
	}
	if len(history.records) != 1 || history.records[0].ReclaimedBytes != fixtureBytes || history.records[0].ReclaimedKind != MeasurementMeasuredLogical {
		t.Fatalf("history record not persisted: %#v", history.records)
	}
}
