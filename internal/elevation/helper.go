package elevation

import (
	"context"
	"time"
)

func (s Store) RunM1Probe(ctx context.Context, id string) (OperationStatus, error) {
	request, err := s.LoadRequest(id)
	if err != nil {
		return OperationStatus{}, err
	}
	if err := request.Validate(time.Now().UTC()); err != nil {
		return OperationStatus{}, err
	}

	status := statusFor(request, StateRunning, "Elevated M1 probe is running.")
	if err := s.SaveStatus(status); err != nil {
		return OperationStatus{}, err
	}

	delay := time.Duration(request.ProbeDelayMillis) * time.Millisecond
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(delay)
	defer deadline.Stop()

	for {
		cancelled, err := s.CancellationRequested(request.ID)
		if err != nil {
			return OperationStatus{}, err
		}
		if cancelled || ctx.Err() != nil {
			status = statusFor(request, StateCancelled, "Elevation probe cancelled before any cleanup command was started.")
			return status, s.SaveStatus(status)
		}
		if !request.ExpiresAt.After(time.Now().UTC()) {
			status = statusFor(request, StateTimedOut, "Elevation probe timed out before any cleanup command was started.")
			return status, s.SaveStatus(status)
		}

		select {
		case <-deadline.C:
			status = statusFor(request, StateSucceeded, "Elevation probe completed. No filesystem or tool command was executed.")
			return status, s.SaveStatus(status)
		case <-ticker.C:
		case <-ctx.Done():
		}
	}
}

func statusFor(request Request, state State, message string) OperationStatus {
	return OperationStatus{ID: request.ID, ActionID: request.ActionID, State: state, Message: message, UpdatedAt: time.Now().UTC()}
}
