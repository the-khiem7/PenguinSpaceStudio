package dockerinventory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
)

type NetworkRemovalController struct {
	mu        sync.Mutex
	inspector *Inspector
	issued    core.DockerNetworkRemovalPlan
}

func NewNetworkRemovalController(inspector *Inspector) *NetworkRemovalController {
	return &NetworkRemovalController{inspector: inspector}
}

func RecordNetworkRemovalOutcome(ctx context.Context, recorder core.HistoryRecorder, outcome core.DockerNetworkRemovalOutcome, createdAt time.Time) core.DockerNetworkRemovalOutcome {
	if !outcome.VerifiedAbsent {
		return outcome
	}
	if err := recorder.Append(ctx, core.HistoryRecord{
		ID:             fmt.Sprintf("docker-network-%d", createdAt.UnixNano()),
		ProviderID:     core.DockerNetworkRemovalProviderID,
		PlanID:         outcome.PlanID,
		ReclaimedBytes: outcome.ReclaimedActual.Bytes,
		ReclaimedKind:  outcome.ReclaimedActual.Kind,
		CreatedAt:      createdAt.UTC(),
	}); err != nil {
		outcome.Message = "The exact Compose network was removed and verified absent, but its local history record failed. Refreshed awareness is preserved."
		outcome.Failure = err.Error()
		return outcome
	}
	outcome.HistoryRecorded = true
	return outcome
}

func (c *NetworkRemovalController) Inspect(ctx context.Context, networkID string) (core.DockerNetworkRemovalPlan, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	plan, err := c.inspector.reviewNetworkRemoval(ctx, networkID)
	if err != nil {
		return core.DockerNetworkRemovalPlan{}, err
	}
	c.issued = plan
	return plan, nil
}

func (c *NetworkRemovalController) Execute(ctx context.Context, planID string, confirmed bool) (core.DockerNetworkRemovalOutcome, error) {
	return c.ExecuteWithRefresh(ctx, ctx, planID, confirmed)
}

func (c *NetworkRemovalController) ExecuteWithRefresh(executionContext, refreshContext context.Context, planID string, confirmed bool) (core.DockerNetworkRemovalOutcome, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !confirmed {
		return core.DockerNetworkRemovalOutcome{}, errors.New("Docker network removal requires explicit confirmation")
	}
	if planID == "" || c.issued.ID == "" || planID != c.issued.ID {
		return core.DockerNetworkRemovalOutcome{}, errors.New("inspect the Docker network again before removal")
	}
	plan := c.issued
	c.issued = core.DockerNetworkRemovalPlan{}
	outcome, err := c.inspector.executeNetworkRemoval(executionContext, plan)
	if err != nil {
		return core.DockerNetworkRemovalOutcome{}, err
	}
	if outcome.RemovalCommandAttempted {
		if !outcome.VerifiedAbsent {
			executable, reconciliationErr := c.inspector.dockerExecutable()
			if reconciliationErr == nil {
				reconciliationErr = c.inspector.verifyNetworkAbsent(refreshContext, executable, outcome.NetworkID)
			}
			if reconciliationErr == nil {
				outcome.VerifiedAbsent = true
				outcome.Message = "The Docker command or initial verification reported an error, but independent exact-ID reconciliation confirms the network is absent. Reclaimed bytes are unavailable."
			} else {
				outcome.Failure = strings.TrimSpace(outcome.Failure + "; exact-ID reconciliation: " + reconciliationErr.Error())
			}
		}
		outcome.Awareness = c.inspector.Inspect(refreshContext)
		outcome.AwarenessRefreshed = true
	}
	return outcome, nil
}

func (i *Inspector) reviewNetworkRemoval(ctx context.Context, networkID string) (core.DockerNetworkRemovalPlan, error) {
	if networkID == "" || strings.TrimSpace(networkID) != networkID {
		return core.DockerNetworkRemovalPlan{}, errors.New("an exact Docker network ID is required")
	}

	report := i.Inspect(ctx)
	if !report.Daemon.Available {
		return core.DockerNetworkRemovalPlan{}, errors.New("Docker daemon is unavailable; refresh inspection before reviewing removal")
	}
	if !report.OwnershipComplete {
		return core.DockerNetworkRemovalPlan{}, errors.New("Docker ownership inspection is incomplete; no network removal plan was issued")
	}

	for _, group := range report.OwnershipGroups {
		if group.Scope != "compose-project" || group.Project == "" {
			continue
		}
		for _, resource := range group.Resources {
			if resource.Kind != "network" || resource.ID != networkID {
				continue
			}
			if resource.Labels.Project != group.Project || resource.Labels.Network == "" || !validComposeScope("network", resource.Labels) {
				return core.DockerNetworkRemovalPlan{}, errors.New("network does not have valid canonical Compose project and network labels")
			}
			if !hasZeroAvailableAttachments(resource.Relationships) {
				return core.DockerNetworkRemovalPlan{}, errors.New("network attachment state is unavailable or non-zero; removal is not eligible")
			}
			return core.DockerNetworkRemovalPlan{
				ID:           fmt.Sprintf("docker-network-%d", time.Now().UnixNano()),
				NetworkID:    resource.ID,
				NetworkName:  resource.Name,
				Project:      resource.Labels.Project,
				NetworkLabel: resource.Labels.Network,
				Risk:         core.RiskReview,
				Consequence:  "Remove this exact unattached Compose network. Docker may recreate it for a future Compose run; no storage bytes are claimed.",
				CreatedAt:    time.Now().UTC(),
			}, nil
		}
	}
	return core.DockerNetworkRemovalPlan{}, errors.New("network is not eligible for exact Compose network removal")
}

func (i *Inspector) executeNetworkRemoval(ctx context.Context, plan core.DockerNetworkRemovalPlan) (core.DockerNetworkRemovalOutcome, error) {
	if plan.NetworkID == "" || plan.NetworkName == "" || plan.Project == "" || plan.NetworkLabel == "" || plan.Risk != core.RiskReview {
		return core.DockerNetworkRemovalOutcome{}, errors.New("Docker network removal plan is invalid")
	}

	executable, err := i.dockerExecutable()
	if err != nil {
		return core.DockerNetworkRemovalOutcome{}, err
	}
	current, err := i.inspectExactNetwork(ctx, executable, plan.NetworkID)
	if err != nil {
		return core.DockerNetworkRemovalOutcome{}, fmt.Errorf("re-inspect Docker network before removal: %w", err)
	}
	labels := composeLabels(current.Labels)
	if current.ID != plan.NetworkID || current.Name != plan.NetworkName || labels.Project != plan.Project || labels.Network != plan.NetworkLabel || !validComposeScope("network", labels) {
		return core.DockerNetworkRemovalOutcome{}, errors.New("Docker network identity or Compose labels changed after review; inspect again")
	}
	if current.Containers == nil {
		return core.DockerNetworkRemovalOutcome{}, errors.New("Docker omitted network attachment metadata after review; removal was not run")
	}
	if len(*current.Containers) != 0 {
		return core.DockerNetworkRemovalOutcome{}, errors.New("Docker network gained container attachments after review; removal was not run")
	}

	outcome := core.DockerNetworkRemovalOutcome{
		PlanID:                  plan.ID,
		NetworkID:               plan.NetworkID,
		NetworkName:             plan.NetworkName,
		RemovalCommandAttempted: true,
		ReclaimedActual:         core.Measurement{Kind: core.MeasurementUnavailable},
	}
	if _, err := i.runner.Run(ctx, executable, "network", "rm", plan.NetworkID); err != nil {
		outcome.Message = "The exact Docker network removal command returned an error, so daemon state is uncertain. Refreshed awareness reports the latest observable state."
		outcome.Failure = err.Error()
		return outcome, nil
	}

	outcome.RemovalCommandCompleted = true
	if err := i.verifyNetworkAbsent(ctx, executable, plan.NetworkID); err != nil {
		outcome.Message = "Docker accepted the exact network removal command, but absence verification failed. Refreshed awareness reports the latest observable state."
		outcome.Failure = err.Error()
		return outcome, nil
	}

	outcome.VerifiedAbsent = true
	outcome.Message = "The exact Compose network was removed and its ID is absent. Reclaimed bytes are unavailable."
	return outcome, nil
}

func (i *Inspector) dockerExecutable() (string, error) {
	executable, err := i.runner.LookPath("docker")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", errors.New("Docker CLI was not found on PATH")
		}
		return "", fmt.Errorf("Docker CLI lookup failed: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return "", fmt.Errorf("resolve Docker CLI path: %w", err)
	}
	return executable, nil
}

func (i *Inspector) inspectExactNetwork(ctx context.Context, executable, networkID string) (networkInspect, error) {
	output, err := i.runner.Run(ctx, executable, "network", "inspect", "--format", "{{json .}}", networkID)
	if err != nil {
		return networkInspect{}, err
	}
	lines := nonEmptyLines(output)
	if len(lines) != 1 {
		return networkInspect{}, fmt.Errorf("Docker returned %d network inspect rows", len(lines))
	}
	var value networkInspect
	if err := json.Unmarshal([]byte(lines[0]), &value); err != nil || value.ID == "" || value.Name == "" {
		return networkInspect{}, errors.New("Docker returned malformed network inspect data")
	}
	return value, nil
}

func (i *Inspector) verifyNetworkAbsent(ctx context.Context, executable, networkID string) error {
	output, err := i.runner.Run(ctx, executable, "network", "ls", "--no-trunc", "--filter", "id="+networkID, "--format", "json")
	if err != nil {
		return fmt.Errorf("verify Docker network removal: %w", err)
	}
	for _, line := range nonEmptyLines(output) {
		var value struct {
			ID string `json:"ID"`
		}
		if err := json.Unmarshal([]byte(line), &value); err != nil || value.ID == "" {
			return errors.New("Docker returned malformed network verification data")
		}
		if value.ID == networkID {
			return errors.New("Docker network is still present after removal command")
		}
	}
	return nil
}

func hasZeroAvailableAttachments(relationships []core.DockerRelationshipObservation) bool {
	found := false
	for _, relationship := range relationships {
		if relationship.Kind != "container-attachments" {
			continue
		}
		if found || !relationship.Available || relationship.Count != 0 {
			return false
		}
		found = true
	}
	return found
}
