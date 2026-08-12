package uv

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
	ProviderID  = "uv.global-cache"
	cacheItemID = "uv-global-cache"
	planID      = "uv-cache-prune-plan"
	actionID    = "uv-cache-prune"
)

var versionPattern = regexp.MustCompile(`^uv\s+(\d+)\.(\d+)\.(\d+)(?:\s+.*)?$`)

type CommandRunner = common.CommandRunner
type Provider struct{ runner CommandRunner }

func NewProvider(runner CommandRunner) *Provider { return &Provider{runner: runner} }
func NewSystemProvider() *Provider               { return NewProvider(common.SystemRunner{}) }
func (p *Provider) ID() string                   { return ProviderID }
func (p *Provider) ExecutionEnabled() bool       { return true }

func (p *Provider) Detect(ctx context.Context) (core.ProviderDetection, error) {
	executable, err := p.runner.LookPath("uv")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return core.ProviderDetection{ProviderID: ProviderID, Message: "uv was not found on PATH."}, nil
		}
		return core.ProviderDetection{}, fmt.Errorf("locate uv executable: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return core.ProviderDetection{}, fmt.Errorf("resolve uv executable path: %w", err)
	}
	output, err := p.runner.Run(ctx, executable, "--version")
	if err != nil {
		return core.ProviderDetection{}, fmt.Errorf("read uv version: %w", err)
	}
	versionOutput := strings.TrimSpace(output)
	major, minor, version, err := parseVersion(versionOutput)
	if err != nil {
		return core.ProviderDetection{ProviderID: ProviderID, Detected: true, Version: versionOutput, ExecutablePath: executable, Message: "uv was detected, but its version could not be safely classified."}, nil
	}
	supported := major == 0 && minor == 12
	message := "uv 0.12.x is supported for cache inspection and periodic pruning."
	if !supported {
		message = fmt.Sprintf("uv %d.%d.x is detected but not supported; pre-1.0 minor releases are gated independently, so no prune plan will be created.", major, minor)
	}
	return core.ProviderDetection{ProviderID: ProviderID, Detected: true, Supported: supported, Version: version, ExecutablePath: executable, Message: message}, nil
}

func (p *Provider) Scan(ctx context.Context, detection core.ProviderDetection) (core.ScanResult, error) {
	if detection.ProviderID != ProviderID || !detection.Detected || !detection.Supported {
		return core.ScanResult{}, errors.New("uv provider is not available for scanning")
	}
	cacheRoot, err := p.resolveCacheRoot(ctx, detection.ExecutablePath)
	if err != nil {
		return core.ScanResult{}, err
	}
	bytes, err := p.resolveCacheSize(ctx, detection.ExecutablePath)
	if err != nil {
		return core.ScanResult{}, err
	}
	return core.ScanResult{ProviderID: ProviderID, ScannedAt: time.Now().UTC(), Items: []core.StorageItem{{
		ID: cacheItemID, Name: "uv global cache", StorageClass: core.StorageDisposable,
		Risk: core.RiskSafe, RecoveryCost: core.RecoveryDownload,
		Measured: core.Measurement{Bytes: bytes, Kind: core.MeasurementMeasuredLogical}, Location: cacheRoot,
	}}}, nil
}

func (p *Provider) Plan(scan core.ScanResult) (core.CleanupPlan, error) {
	if scan.ProviderID != ProviderID || len(scan.Items) != 1 || scan.Items[0].ID != cacheItemID {
		return core.CleanupPlan{}, errors.New("invalid uv cache scan result")
	}
	item := scan.Items[0]
	if item.Risk != core.RiskSafe || item.RecoveryCost != core.RecoveryDownload {
		return core.CleanupPlan{}, errors.New("uv cache classification does not match the allow-listed profile")
	}
	return core.CleanupPlan{ID: planID, ProviderID: ProviderID, CreatedAt: time.Now().UTC(), Actions: []core.CleanupAction{{
		ID: actionID, ItemID: cacheItemID, Location: item.Location, Risk: core.RiskSafe, RecoveryCost: core.RecoveryDownload,
		Consequence: "uv will remove unused cache entries and all centralized project environments. The displayed cache size is not a reclaim estimate; removed environments are recreated as needed, and packages or source builds may be downloaded or rebuilt again.",
		Observed:    item.Measured,
		Estimated:   core.Measurement{Kind: core.MeasurementUnavailable},
	}}}, nil
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
		return core.ExecutionResult{}, errors.New("uv is no longer available for pruning")
	}
	currentRoot, err := p.resolveCacheRoot(ctx, detection.ExecutablePath)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	if !common.SamePath(currentRoot, plan.Actions[0].Location) {
		return core.ExecutionResult{}, errors.New("uv cache path changed after review; inspect again before pruning")
	}
	commandDirectory, cleanup, err := common.NewCommandContext("uv-probe")
	if err != nil {
		return core.ExecutionResult{}, fmt.Errorf("create isolated uv command context: %w", err)
	}
	defer cleanup()
	if _, err := p.runner.Run(ctx, detection.ExecutablePath, "--directory", commandDirectory, "cache", "prune"); err != nil {
		return core.ExecutionResult{}, fmt.Errorf("prune uv cache: %w", err)
	}
	return core.ExecutionResult{PlanID: plan.ID, Executed: true, Destructive: true, Message: "uv cache prune completed; removed entries and centralized environments may be recreated later."}, nil
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
	before, afterBytes := plan.Actions[0].Observed.Bytes, after.Items[0].Measured.Bytes
	reclaimed := uint64(0)
	if before > afterBytes {
		reclaimed = before - afterBytes
	}
	return core.VerificationResult{PlanID: plan.ID, MeasuredAfter: after.Items[0].Measured, ReclaimedActual: core.Measurement{Bytes: reclaimed, Kind: core.MeasurementMeasuredLogical}}, nil
}

func (p *Provider) resolveCacheRoot(ctx context.Context, executable string) (string, error) {
	commandDirectory, cleanup, err := common.NewCommandContext("uv-probe")
	if err != nil {
		return "", fmt.Errorf("create isolated uv command context: %w", err)
	}
	defer cleanup()
	output, err := p.runner.Run(ctx, executable, "--directory", commandDirectory, "cache", "dir")
	if err != nil {
		return "", fmt.Errorf("resolve uv cache: %w", err)
	}
	return common.ValidateStorageRoot(output, "uv")
}

func (p *Provider) resolveCacheSize(ctx context.Context, executable string) (uint64, error) {
	commandDirectory, cleanup, err := common.NewCommandContext("uv-probe")
	if err != nil {
		return 0, fmt.Errorf("create isolated uv command context: %w", err)
	}
	defer cleanup()
	output, err := p.runner.Run(ctx, executable, "--directory", commandDirectory, "--preview-features", "cache-size", "cache", "size")
	if err != nil {
		return 0, fmt.Errorf("measure uv cache: %w", err)
	}
	bytes, err := strconv.ParseUint(strings.TrimSpace(output), 10, 64)
	if err != nil {
		return 0, errors.New("uv cache size returned an invalid byte count")
	}
	return bytes, nil
}

func parseVersion(version string) (int, int, string, error) {
	matches := versionPattern.FindStringSubmatch(version)
	if len(matches) != 4 {
		return 0, 0, "", errors.New("unsupported uv version format")
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, 0, "", errors.New("invalid uv major version")
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return 0, 0, "", errors.New("invalid uv minor version")
	}
	return major, minor, strings.Join(matches[1:4], "."), nil
}

func validatePlan(plan core.CleanupPlan) error {
	if plan.ID != planID || plan.ProviderID != ProviderID || len(plan.Actions) != 1 {
		return errors.New("unrecognised uv prune plan")
	}
	action := plan.Actions[0]
	if action.ID != actionID || action.ItemID != cacheItemID || action.Risk != core.RiskSafe || action.RecoveryCost != core.RecoveryDownload || action.Estimated.Kind != core.MeasurementUnavailable {
		return errors.New("unrecognised uv prune action")
	}
	if _, err := common.ValidateStorageRoot(action.Location, "uv"); err != nil {
		return fmt.Errorf("invalid uv cache target: %w", err)
	}
	return nil
}
