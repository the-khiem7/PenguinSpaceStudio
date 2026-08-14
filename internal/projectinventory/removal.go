package projectinventory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/providers/common"
)

// RemovalBoundary is the fixed statement rendered beside every removal plan and
// outcome. Per the 2026-08-14 removal-method decision, this phase's only removal
// method is a classified filesystem fallback; nothing here invokes a tool-native
// command, even for an artifact kind (Cargo/Gradle/Maven) that has one elsewhere.
const RemovalBoundary = "Removal targets exactly one already-claimed generated directory, revalidated immediately before deletion. Filesystem removal is the only method in this phase; no tool-native command is invoked. No other path is touched, and reclaimed bytes are logical only, never a physical host reclaim claim."

// RemovalController retains at most one reviewed removal plan at a time, mirroring
// the M3.5 Docker network removal lifecycle: Inspect issues a plan from a fresh,
// complete snapshot, and Execute consumes it only after independent action-time
// revalidation of the project marker, the artifact, and its reparse state.
type RemovalController struct {
	mu        sync.Mutex
	inspector *Inspector
	issued    core.ProjectArtifactRemovalPlan
}

func NewRemovalController(inspector *Inspector) *RemovalController {
	return &RemovalController{inspector: inspector}
}

// Inspect issues a plan for one exact artifact below one exact project. The caller
// supplies only paths that must already appear in a fresh discovery pass; there is
// no way to name an arbitrary filesystem path here.
func (c *RemovalController) Inspect(ctx context.Context, root, projectPath, artifactPath string) (core.ProjectArtifactRemovalPlan, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	plan, err := c.inspector.reviewArtifactRemoval(ctx, root, projectPath, artifactPath)
	if err != nil {
		return core.ProjectArtifactRemovalPlan{}, err
	}
	c.issued = plan
	return plan, nil
}

// Execute consumes the retained plan by its exact ID after explicit confirmation.
// Only one plan is ever retained; issuing a new plan or a mismatched ID invalidates
// the previous one.
func (c *RemovalController) Execute(ctx context.Context, planID string, confirmed bool) (core.ProjectArtifactRemovalOutcome, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !confirmed {
		return core.ProjectArtifactRemovalOutcome{}, errors.New("project artifact removal requires explicit confirmation")
	}
	if planID == "" || c.issued.ID == "" || planID != c.issued.ID {
		return core.ProjectArtifactRemovalOutcome{}, errors.New("review the artifact again before removal")
	}
	plan := c.issued
	c.issued = core.ProjectArtifactRemovalPlan{}
	return c.inspector.executeArtifactRemoval(ctx, plan)
}

// RecordArtifactRemovalOutcome appends a history record only for a verified-absent
// outcome, mirroring the Docker network removal pattern: an attempted-but-unverified
// removal is never presented as a completed, historied action.
func RecordArtifactRemovalOutcome(ctx context.Context, recorder core.HistoryRecorder, outcome core.ProjectArtifactRemovalOutcome, createdAt time.Time) core.ProjectArtifactRemovalOutcome {
	if !outcome.VerifiedAbsent {
		return outcome
	}
	if err := recorder.Append(ctx, core.HistoryRecord{
		ID:             fmt.Sprintf("project-artifact-%d", createdAt.UnixNano()),
		ProviderID:     core.ProjectArtifactRemovalProviderID,
		PlanID:         outcome.PlanID,
		ReclaimedBytes: outcome.ReclaimedActual.Bytes,
		ReclaimedKind:  outcome.ReclaimedActual.Kind,
		CreatedAt:      createdAt.UTC(),
	}); err != nil {
		outcome.Message = "The claimed generated directory was removed and verified absent, but its local history record failed."
		outcome.Failure = err.Error()
		return outcome
	}
	outcome.HistoryRecorded = true
	return outcome
}

func (i *Inspector) reviewArtifactRemoval(ctx context.Context, root, projectPath, artifactPath string) (core.ProjectArtifactRemovalPlan, error) {
	if strings.TrimSpace(projectPath) == "" || strings.TrimSpace(artifactPath) == "" {
		return core.ProjectArtifactRemovalPlan{}, errors.New("a project path and an artifact path are required")
	}
	discovery := i.Discover(ctx, root)
	if !discovery.RootApproved {
		return core.ProjectArtifactRemovalPlan{}, errors.New(discovery.Message)
	}
	if !discovery.Complete || discovery.Truncated {
		return core.ProjectArtifactRemovalPlan{}, errors.New("the discovery snapshot behind this review was incomplete or truncated; discover projects again before reviewing removal")
	}

	project, found := findProject(discovery, projectPath)
	if !found {
		return core.ProjectArtifactRemovalPlan{}, fmt.Errorf("%q is not a marker-backed project below the approved root; discover projects again before reviewing removal", projectPath)
	}
	artifact, found := findArtifact(project, artifactPath)
	if !found {
		return core.ProjectArtifactRemovalPlan{}, fmt.Errorf("%q is not a claimed generated directory of %q; discover projects again before reviewing removal", artifactPath, project.Name)
	}
	// Every M4.1 allow-list rule assigns Review risk; this rejects any future
	// extension that introduces a Danger-class artifact before M4.4 is revisited.
	if artifact.Risk != core.RiskReview {
		return core.ProjectArtifactRemovalPlan{}, errors.New("only a Review-risk artifact is eligible for removal in this phase")
	}

	state := &measureState{root: discovery.Root}
	measured := i.measureArtifact(ctx, state, artifact)

	return core.ProjectArtifactRemovalPlan{
		ID:             fmt.Sprintf("project-artifact-removal-%d", time.Now().UnixNano()),
		Root:           discovery.Root,
		ProjectPath:    project.Path,
		ArtifactPath:   artifact.Path,
		ArtifactName:   artifact.Name,
		RelativePath:   artifact.RelativePath,
		Ecosystem:      artifact.Ecosystem,
		Risk:           artifact.Risk,
		RecoveryCost:   artifact.RecoveryCost,
		Method:         core.ProjectArtifactRemovalMethodFilesystemFallback,
		Consequence:    artifact.Boundary,
		MeasuredBefore: measured.Measured,
		CreatedAt:      nowUTC(),
	}, nil
}

func (i *Inspector) executeArtifactRemoval(ctx context.Context, plan core.ProjectArtifactRemovalPlan) (core.ProjectArtifactRemovalOutcome, error) {
	if plan.ArtifactPath == "" || plan.Root == "" || plan.Method != core.ProjectArtifactRemovalMethodFilesystemFallback || plan.Risk != core.RiskReview {
		return core.ProjectArtifactRemovalOutcome{}, errors.New("project artifact removal plan is invalid")
	}

	// Independent fresh re-derivation; the retained plan's own fields are never
	// trusted for what actually gets deleted.
	discovery := i.Discover(ctx, plan.Root)
	if !discovery.RootApproved || !discovery.Complete || discovery.Truncated {
		return core.ProjectArtifactRemovalOutcome{}, errors.New("the approved root could not be freshly revalidated before removal; review again")
	}
	project, found := findProject(discovery, plan.ProjectPath)
	if !found {
		return core.ProjectArtifactRemovalOutcome{}, errors.New("the project marker is no longer present; review again before removal")
	}
	artifact, found := findArtifact(project, plan.ArtifactPath)
	if !found {
		return core.ProjectArtifactRemovalOutcome{}, errors.New("the artifact is no longer claimed by this project; review again before removal")
	}
	if artifact.Risk != core.RiskReview || artifact.Name != plan.ArtifactName || artifact.RelativePath != plan.RelativePath {
		return core.ProjectArtifactRemovalOutcome{}, errors.New("the artifact identity changed after review; review again before removal")
	}

	target, err := common.ValidateWorkspaceTarget(discovery.Root, artifact.Path, "artifact removal")
	if err != nil {
		return core.ProjectArtifactRemovalOutcome{}, fmt.Errorf("revalidate artifact target before removal: %w", err)
	}
	info, statErr := os.Lstat(target)
	if statErr != nil {
		return core.ProjectArtifactRemovalOutcome{}, fmt.Errorf("revalidate artifact target before removal: %w", statErr)
	}
	if isReparse(info.Mode()) || !info.IsDir() {
		return core.ProjectArtifactRemovalOutcome{}, errors.New("the artifact resolved to a reparse point or a non-directory at removal time; removal was not attempted")
	}

	// Measure immediately before removal so "before" reflects the exact bytes about
	// to be deleted, not a possibly stale plan-time measurement.
	state := &measureState{root: discovery.Root}
	before := i.measureArtifact(ctx, state, artifact)

	outcome := core.ProjectArtifactRemovalOutcome{
		PlanID:          plan.ID,
		ArtifactPath:    target,
		ArtifactName:    artifact.Name,
		Method:          core.ProjectArtifactRemovalMethodFilesystemFallback,
		MeasuredBefore:  before.Measured,
		MeasuredAfter:   core.Measurement{Kind: core.MeasurementUnavailable},
		ReclaimedActual: core.Measurement{Kind: core.MeasurementUnavailable},
	}

	outcome.RemovalAttempted = true
	if err := os.RemoveAll(target); err != nil {
		outcome.Message = "The filesystem removal command returned an error, so the artifact's state is uncertain."
		outcome.Failure = err.Error()
		return outcome, nil
	}
	outcome.RemovalCompleted = true

	if _, statErr := os.Lstat(target); statErr == nil {
		outcome.Message = "Filesystem removal completed without an error, but the artifact path is still present."
		outcome.Failure = "artifact path still exists after removal"
		return outcome, nil
	} else if !os.IsNotExist(statErr) {
		outcome.Message = "Filesystem removal completed, but absence could not be verified."
		outcome.Failure = statErr.Error()
		return outcome, nil
	}

	outcome.VerifiedAbsent = true
	outcome.MeasuredAfter = core.Measurement{Bytes: 0, Kind: core.MeasurementMeasuredLogical}
	// Reclaimed bytes are logical, not physical: they equal the exact bytes counted
	// immediately before deletion, only when that count was itself a complete
	// measured-logical value rather than an unavailable one.
	if before.Measured.Kind == core.MeasurementMeasuredLogical {
		outcome.ReclaimedActual = core.Measurement{Bytes: before.Measured.Bytes, Kind: core.MeasurementMeasuredLogical}
	}
	outcome.Message = "The claimed generated directory was removed by filesystem deletion and verified absent."
	return outcome, nil
}

func findArtifact(project core.ProjectObservation, artifactPath string) (core.ProjectArtifactObservation, bool) {
	for _, artifact := range project.Artifacts {
		if common.SamePath(artifact.Path, artifactPath) {
			return artifact, true
		}
	}
	return core.ProjectArtifactObservation{}, false
}
