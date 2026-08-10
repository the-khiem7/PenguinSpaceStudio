package elevation

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRequestRejectsUnknownActionAndExpiredRequest(t *testing.T) {
	request, err := NewRequest(time.Now().UTC(), time.Second, ProbeModeConsent)
	if err != nil {
		t.Fatal(err)
	}
	request.ActionID = "powershell -Command Remove-Item"
	if err := request.Validate(time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "allow-listed") {
		t.Fatalf("expected allow-list rejection, got %v", err)
	}

	request.ActionID = ActionM1ElevationProbe
	request.ProbeMode = ProbeMode("cleanup")
	if err := request.Validate(time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "mode") {
		t.Fatalf("expected mode rejection, got %v", err)
	}

	request.ProbeMode = ProbeModeConsent
	request.ExpiresAt = time.Now().UTC().Add(-time.Second)
	if err := request.Validate(time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expiry rejection, got %v", err)
	}
}

func TestRequestUsesFixedSafeProbeProfiles(t *testing.T) {
	now := time.Now().UTC()
	cancellation, err := NewRequest(now, 2*time.Second, ProbeModeCancellation)
	if err != nil {
		t.Fatal(err)
	}
	if cancellation.ProbeDelayMillis != 1000 {
		t.Fatalf("cancellation delay = %dms, want 1000ms", cancellation.ProbeDelayMillis)
	}

	timeout, err := NewRequest(now, 2*time.Second, ProbeModeTimeout)
	if err != nil {
		t.Fatal(err)
	}
	if timeout.ProbeDelayMillis != 2000 {
		t.Fatalf("timeout delay = %dms, want 2000ms", timeout.ProbeDelayMillis)
	}
}

func TestControllerCompletesSafeProbe(t *testing.T) {
	store := NewStore(t.TempDir())
	controller := NewController(store, LauncherFunc(func(id string) error {
		go func() { _, _ = store.RunM1Probe(context.Background(), id) }()
		return nil
	}), time.Second)

	if _, err := controller.StartM1Probe(ProbeModeConsent); err != nil {
		t.Fatal(err)
	}
	status := waitForTerminal(t, controller)
	if status.State != StateSucceeded {
		t.Fatalf("got %s: %s", status.State, status.Message)
	}
}

func TestControllerCancellationStopsProbe(t *testing.T) {
	store := NewStore(t.TempDir())
	controller := NewController(store, LauncherFunc(func(id string) error {
		go func() { _, _ = store.RunM1Probe(context.Background(), id) }()
		return nil
	}), time.Second)

	if _, err := controller.StartM1Probe(ProbeModeCancellation); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Cancel(); err != nil {
		t.Fatal(err)
	}
	status := waitForTerminal(t, controller)
	if status.State != StateCancelled {
		t.Fatalf("got %s: %s", status.State, status.Message)
	}
}

func TestControllerTimeoutDoesNotStartCleanup(t *testing.T) {
	store := NewStore(t.TempDir())
	controller := NewController(store, LauncherFunc(func(string) error { return nil }), 20*time.Millisecond)

	if _, err := controller.StartM1Probe(ProbeModeTimeout); err != nil {
		t.Fatal(err)
	}
	status := waitForTerminal(t, controller)
	if status.State != StateTimedOut || !strings.Contains(status.Message, "no cleanup command") {
		t.Fatalf("got %s: %s", status.State, status.Message)
	}
}

func waitForTerminal(t *testing.T, controller *Controller) OperationStatus {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status := controller.Status()
		if status.State.Terminal() {
			return status
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("operation did not reach a terminal state")
	return OperationStatus{}
}
