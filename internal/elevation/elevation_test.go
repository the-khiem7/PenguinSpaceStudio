package elevation

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRequestRejectsUnknownActionAndExpiredRequest(t *testing.T) {
	request, err := NewRequest(time.Now().UTC(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	request.ActionID = "powershell -Command Remove-Item"
	if err := request.Validate(time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "allow-listed") {
		t.Fatalf("expected allow-list rejection, got %v", err)
	}

	request.ActionID = ActionM1ElevationProbe
	request.ExpiresAt = time.Now().UTC().Add(-time.Second)
	if err := request.Validate(time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expiry rejection, got %v", err)
	}
}

func TestControllerCompletesSafeProbe(t *testing.T) {
	store := NewStore(t.TempDir())
	controller := NewController(store, LauncherFunc(func(id string) error {
		go func() { _, _ = store.RunM1Probe(context.Background(), id) }()
		return nil
	}), time.Second)

	if _, err := controller.StartM1Probe(); err != nil {
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
		go func() {
			request, err := store.LoadRequest(id)
			if err == nil {
				request.ProbeDelayMillis = 500
				_ = store.SaveRequest(request)
			}
			_, _ = store.RunM1Probe(context.Background(), id)
		}()
		return nil
	}), time.Second)

	if _, err := controller.StartM1Probe(); err != nil {
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

	if _, err := controller.StartM1Probe(); err != nil {
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
