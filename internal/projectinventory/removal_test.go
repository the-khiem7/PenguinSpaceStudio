package projectinventory

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
)

func nodeArtifactRoot(t *testing.T) (root, artifact string) {
	t.Helper()
	root = t.TempDir()
	writeFixture(t, root, nil, []string{"package.json"})
	writeSizedFile(t, root, "dist/out.bin", 4096)
	return root, filepath.Join(root, "dist")
}

func TestRemovalInspectAndExecuteLifecycle(t *testing.T) {
	root, artifact := nodeArtifactRoot(t)
	controller := NewRemovalController(NewSystemInspector())

	plan, err := controller.Inspect(context.Background(), root, root, artifact)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if plan.Method != core.ProjectArtifactRemovalMethodFilesystemFallback {
		t.Fatalf("unexpected method: %q", plan.Method)
	}
	if plan.Risk != core.RiskReview {
		t.Fatalf("unexpected risk: %q", plan.Risk)
	}
	if plan.MeasuredBefore.Kind != core.MeasurementMeasuredLogical || plan.MeasuredBefore.Bytes != 4096 {
		t.Fatalf("unexpected pre-removal measurement: %+v", plan.MeasuredBefore)
	}

	outcome, err := controller.Execute(context.Background(), plan.ID, true)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !outcome.RemovalAttempted || !outcome.RemovalCompleted || !outcome.VerifiedAbsent {
		t.Fatalf("expected a completed and verified outcome: %+v", outcome)
	}
	if outcome.ReclaimedActual.Kind != core.MeasurementMeasuredLogical || outcome.ReclaimedActual.Bytes != 4096 {
		t.Fatalf("unexpected reclaimed bytes: %+v", outcome.ReclaimedActual)
	}
	if _, statErr := os.Lstat(artifact); !os.IsNotExist(statErr) {
		t.Fatalf("artifact must actually be removed from disk: %v", statErr)
	}
}

func TestRemovalExecuteRequiresConfirmation(t *testing.T) {
	root, artifact := nodeArtifactRoot(t)
	controller := NewRemovalController(NewSystemInspector())

	plan, err := controller.Inspect(context.Background(), root, root, artifact)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Execute(context.Background(), plan.ID, false); err == nil {
		t.Fatal("execution without confirmation was accepted")
	}
	if _, statErr := os.Lstat(artifact); statErr != nil {
		t.Fatalf("artifact must remain untouched without confirmation: %v", statErr)
	}
}

func TestRemovalExecuteRejectsUnknownOrConsumedPlan(t *testing.T) {
	root, artifact := nodeArtifactRoot(t)
	controller := NewRemovalController(NewSystemInspector())

	if _, err := controller.Execute(context.Background(), "unknown-plan", true); err == nil {
		t.Fatal("an unknown plan ID was accepted")
	}

	plan, err := controller.Inspect(context.Background(), root, root, artifact)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Execute(context.Background(), plan.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Execute(context.Background(), plan.ID, true); err == nil {
		t.Fatal("a consumed plan ID was accepted a second time")
	}
}

func TestRemovalInspectRejectsUnclaimedOrUnknownArtifact(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, []string{"src"}, []string{"package.json"})
	controller := NewRemovalController(NewSystemInspector())

	if _, err := controller.Inspect(context.Background(), root, root, filepath.Join(root, "src")); err == nil {
		t.Fatal("a non-artifact directory was accepted for removal review")
	}
	if _, err := controller.Inspect(context.Background(), root, root, filepath.Join(root, "dist")); err == nil {
		t.Fatal("an artifact absent from discovery was accepted for removal review")
	}
	if _, err := controller.Inspect(context.Background(), root, filepath.Join(root, "src"), filepath.Join(root, "src")); err == nil {
		t.Fatal("a directory with no marker was accepted as a project")
	}
}

func TestRemovalInspectRejectsUnapprovedRoot(t *testing.T) {
	root, artifact := nodeArtifactRoot(t)
	controller := NewRemovalController(NewSystemInspector())
	if _, err := controller.Inspect(context.Background(), filepath.Join(root, "absent"), root, artifact); err == nil {
		t.Fatal("an unapproved root was accepted")
	}
}

func TestRemovalExecuteRevalidatesArtifactBeforeDeleting(t *testing.T) {
	root, artifact := nodeArtifactRoot(t)
	controller := NewRemovalController(NewSystemInspector())

	plan, err := controller.Inspect(context.Background(), root, root, artifact)
	if err != nil {
		t.Fatal(err)
	}
	// The artifact is removed out-of-band between review and execution, imitating a
	// concurrent change. Execution must fail closed rather than deleting nothing
	// silently or deleting an unrelated path.
	if err := os.RemoveAll(artifact); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Execute(context.Background(), plan.ID, true); err == nil {
		t.Fatal("execution succeeded against an artifact that no longer exists")
	}
}

func TestRemovalExecuteRejectsArtifactBecomeReparsePoint(t *testing.T) {
	root, artifact := nodeArtifactRoot(t)
	outside := t.TempDir()
	if err := os.RemoveAll(artifact); err != nil {
		t.Fatal(err)
	}
	controllerInspector := NewSystemInspector()
	// Discover once while dist is a plain directory so it is claimed as an artifact,
	// then replace it with a symlink before execution.
	if err := os.Mkdir(artifact, 0o700); err != nil {
		t.Fatal(err)
	}
	controller := NewRemovalController(controllerInspector)
	plan, err := controller.Inspect(context.Background(), root, root, artifact)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(artifact); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, artifact); err != nil {
		t.Skipf("symlink unsupported on this host: %v", err)
	}

	if _, err := controller.Execute(context.Background(), plan.ID, true); err == nil {
		t.Fatal("execution against a reparse point was accepted")
	}
	if _, statErr := os.Lstat(outside); statErr != nil {
		t.Fatalf("the link target must never be touched: %v", statErr)
	}
}

func TestRemovalExecuteRejectsDangerRiskArtifact(t *testing.T) {
	root, artifact := nodeArtifactRoot(t)
	controller := NewRemovalController(NewSystemInspector())
	plan, err := controller.Inspect(context.Background(), root, root, artifact)
	if err != nil {
		t.Fatal(err)
	}
	plan.Risk = core.RiskDanger
	controller.issued = plan
	if _, err := controller.Execute(context.Background(), plan.ID, true); err == nil {
		t.Fatal("a Danger-risk plan was accepted for execution")
	}
}

func TestRemovalInspectRejectsIncompleteDiscovery(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, []string{"one", "two"}, []string{"one/package.json", "two/package.json"})
	bounded := NewInspector(Limits{MaxDirectories: 1})
	controller := NewRemovalController(bounded)
	if _, err := controller.Inspect(context.Background(), root, filepath.Join(root, "one"), filepath.Join(root, "one", "dist")); err == nil {
		t.Fatal("an incomplete discovery snapshot was accepted for removal review")
	}
}

func TestRecordArtifactRemovalOutcomeOnlyRecordsVerifiedAbsence(t *testing.T) {
	recorder := &fakeHistoryRecorder{}
	unverified := core.ProjectArtifactRemovalOutcome{PlanID: "p1"}
	result := RecordArtifactRemovalOutcome(context.Background(), recorder, unverified, nowUTC())
	if result.HistoryRecorded || len(recorder.records) != 0 {
		t.Fatalf("an unverified outcome must not be recorded: %+v / %d records", result, len(recorder.records))
	}

	verified := core.ProjectArtifactRemovalOutcome{
		PlanID:          "p2",
		VerifiedAbsent:  true,
		ReclaimedActual: core.Measurement{Bytes: 100, Kind: core.MeasurementMeasuredLogical},
	}
	result = RecordArtifactRemovalOutcome(context.Background(), recorder, verified, nowUTC())
	if !result.HistoryRecorded || len(recorder.records) != 1 {
		t.Fatalf("a verified outcome must be recorded exactly once: %+v / %d records", result, len(recorder.records))
	}
	if recorder.records[0].ProviderID != core.ProjectArtifactRemovalProviderID {
		t.Fatalf("unexpected provider ID: %q", recorder.records[0].ProviderID)
	}
}

type fakeHistoryRecorder struct {
	records  []core.HistoryRecord
	failNext bool
}

func (r *fakeHistoryRecorder) Append(_ context.Context, record core.HistoryRecord) error {
	if r.failNext {
		r.failNext = false
		return os.ErrClosed
	}
	r.records = append(r.records, record)
	return nil
}

func TestRecordArtifactRemovalOutcomeReportsHistoryFailure(t *testing.T) {
	recorder := &fakeHistoryRecorder{failNext: true}
	verified := core.ProjectArtifactRemovalOutcome{PlanID: "p3", VerifiedAbsent: true}
	result := RecordArtifactRemovalOutcome(context.Background(), recorder, verified, nowUTC())
	if result.HistoryRecorded {
		t.Fatal("a failed history append must not report HistoryRecorded")
	}
	if result.Failure == "" {
		t.Fatal("a failed history append must surface the failure")
	}
	if !result.VerifiedAbsent {
		t.Fatal("verified-absent state must be preserved even when history fails")
	}
}
