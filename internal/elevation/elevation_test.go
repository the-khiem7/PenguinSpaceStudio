package elevation

import (
	"context"
	"errors"
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
	request.CreatedAt = time.Now().UTC().Add(-3 * time.Second)
	request, err = request.Activate(time.Now().UTC().Add(-2 * time.Second))
	if err != nil {
		t.Fatal(err)
	}
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
	if timeout.ProbeDelayMillis != 2250 {
		t.Fatalf("timeout delay = %dms, want 2250ms", timeout.ProbeDelayMillis)
	}
}

func TestControllerConsentWaitDoesNotConsumeExecutionTimeout(t *testing.T) {
	store := NewStore(t.TempDir())
	consentWait := 80 * time.Millisecond
	controller := NewController(store, LauncherFunc(func(id string) error {
		time.Sleep(consentWait)
		go func() { _, _ = store.RunM1Probe(context.Background(), id) }()
		return nil
	}), 40*time.Millisecond)

	startedAt := time.Now()
	if _, err := controller.StartM1Probe(ProbeModeConsent); err != nil {
		t.Fatal(err)
	}
	status := waitForTerminal(t, controller)
	if status.State != StateSucceeded {
		t.Fatalf("got %s after %s: %s", status.State, time.Since(startedAt), status.Message)
	}
	if elapsed := time.Since(startedAt); elapsed < consentWait {
		t.Fatalf("operation completed before simulated consent returned: %s", elapsed)
	}
}

func TestControllerTimeoutBeginsAfterConsent(t *testing.T) {
	store := NewStore(t.TempDir())
	consentWait := 80 * time.Millisecond
	executionTimeout := 40 * time.Millisecond
	controller := NewController(store, LauncherFunc(func(id string) error {
		time.Sleep(consentWait)
		go func() { _, _ = store.RunM1Probe(context.Background(), id) }()
		return nil
	}), executionTimeout)

	startedAt := time.Now()
	if _, err := controller.StartM1Probe(ProbeModeTimeout); err != nil {
		t.Fatal(err)
	}
	status := waitForTerminal(t, controller)
	if status.State != StateTimedOut {
		t.Fatalf("got %s after %s: %s", status.State, time.Since(startedAt), status.Message)
	}
	if elapsed := time.Since(startedAt); elapsed < consentWait+executionTimeout {
		t.Fatalf("execution timeout consumed consent wait: completed after %s", elapsed)
	}
}

func TestControllerReportsElevationRefusal(t *testing.T) {
	store := NewStore(t.TempDir())
	controller := NewController(store, LauncherFunc(func(string) error {
		return errors.New("the operation was canceled by the user")
	}), time.Second)

	if _, err := controller.StartM1Probe(ProbeModeConsent); err != nil {
		t.Fatal(err)
	}
	status := waitForTerminal(t, controller)
	if status.State != StateFailed || !strings.Contains(status.Message, "canceled by the user") {
		t.Fatalf("got %s: %s", status.State, status.Message)
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
