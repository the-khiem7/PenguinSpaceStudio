package elevation

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Launcher interface {
	Launch(requestID string) error
}

type LauncherFunc func(requestID string) error

func (f LauncherFunc) Launch(requestID string) error { return f(requestID) }

type Controller struct {
	store    Store
	launcher Launcher
	timeout  time.Duration
	poll     time.Duration

	mu      sync.RWMutex
	current OperationStatus
}

func NewController(store Store, launcher Launcher, timeout time.Duration) *Controller {
	return &Controller{store: store, launcher: launcher, timeout: timeout, poll: 50 * time.Millisecond}
}

func (c *Controller) StartM1Probe() (OperationStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.current.State.Terminal() && c.current.ID != "" {
		return OperationStatus{}, errors.New("an elevation operation is already in progress")
	}

	request, err := NewRequest(time.Now().UTC(), c.timeout)
	if err != nil {
		return OperationStatus{}, err
	}
	if err := c.store.SaveRequest(request); err != nil {
		return OperationStatus{}, err
	}
	c.current = statusFor(request, StateQueued, "Elevation probe queued; Windows consent may be requested.")
	go c.run(request)
	return c.current, nil
}

func (c *Controller) Cancel() (OperationStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.current.ID == "" || c.current.State.Terminal() {
		return c.current, errors.New("no elevation operation is in progress")
	}
	if err := c.store.RequestCancellation(c.current.ID); err != nil {
		return OperationStatus{}, err
	}
	c.current.State = StateCancellationPending
	c.current.Message = "Cancellation requested. The helper will stop before any cleanup command can run."
	c.current.UpdatedAt = time.Now().UTC()
	return c.current, nil
}

func (c *Controller) Status() OperationStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.current
}

func (c *Controller) run(request Request) {
	c.set(statusFor(request, StateAwaitingConsent, "Awaiting Windows elevation consent."))
	if err := c.launcher.Launch(request.ID); err != nil {
		c.set(statusFor(request, StateFailed, "Elevation was not launched: "+err.Error()))
		return
	}
	c.set(statusFor(request, StateRunning, "Elevated helper started; waiting for its safe probe result."))

	ticker := time.NewTicker(c.poll)
	defer ticker.Stop()
	timeout := time.NewTimer(time.Until(request.ExpiresAt))
	defer timeout.Stop()

	for {
		if status, err := c.store.LoadStatus(request.ID); err == nil && status.State.Terminal() {
			c.set(status)
			return
		}

		select {
		case <-ticker.C:
		case <-timeout.C:
			_ = c.store.RequestCancellation(request.ID)
			c.set(statusFor(request, StateTimedOut, "Elevation probe timed out; no cleanup command was started."))
			return
		}
	}
}

func (c *Controller) set(status OperationStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current = status
}

func (c *Controller) RunHelper(ctx context.Context, requestID string) (OperationStatus, error) {
	return c.store.RunM1Probe(ctx, requestID)
}
