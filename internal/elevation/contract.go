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
	contractVersion        = 1
	maxProbeDelay          = 30 * time.Second
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
	Version          int       `json:"version"`
	ID               string    `json:"id"`
	ActionID         string    `json:"actionId"`
	ExpiresAt        time.Time `json:"expiresAt"`
	ProbeDelayMillis int       `json:"probeDelayMillis"`
}

type OperationStatus struct {
	ID        string    `json:"id"`
	ActionID  string    `json:"actionId"`
	State     State     `json:"state"`
	Message   string    `json:"message"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func NewRequest(now time.Time, timeout time.Duration) (Request, error) {
	if timeout <= 0 || timeout > maxProbeDelay {
		return Request{}, fmt.Errorf("elevation timeout must be between 1ms and %s", maxProbeDelay)
	}

	identifier := make([]byte, 16)
	if _, err := rand.Read(identifier); err != nil {
		return Request{}, fmt.Errorf("generate elevation request id: %w", err)
	}

	return Request{
		Version:   contractVersion,
		ID:        hex.EncodeToString(identifier),
		ActionID:  ActionM1ElevationProbe,
		ExpiresAt: now.UTC().Add(timeout),
	}, nil
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
	if r.ProbeDelayMillis < 0 || time.Duration(r.ProbeDelayMillis)*time.Millisecond > maxProbeDelay {
		return errors.New("invalid elevation probe delay")
	}
	if !r.ExpiresAt.After(now.UTC()) {
		return errors.New("elevation request has expired")
	}
	return nil
}
