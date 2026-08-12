package elevation

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const activationHandoffTimeout = 5 * time.Second

func (s Store) RunM1Probe(ctx context.Context, id string) (OperationStatus, error) {
	request, err := s.waitForActivation(ctx, id)
	if err != nil {
		return OperationStatus{}, err
	}

	status := statusFor(request, StateRunning, "Elevated M1 probe is running.")
	if err := s.SaveStatus(status); err != nil {
		return OperationStatus{}, err
	}

	delay := time.Duration(request.ProbeDelayMillis) * time.Millisecond
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	completion := time.NewTimer(delay)
	defer completion.Stop()
	executionTimeout := time.NewTimer(time.Until(*request.ExecutionDeadline))
	defer executionTimeout.Stop()

	for {
		cancelled, err := s.CancellationRequested(request.ID)
		if err != nil {
			return OperationStatus{}, err
		}
		if cancelled || ctx.Err() != nil {
			status = statusFor(request, StateCancelled, "Elevation probe cancelled before any cleanup command was started.")
			return status, s.SaveStatus(status)
		}
		select {
		case <-completion.C:
			status = statusFor(request, StateSucceeded, "Elevation probe completed. No filesystem or tool command was executed.")
			return status, s.SaveStatus(status)
		case <-executionTimeout.C:
			status = statusFor(request, StateTimedOut, "Elevation probe timed out before any cleanup command was started.")
			return status, s.SaveStatus(status)
		case <-ticker.C:
		case <-ctx.Done():
		}
	}
}

func (s Store) waitForActivation(ctx context.Context, id string) (Request, error) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(activationHandoffTimeout)
	defer timeout.Stop()

	for {
		request, err := s.LoadRequest(id)
		if err != nil {
			return Request{}, err
		}
		if err := request.Validate(time.Now().UTC()); err != nil {
			return Request{}, err
		}
		if request.Activated() {
			return request, nil
		}

		select {
		case <-ticker.C:
		case <-timeout.C:
			return Request{}, errors.New("elevation execution window was not activated")
		case <-ctx.Done():
			return Request{}, fmt.Errorf("wait for elevation activation: %w", ctx.Err())
		}
	}
}

func statusFor(request Request, state State, message string) OperationStatus {
	return OperationStatus{ID: request.ID, ActionID: request.ActionID, State: state, Message: message, UpdatedAt: time.Now().UTC()}
}
