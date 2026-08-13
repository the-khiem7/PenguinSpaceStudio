package dockerinventory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
)

const (
	testNetworkListID = "abc123def456"
	testNetworkID     = "abc123def456abc123def456abc123def456abc123def456abc123def456abcd"
)

type networkRemovalRunner struct {
	present                       bool
	attached                      bool
	failVolumeList                bool
	omitNetworkInspect            bool
	omitContainers                bool
	preserveOnRemove              bool
	removeError                   bool
	daemonUnavailableAfterRemoval bool
	project                       string
	networkLabel                  string
	calls                         [][]string
}

func newNetworkRemovalRunner() *networkRemovalRunner {
	return &networkRemovalRunner{
		present:      true,
		project:      "sample",
		networkLabel: "default",
	}
}

func (r *networkRemovalRunner) LookPath(string) (string, error) {
	return "docker", nil
}

func (r *networkRemovalRunner) Run(_ context.Context, _ string, args ...string) (string, error) {
	r.calls = append(r.calls, append([]string(nil), args...))
	command := strings.Join(args, " ")
	switch {
	case command == "version --format {{json .Server}}":
		if r.daemonUnavailableAfterRemoval && !r.present {
			return "", errors.New("daemon unavailable")
		}
		return `{"Version":"29.5.3","Os":"linux","Arch":"amd64"}`, nil
	case command == "system df --format json", command == "builder du --format json":
		return "", nil
	case command == "image ls --all --format json", command == "image ls --all --no-trunc --format json":
		return "", nil
	case strings.HasPrefix(command, "container ls "):
		return "", nil
	case command == "volume ls --format json":
		if r.failVolumeList {
			return "", errors.New("volume listing failed")
		}
		return "", nil
	case command == "network ls --filter type=custom --format json":
		if !r.present {
			return "", nil
		}
		return fmt.Sprintf(`{"ID":%q}`, testNetworkListID), nil
	case command == "network ls --no-trunc --filter type=custom --format json":
		if !r.present {
			return "", nil
		}
		return fmt.Sprintf(`{"ID":%q}`, testNetworkID), nil
	case strings.HasPrefix(command, "network inspect --format {{json .}} "):
		if !r.present {
			return "", errors.New("network not found")
		}
		if r.omitNetworkInspect {
			return "", nil
		}
		containers := `{}`
		if r.attached {
			containers = `{"container-1":{}}`
		}
		if r.omitContainers {
			return fmt.Sprintf(`{"Id":%q,"Name":"sample_default","Labels":{"com.docker.compose.project":%q,"com.docker.compose.network":%q}}`,
				testNetworkID, r.project, r.networkLabel), nil
		}
		return fmt.Sprintf(`{"Id":%q,"Name":"sample_default","Labels":{"com.docker.compose.project":%q,"com.docker.compose.network":%q},"Containers":%s}`,
			testNetworkID, r.project, r.networkLabel, containers), nil
	case command == "network rm "+testNetworkID:
		if !r.preserveOnRemove {
			r.present = false
		}
		if r.removeError {
			return "", errors.New("Docker CLI timed out after dispatch")
		}
		return testNetworkID, nil
	case command == "network ls --no-trunc --filter id="+testNetworkID+" --format json":
		if r.daemonUnavailableAfterRemoval && !r.present {
			return "", errors.New("daemon unavailable")
		}
		if r.present {
			return fmt.Sprintf(`{"ID":%q}`, testNetworkID), nil
		}
		return "", nil
	default:
		return "", fmt.Errorf("unexpected Docker command: %s", command)
	}
}

func TestNetworkRemovalLifecycleUsesRetainedExactID(t *testing.T) {
	runner := newNetworkRemovalRunner()
	controller := NewNetworkRemovalController(NewInspector(runner))

	plan, err := controller.Inspect(context.Background(), testNetworkID)
	if err != nil {
		t.Fatalf("inspect removal: %v", err)
	}
	if plan.NetworkID != testNetworkID || plan.Project != "sample" || plan.NetworkLabel != "default" || plan.Risk != core.RiskReview {
		t.Fatalf("unexpected plan: %#v", plan)
	}

	if _, err := controller.Execute(context.Background(), "wrong-plan", true); err == nil {
		t.Fatal("expected an unknown plan ID to be rejected")
	}
	if _, err := controller.Execute(context.Background(), plan.ID, false); err == nil {
		t.Fatal("expected explicit confirmation to be required")
	}

	outcome, err := controller.Execute(context.Background(), plan.ID, true)
	if err != nil {
		t.Fatalf("execute removal: %v", err)
	}
	if !outcome.RemovalCommandAttempted || !outcome.RemovalCommandCompleted || !outcome.VerifiedAbsent || !outcome.AwarenessRefreshed || outcome.NetworkID != testNetworkID {
		t.Fatalf("unexpected outcome: %#v", outcome)
	}
	if outcome.ReclaimedActual.Kind != core.MeasurementUnavailable || outcome.ReclaimedActual.Bytes != 0 {
		t.Fatalf("reclaimed measurement must be unavailable: %#v", outcome.ReclaimedActual)
	}
	if runner.present {
		t.Fatal("network still present after verified removal")
	}
	assertOnlyExactRemoval(t, runner.calls)

	if _, err := controller.Execute(context.Background(), plan.ID, true); err == nil {
		t.Fatal("expected the consumed plan to require a new inspection")
	}
}

func TestNetworkRemovalRechecksAttachmentsBeforeCommand(t *testing.T) {
	runner := newNetworkRemovalRunner()
	controller := NewNetworkRemovalController(NewInspector(runner))
	plan, err := controller.Inspect(context.Background(), testNetworkID)
	if err != nil {
		t.Fatalf("inspect removal: %v", err)
	}

	runner.attached = true
	if _, err := controller.Execute(context.Background(), plan.ID, true); err == nil || !strings.Contains(err.Error(), "gained container attachments") {
		t.Fatalf("expected changed attachments to block removal, got %v", err)
	}
	if hasCommand(runner.calls, "network rm "+testNetworkID) {
		t.Fatal("network removal ran after an attachment appeared")
	}
}

func TestNetworkRemovalRejectsMissingAttachmentMetadataBeforeCommand(t *testing.T) {
	runner := newNetworkRemovalRunner()
	controller := NewNetworkRemovalController(NewInspector(runner))
	plan, err := controller.Inspect(context.Background(), testNetworkID)
	if err != nil {
		t.Fatalf("inspect removal: %v", err)
	}

	runner.omitContainers = true
	if _, err := controller.Execute(context.Background(), plan.ID, true); err == nil || !strings.Contains(err.Error(), "omitted network attachment metadata") {
		t.Fatalf("expected missing attachment metadata to block removal, got %v", err)
	}
	if hasCommand(runner.calls, "network rm "+testNetworkID) {
		t.Fatal("network removal ran without attachment metadata")
	}
}

func TestNetworkRemovalRequiresCompleteOwnershipAndCanonicalLabels(t *testing.T) {
	t.Run("incomplete ownership", func(t *testing.T) {
		runner := newNetworkRemovalRunner()
		runner.failVolumeList = true
		controller := NewNetworkRemovalController(NewInspector(runner))
		if _, err := controller.Inspect(context.Background(), testNetworkID); err == nil || !strings.Contains(err.Error(), "incomplete") {
			t.Fatalf("expected incomplete ownership to block a plan, got %v", err)
		}
	})

	t.Run("missing inspect identity", func(t *testing.T) {
		runner := newNetworkRemovalRunner()
		runner.omitNetworkInspect = true
		controller := NewNetworkRemovalController(NewInspector(runner))
		if _, err := controller.Inspect(context.Background(), testNetworkID); err == nil || !strings.Contains(err.Error(), "incomplete") {
			t.Fatalf("expected a missing inspect row to block a plan, got %v", err)
		}
	})

	t.Run("missing attachment metadata", func(t *testing.T) {
		runner := newNetworkRemovalRunner()
		runner.omitContainers = true
		controller := NewNetworkRemovalController(NewInspector(runner))
		if _, err := controller.Inspect(context.Background(), testNetworkID); err == nil || !strings.Contains(err.Error(), "incomplete") {
			t.Fatalf("expected missing attachment metadata to block a plan, got %v", err)
		}
	})

	t.Run("missing network label", func(t *testing.T) {
		runner := newNetworkRemovalRunner()
		runner.networkLabel = ""
		controller := NewNetworkRemovalController(NewInspector(runner))
		if _, err := controller.Inspect(context.Background(), testNetworkID); err == nil || !strings.Contains(err.Error(), "valid canonical") {
			t.Fatalf("expected missing canonical network label to block a plan, got %v", err)
		}
	})
}

func TestNetworkRemovalFailsWhenExactIDRemains(t *testing.T) {
	runner := newNetworkRemovalRunner()
	runner.preserveOnRemove = true
	controller := NewNetworkRemovalController(NewInspector(runner))
	plan, err := controller.Inspect(context.Background(), testNetworkID)
	if err != nil {
		t.Fatalf("inspect removal: %v", err)
	}
	outcome, err := controller.Execute(context.Background(), plan.ID, true)
	if err != nil {
		t.Fatalf("post-command verification failure should be structured, got %v", err)
	}
	if !outcome.RemovalCommandCompleted || outcome.VerifiedAbsent || !strings.Contains(outcome.Failure, "still present") {
		t.Fatalf("expected preserved partial outcome, got %#v", outcome)
	}
	assertOnlyExactRemoval(t, runner.calls)
}

func TestNetworkRemovalReconcilesMutateThenErrorWhenDaemonRemainsAvailable(t *testing.T) {
	runner := newNetworkRemovalRunner()
	runner.removeError = true
	controller := NewNetworkRemovalController(NewInspector(runner))
	plan, err := controller.Inspect(context.Background(), testNetworkID)
	if err != nil {
		t.Fatalf("inspect removal: %v", err)
	}

	outcome, err := controller.Execute(context.Background(), plan.ID, true)
	if err != nil {
		t.Fatalf("mutate-then-error should return a structured outcome, got %v", err)
	}
	if !outcome.RemovalCommandAttempted || outcome.RemovalCommandCompleted || !outcome.VerifiedAbsent || !outcome.AwarenessRefreshed {
		t.Fatalf("exact-ID reconciliation did not resolve the outcome: %#v", outcome)
	}
	if !outcome.Awareness.Daemon.Available || !strings.Contains(outcome.Failure, "timed out") {
		t.Fatalf("CLI warning or refreshed daemon awareness was lost: %#v", outcome)
	}
	assertOnlyExactRemoval(t, runner.calls)
}

func TestNetworkRemovalReconcilesMutateThenErrorWithUnavailableDaemon(t *testing.T) {
	runner := newNetworkRemovalRunner()
	runner.removeError = true
	runner.daemonUnavailableAfterRemoval = true
	controller := NewNetworkRemovalController(NewInspector(runner))
	plan, err := controller.Inspect(context.Background(), testNetworkID)
	if err != nil {
		t.Fatalf("inspect removal: %v", err)
	}

	outcome, err := controller.Execute(context.Background(), plan.ID, true)
	if err != nil {
		t.Fatalf("mutate-then-error should return a structured outcome, got %v", err)
	}
	if !outcome.RemovalCommandAttempted || outcome.RemovalCommandCompleted || !outcome.AwarenessRefreshed {
		t.Fatalf("unexpected uncertain outcome: %#v", outcome)
	}
	if outcome.Awareness.Daemon.Available || !strings.Contains(outcome.Failure, "timed out") {
		t.Fatalf("latest unavailable awareness or command failure was lost: %#v", outcome)
	}
	if runner.present {
		t.Fatal("fixture should model daemon-side removal before the CLI error")
	}
	assertOnlyExactRemoval(t, runner.calls)
}

type recordingHistory struct {
	err     error
	records []core.HistoryRecord
}

func (h *recordingHistory) Append(_ context.Context, record core.HistoryRecord) error {
	if h.err != nil {
		return h.err
	}
	h.records = append(h.records, record)
	return nil
}

func TestRecordNetworkRemovalOutcomePreservesHistoryFailure(t *testing.T) {
	original := core.DockerNetworkRemovalOutcome{
		PlanID:                  "plan-1",
		RemovalCommandAttempted: true,
		RemovalCommandCompleted: true,
		VerifiedAbsent:          true,
		AwarenessRefreshed:      true,
		ReclaimedActual:         core.Measurement{Kind: core.MeasurementUnavailable},
		Message:                 "verified",
		Awareness:               core.DockerAwareness{InspectedAt: time.Unix(10, 0)},
	}
	history := &recordingHistory{err: errors.New("database is read-only")}
	outcome := RecordNetworkRemovalOutcome(context.Background(), history, original, time.Unix(20, 0))
	if outcome.HistoryRecorded || !strings.Contains(outcome.Failure, "read-only") || !outcome.VerifiedAbsent || !outcome.AwarenessRefreshed {
		t.Fatalf("history failure discarded known outcome state: %#v", outcome)
	}

	history = &recordingHistory{}
	outcome = RecordNetworkRemovalOutcome(context.Background(), history, original, time.Unix(20, 0))
	if !outcome.HistoryRecorded || len(history.records) != 1 || history.records[0].ReclaimedKind != core.MeasurementUnavailable {
		t.Fatalf("verified unavailable reclaim outcome was not recorded: %#v %#v", outcome, history.records)
	}
}

func assertOnlyExactRemoval(t *testing.T, calls [][]string) {
	t.Helper()
	removals := 0
	for _, args := range calls {
		command := strings.Join(args, " ")
		if strings.Contains(command, "prune") || strings.Contains(command, "--force") || strings.HasPrefix(command, "system prune") {
			t.Fatalf("prohibited Docker command observed: %s", command)
		}
		if strings.HasPrefix(command, "network rm ") {
			removals++
			if command != "network rm "+testNetworkID {
				t.Fatalf("network removal was not exact-ID only: %s", command)
			}
		}
	}
	if removals != 1 {
		t.Fatalf("network removal command count = %d, want 1", removals)
	}
}

func hasCommand(calls [][]string, expected string) bool {
	for _, args := range calls {
		if strings.Join(args, " ") == expected {
			return true
		}
	}
	return false
}
