package elevation

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const (
	ActionM1ElevationProbe = "m1.elevation.probe"
	contractVersion        = 2
	maxExecutionTimeout    = 30 * time.Second
	timeoutProbeOverrun    = 250 * time.Millisecond
	maxProbeDelay          = maxExecutionTimeout + timeoutProbeOverrun
)

type ProbeMode string

const (
	ProbeModeConsent      ProbeMode = "consent"
	ProbeModeCancellation ProbeMode = "cancellation-test"
	ProbeModeTimeout      ProbeMode = "timeout-test"
)

type State string

const (
	StateQueued              State = "queued"
	StateAwaitingConsent     State = "awaiting-consent"
	StateRunning             State = "running"
	StateCancellationPending State = "cancellation-requested"
	StateCancelled           State = "cancelled"
	StateTimedOut            State = "timed-out"
	StateSucceeded           State = "succeeded"
	StateFailed              State = "failed"
)

func (s State) Terminal() bool {
	return s == StateCancelled || s == StateTimedOut || s == StateSucceeded || s == StateFailed
}

type Request struct {
	Version                int        `json:"version"`
	ID                     string     `json:"id"`
	ActionID               string     `json:"actionId"`
	ProbeMode              ProbeMode  `json:"probeMode"`
	CreatedAt              time.Time  `json:"createdAt"`
	ExecutionTimeoutMillis int        `json:"executionTimeoutMillis"`
	ExecutionStartedAt     *time.Time `json:"executionStartedAt,omitempty"`
	ExecutionDeadline      *time.Time `json:"executionDeadline,omitempty"`
	ProbeDelayMillis       int        `json:"probeDelayMillis"`
}

type OperationStatus struct {
	ID        string    `json:"id"`
	ActionID  string    `json:"actionId"`
	State     State     `json:"state"`
	Message   string    `json:"message"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func NewRequest(now time.Time, timeout time.Duration, mode ProbeMode) (Request, error) {
	if timeout <= 0 || timeout > maxExecutionTimeout {
		return Request{}, fmt.Errorf("elevation timeout must be between 1ms and %s", maxExecutionTimeout)
	}
	delay, err := probeDelay(mode, timeout)
	if err != nil {
		return Request{}, err
	}

	identifier := make([]byte, 16)
	if _, err := rand.Read(identifier); err != nil {
		return Request{}, fmt.Errorf("generate elevation request id: %w", err)
	}

	return Request{
		Version:                contractVersion,
		ID:                     hex.EncodeToString(identifier),
		ActionID:               ActionM1ElevationProbe,
		ProbeMode:              mode,
		CreatedAt:              now.UTC(),
		ExecutionTimeoutMillis: int(timeout / time.Millisecond),
		ProbeDelayMillis:       int(delay / time.Millisecond),
	}, nil
}

func (r Request) Activate(now time.Time) (Request, error) {
	if r.Activated() {
		return Request{}, errors.New("elevation request is already activated")
	}
	startedAt := now.UTC()
	deadline := startedAt.Add(r.executionTimeout())
	r.ExecutionStartedAt = &startedAt
	r.ExecutionDeadline = &deadline
	if err := r.Validate(startedAt); err != nil {
		return Request{}, err
	}
	return r, nil
}

func (r Request) Activated() bool {
	return r.ExecutionStartedAt != nil && r.ExecutionDeadline != nil
}

func (r Request) Validate(now time.Time) error {
	if r.Version != contractVersion {
		return fmt.Errorf("unsupported elevation contract version %d", r.Version)
	}
	if len(r.ID) != 32 {
		return errors.New("invalid elevation request id")
	}
	if _, err := hex.DecodeString(r.ID); err != nil {
		return errors.New("invalid elevation request id")
	}
	if r.ActionID != ActionM1ElevationProbe {
		return errors.New("elevation action is not allow-listed")
	}
	if !r.ProbeMode.valid() {
		return errors.New("elevation probe mode is not allow-listed")
	}
	if r.CreatedAt.IsZero() || r.CreatedAt.After(now.UTC().Add(time.Second)) {
		return errors.New("invalid elevation request creation time")
	}
	if r.ExecutionTimeoutMillis <= 0 || time.Duration(r.ExecutionTimeoutMillis)*time.Millisecond > maxExecutionTimeout {
		return errors.New("invalid elevation execution timeout")
	}
	if r.ProbeDelayMillis < 0 || time.Duration(r.ProbeDelayMillis)*time.Millisecond > maxProbeDelay {
		return errors.New("invalid elevation probe delay")
	}
	if (r.ExecutionStartedAt == nil) != (r.ExecutionDeadline == nil) {
		return errors.New("incomplete elevation execution window")
	}
	if r.Activated() {
		startedAt := r.ExecutionStartedAt.UTC()
		deadline := r.ExecutionDeadline.UTC()
		if startedAt.Before(r.CreatedAt.UTC()) {
			return errors.New("elevation execution starts before request creation")
		}
		if deadline.Sub(startedAt) != r.executionTimeout() {
			return errors.New("invalid elevation execution window")
		}
		if startedAt.After(now.UTC().Add(time.Second)) {
			return errors.New("elevation execution window starts in the future")
		}
		if !deadline.After(now.UTC()) {
			return errors.New("elevation execution window has expired")
		}
	}
	return nil
}

func (r Request) executionTimeout() time.Duration {
	return time.Duration(r.ExecutionTimeoutMillis) * time.Millisecond
}

func (m ProbeMode) valid() bool {
	return m == ProbeModeConsent || m == ProbeModeCancellation || m == ProbeModeTimeout
}

func probeDelay(mode ProbeMode, timeout time.Duration) (time.Duration, error) {
	switch mode {
	case ProbeModeConsent:
		return 0, nil
	case ProbeModeCancellation:
		return timeout / 2, nil
	case ProbeModeTimeout:
		return timeout + timeoutProbeOverrun, nil
	default:
		return 0, errors.New("elevation probe mode is not allow-listed")
	}
}
