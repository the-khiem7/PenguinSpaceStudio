package bun

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/providers/common"
)

const (
	ProviderID  = "bun.global-cache"
	cacheItemID = "bun-global-module-cache"
	planID      = "bun-global-cache-plan"
	actionID    = "bun-global-cache-clear"
)

var versionPattern = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:[-+].*)?$`)

type CommandRunner = common.CommandRunner

type Provider struct {
	runner CommandRunner
}

func NewProvider(runner CommandRunner) *Provider {
	return &Provider{runner: runner}
}

func NewSystemProvider() *Provider {
	return NewProvider(common.SystemRunner{})
}

func (p *Provider) ID() string { return ProviderID }

func (p *Provider) ExecutionEnabled() bool { return true }

func (p *Provider) Detect(ctx context.Context) (core.ProviderDetection, error) {
	executable, err := p.runner.LookPath("bun")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return core.ProviderDetection{
				ProviderID: ProviderID,
				Message:    "Bun was not found on PATH.",
			}, nil
		}
		return core.ProviderDetection{}, fmt.Errorf("locate Bun executable: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return core.ProviderDetection{}, fmt.Errorf("resolve Bun executable path: %w", err)
	}

	output, err := p.runner.Run(ctx, executable, "--version")
	if err != nil {
		return core.ProviderDetection{}, fmt.Errorf("read Bun version: %w", err)
	}
	version := strings.TrimSpace(output)
	major, err := parseMajorVersion(version)
	if err != nil {
		return core.ProviderDetection{
			ProviderID:     ProviderID,
			Detected:       true,
			Version:        version,
			ExecutablePath: executable,
			Message:        "Bun was detected, but its version could not be safely classified.",
		}, nil
	}
	supported := major == 1
	message := "Bun 1.x is supported for cache inspection and planning."
	if !supported {
		message = fmt.Sprintf("Bun major version %d is detected but not supported; no cleanup plan will be created.", major)
	}
	return core.ProviderDetection{
		ProviderID:     ProviderID,
		Detected:       true,
		Supported:      supported,
		Version:        version,
		ExecutablePath: executable,
		Message:        message,
	}, nil
}

func (p *Provider) Scan(ctx context.Context, detection core.ProviderDetection) (core.ScanResult, error) {
	if detection.ProviderID != ProviderID || !detection.Detected || !detection.Supported {
		return core.ScanResult{}, errors.New("Bun provider is not available for scanning")
	}
	cacheRoot, err := p.resolveCacheRoot(ctx, detection.ExecutablePath)
	if err != nil {
		return core.ScanResult{}, err
	}
	bytes, err := common.MeasureDirectory(ctx, cacheRoot)
	if err != nil {
		return core.ScanResult{}, fmt.Errorf("measure Bun global cache: %w", err)
	}

	return core.ScanResult{
		ProviderID: ProviderID,
		ScannedAt:  time.Now().UTC(),
		Items: []core.StorageItem{{
			ID:           cacheItemID,
			Name:         "Bun global module cache",
			StorageClass: core.StorageDisposable,
			Risk:         core.RiskSafe,
			RecoveryCost: core.RecoveryDownload,
			Measured: core.Measurement{
				Bytes: bytes,
				Kind:  core.MeasurementMeasuredLogical,
			},
			Location: cacheRoot,
		}},
	}, nil
}

func createProbeDirectory() (string, func(), error) {
	directory, err := os.MkdirTemp("", "penguinspace-bun-probe-")
	if err != nil {
		return "", nil, err
	}
	manifest := filepath.Join(directory, "package.json")
	if err := os.WriteFile(manifest, []byte("{\"private\":true}\n"), 0o600); err != nil {
		_ = os.Remove(directory)
		return "", nil, err
	}
	cleanup := func() {
		_ = os.Remove(manifest)
		_ = os.Remove(directory)
	}
	return directory, cleanup, nil
}

func (p *Provider) resolveCacheRoot(ctx context.Context, executable string) (string, error) {
	probeDirectory, cleanup, err := createProbeDirectory()
	if err != nil {
		return "", fmt.Errorf("create isolated Bun command context: %w", err)
	}
	defer cleanup()
	output, err := p.runner.Run(ctx, executable, "pm", "--cwd", probeDirectory, "cache")
	if err != nil {
		return "", fmt.Errorf("resolve Bun global cache: %w", err)
	}
	return common.ValidateStorageRoot(output, "Bun")
}

func (p *Provider) Plan(scan core.ScanResult) (core.CleanupPlan, error) {
	if scan.ProviderID != ProviderID || len(scan.Items) != 1 || scan.Items[0].ID != cacheItemID {
		return core.CleanupPlan{}, errors.New("invalid Bun cache scan result")
	}
	item := scan.Items[0]
	if item.StorageClass != core.StorageDisposable || item.Risk != core.RiskSafe || item.RecoveryCost != core.RecoveryDownload {
		return core.CleanupPlan{}, errors.New("Bun cache classification does not match the allow-listed profile")
	}
	return core.CleanupPlan{
		ID:         planID,
		ProviderID: ProviderID,
		CreatedAt:  time.Now().UTC(),
		Actions: []core.CleanupAction{{
			ID:           actionID,
			ItemID:       cacheItemID,
			Location:     item.Location,
			Risk:         core.RiskSafe,
			RecoveryCost: core.RecoveryDownload,
			Consequence:  "Bun may download packages again. Reported bytes are logical cache size and may exceed physical reclaim because Bun can hardlink cached packages on Windows.",
			Observed:     item.Measured,
			Estimated: core.Measurement{
				Bytes: item.Measured.Bytes,
				Kind:  core.MeasurementEstimatedLogical,
			},
		}},
	}, nil
}

func (p *Provider) Execute(ctx context.Context, plan core.CleanupPlan, confirmed bool) (core.ExecutionResult, error) {
	if !confirmed {
		return core.ExecutionResult{}, errors.New("cleanup plan requires confirmation")
	}
	if err := validatePlan(plan); err != nil {
		return core.ExecutionResult{}, err
	}
	detection, err := p.Detect(ctx)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	if !detection.Detected || !detection.Supported {
		return core.ExecutionResult{}, errors.New("Bun is no longer available for cleanup")
	}
	currentRoot, err := p.resolveCacheRoot(ctx, detection.ExecutablePath)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	if !samePath(currentRoot, plan.Actions[0].Location) {
		return core.ExecutionResult{}, errors.New("Bun cache path changed after review; inspect again before cleanup")
	}

	probeDirectory, cleanup, err := createProbeDirectory()
	if err != nil {
		return core.ExecutionResult{}, fmt.Errorf("create isolated Bun command context: %w", err)
	}
	defer cleanup()
	if _, err := p.runner.Run(ctx, detection.ExecutablePath, "pm", "--cwd", probeDirectory, "cache", "rm"); err != nil {
		return core.ExecutionResult{}, fmt.Errorf("clear Bun global cache: %w", err)
	}
	return core.ExecutionResult{
		PlanID:      plan.ID,
		Executed:    true,
		Destructive: true,
		Message:     "Bun global cache cleanup completed; packages may be downloaded again.",
	}, nil
}

func (p *Provider) Verify(ctx context.Context, plan core.CleanupPlan) (core.VerificationResult, error) {
	if err := validatePlan(plan); err != nil {
		return core.VerificationResult{}, err
	}
	detection, err := p.Detect(ctx)
	if err != nil {
		return core.VerificationResult{}, err
	}
	after, err := p.Scan(ctx, detection)
	if err != nil {
		return core.VerificationResult{}, err
	}
	before := plan.Actions[0].Observed.Bytes
	afterBytes := after.Items[0].Measured.Bytes
	reclaimed := uint64(0)
	if before > afterBytes {
		reclaimed = before - afterBytes
	}
	return core.VerificationResult{
		PlanID:          plan.ID,
		MeasuredAfter:   after.Items[0].Measured,
		ReclaimedActual: core.Measurement{Bytes: reclaimed, Kind: core.MeasurementMeasuredLogical},
	}, nil
}

func parseMajorVersion(version string) (int, error) {
	matches := versionPattern.FindStringSubmatch(version)
	if len(matches) != 4 {
		return 0, errors.New("unsupported Bun version format")
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, errors.New("invalid Bun major version")
	}
	return major, nil
}

func validatePlan(plan core.CleanupPlan) error {
	if plan.ID != planID || plan.ProviderID != ProviderID || len(plan.Actions) != 1 {
		return errors.New("unrecognised Bun cache plan")
	}
	action := plan.Actions[0]
	if action.ID != actionID || action.ItemID != cacheItemID || action.Risk != core.RiskSafe || action.RecoveryCost != core.RecoveryDownload {
		return errors.New("unrecognised Bun cache action")
	}
	if _, err := common.ValidateStorageRoot(action.Location, "Bun"); err != nil {
		return fmt.Errorf("invalid Bun cache action target: %w", err)
	}
	return nil
}

func samePath(left, right string) bool {
	return common.SamePath(left, right)
}
