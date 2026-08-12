package npm

import (
	"context"
	"errors"
	"fmt"
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
	ProviderID  = "npm.global-cache"
	cacheItemID = "npm-global-cache"
	planID      = "npm-global-cache-plan"
	actionID    = "npm-global-cache-clear"
)

var versionPattern = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:[-+].*)?$`)

type CommandRunner = common.CommandRunner

type Provider struct {
	runner CommandRunner
}

func NewProvider(runner CommandRunner) *Provider { return &Provider{runner: runner} }

func NewSystemProvider() *Provider { return NewProvider(common.SystemRunner{}) }

func (p *Provider) ID() string { return ProviderID }

func (p *Provider) ExecutionEnabled() bool { return true }

func (p *Provider) Detect(ctx context.Context) (core.ProviderDetection, error) {
	executable, err := p.runner.LookPath("npm")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return core.ProviderDetection{ProviderID: ProviderID, Message: "npm was not found on PATH."}, nil
		}
		return core.ProviderDetection{}, fmt.Errorf("locate npm executable: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return core.ProviderDetection{}, fmt.Errorf("resolve npm executable path: %w", err)
	}
	output, err := p.runner.Run(ctx, executable, "--version")
	if err != nil {
		return core.ProviderDetection{}, fmt.Errorf("read npm version: %w", err)
	}
	version := strings.TrimSpace(output)
	major, err := parseMajorVersion(version)
	if err != nil {
		return core.ProviderDetection{ProviderID: ProviderID, Detected: true, Version: version, ExecutablePath: executable, Message: "npm was detected, but its version could not be safely classified."}, nil
	}
	supported := major == 10 || major == 11
	message := "npm 10.x and 11.x are supported for global cache management."
	if !supported {
		message = fmt.Sprintf("npm major version %d is detected but not supported; no cleanup plan will be created.", major)
	}
	return core.ProviderDetection{ProviderID: ProviderID, Detected: true, Supported: supported, Version: version, ExecutablePath: executable, Message: message}, nil
}

func (p *Provider) Scan(ctx context.Context, detection core.ProviderDetection) (core.ScanResult, error) {
	if detection.ProviderID != ProviderID || !detection.Detected || !detection.Supported {
		return core.ScanResult{}, errors.New("npm provider is not available for scanning")
	}
	cacheRoot, err := p.resolveCacheRoot(ctx, detection.ExecutablePath)
	if err != nil {
		return core.ScanResult{}, err
	}
	bytes, err := common.MeasureDirectory(ctx, filepath.Join(cacheRoot, "_cacache"))
	if err != nil {
		return core.ScanResult{}, fmt.Errorf("measure npm global cache: %w", err)
	}
	return core.ScanResult{
		ProviderID: ProviderID,
		ScannedAt:  time.Now().UTC(),
		Items: []core.StorageItem{{
			ID: cacheItemID, Name: "npm managed content cache (_cacache)", StorageClass: core.StorageDisposable,
			Risk: core.RiskReview, RecoveryCost: core.RecoveryDownload,
			Measured: core.Measurement{Bytes: bytes, Kind: core.MeasurementMeasuredLogical}, Location: cacheRoot,
		}},
	}, nil
}

func (p *Provider) Plan(scan core.ScanResult) (core.CleanupPlan, error) {
	if scan.ProviderID != ProviderID || len(scan.Items) != 1 || scan.Items[0].ID != cacheItemID {
		return core.CleanupPlan{}, errors.New("invalid npm cache scan result")
	}
	item := scan.Items[0]
	if item.StorageClass != core.StorageDisposable || item.Risk != core.RiskReview || item.RecoveryCost != core.RecoveryDownload {
		return core.CleanupPlan{}, errors.New("npm cache classification does not match the allow-listed profile")
	}
	return core.CleanupPlan{
		ID: planID, ProviderID: ProviderID, CreatedAt: time.Now().UTC(),
		Actions: []core.CleanupAction{{
			ID: actionID, ItemID: cacheItemID, Location: item.Location,
			Risk: core.RiskReview, RecoveryCost: core.RecoveryDownload,
			Consequence: "npm's self-healing _cacache content will be cleared with --force. Future installs may download packages again; npm logs and npx cache are outside this measured action, and npm cache verify is not used for scanning because it can garbage-collect data.",
			Observed:    item.Measured,
			Estimated:   core.Measurement{Bytes: item.Measured.Bytes, Kind: core.MeasurementEstimatedLogical},
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
		return core.ExecutionResult{}, errors.New("npm is no longer available for cleanup")
	}
	currentRoot, err := p.resolveCacheRoot(ctx, detection.ExecutablePath)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	if !common.SamePath(currentRoot, plan.Actions[0].Location) {
		return core.ExecutionResult{}, errors.New("npm cache path changed after review; inspect again before cleanup")
	}
	if _, err := p.runner.Run(ctx, detection.ExecutablePath, "cache", "clean", "--force"); err != nil {
		return core.ExecutionResult{}, fmt.Errorf("clear npm global cache: %w", err)
	}
	return core.ExecutionResult{PlanID: plan.ID, Executed: true, Destructive: true, Message: "npm global cache cleanup completed; packages may be downloaded again."}, nil
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
	return core.VerificationResult{PlanID: plan.ID, MeasuredAfter: after.Items[0].Measured, ReclaimedActual: core.Measurement{Bytes: reclaimed, Kind: core.MeasurementMeasuredLogical}}, nil
}

func (p *Provider) resolveCacheRoot(ctx context.Context, executable string) (string, error) {
	output, err := p.runner.Run(ctx, executable, "config", "get", "cache")
	if err != nil {
		return "", fmt.Errorf("resolve npm global cache: %w", err)
	}
	return common.ValidateStorageRoot(output, "npm")
}

func parseMajorVersion(version string) (int, error) {
	matches := versionPattern.FindStringSubmatch(version)
	if len(matches) != 4 {
		return 0, errors.New("unsupported npm version format")
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, errors.New("invalid npm major version")
	}
	return major, nil
}

func validatePlan(plan core.CleanupPlan) error {
	if plan.ID != planID || plan.ProviderID != ProviderID || len(plan.Actions) != 1 {
		return errors.New("unrecognised npm cache plan")
	}
	action := plan.Actions[0]
	if action.ID != actionID || action.ItemID != cacheItemID || action.Risk != core.RiskReview || action.RecoveryCost != core.RecoveryDownload {
		return errors.New("unrecognised npm cache action")
	}
	if _, err := common.ValidateStorageRoot(action.Location, "npm"); err != nil {
		return fmt.Errorf("invalid npm cache action target: %w", err)
	}
	return nil
}
