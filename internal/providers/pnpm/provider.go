package pnpm

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
	ProviderID  = "pnpm.global-store"
	storeItemID = "pnpm-global-store"
	planID      = "pnpm-store-prune-plan"
	actionID    = "pnpm-store-prune"
)

var versionPattern = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:[-+].*)?$`)

type CommandRunner = common.CommandRunner
type Provider struct{ runner CommandRunner }

func NewProvider(runner CommandRunner) *Provider { return &Provider{runner: runner} }
func NewSystemProvider() *Provider               { return NewProvider(common.SystemRunner{}) }
func (p *Provider) ID() string                   { return ProviderID }
func (p *Provider) ExecutionEnabled() bool       { return true }

func (p *Provider) Detect(ctx context.Context) (core.ProviderDetection, error) {
	executable, err := p.runner.LookPath("pnpm")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return core.ProviderDetection{ProviderID: ProviderID, Message: "pnpm was not found on PATH."}, nil
		}
		return core.ProviderDetection{}, fmt.Errorf("locate pnpm executable: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return core.ProviderDetection{}, fmt.Errorf("resolve pnpm executable path: %w", err)
	}
	output, err := p.runner.Run(ctx, executable, "--version")
	if err != nil {
		return core.ProviderDetection{}, fmt.Errorf("read pnpm version: %w", err)
	}
	version := strings.TrimSpace(output)
	major, err := parseMajorVersion(version)
	if err != nil {
		return core.ProviderDetection{ProviderID: ProviderID, Detected: true, Version: version, ExecutablePath: executable, Message: "pnpm was detected, but its version could not be safely classified."}, nil
	}
	versionSupported := major == 11 || major == 12
	if !versionSupported {
		return core.ProviderDetection{ProviderID: ProviderID, Detected: true, Version: version, ExecutablePath: executable, Message: fmt.Sprintf("pnpm major version %d is detected but not supported; no prune plan will be created.", major)}, nil
	}
	explicitStore, err := p.hasExplicitStoreDir(ctx, executable)
	if err != nil {
		return core.ProviderDetection{}, err
	}
	supported := explicitStore
	message := "pnpm 11.x or 12.x with an explicit storeDir is supported for store inspection and pruning."
	if !explicitStore {
		message = "pnpm is supported by version, but no explicit storeDir is configured. Default pnpm stores are per disk, so discovery is deferred until project roots provide drive context; no prune plan will be created."
	}
	return core.ProviderDetection{ProviderID: ProviderID, Detected: true, Supported: supported, Version: version, ExecutablePath: executable, Message: message}, nil
}

func (p *Provider) Scan(ctx context.Context, detection core.ProviderDetection) (core.ScanResult, error) {
	if detection.ProviderID != ProviderID || !detection.Detected || !detection.Supported {
		return core.ScanResult{}, errors.New("pnpm provider is not available for scanning")
	}
	storeRoot, err := p.resolveStoreRoot(ctx, detection.ExecutablePath)
	if err != nil {
		return core.ScanResult{}, err
	}
	bytes, err := common.MeasureDirectory(ctx, storeRoot)
	if err != nil {
		return core.ScanResult{}, fmt.Errorf("measure pnpm store: %w", err)
	}
	return core.ScanResult{ProviderID: ProviderID, ScannedAt: time.Now().UTC(), Items: []core.StorageItem{{
		ID: storeItemID, Name: "pnpm content-addressable store", StorageClass: core.StorageDisposable,
		Risk: core.RiskSafe, RecoveryCost: core.RecoveryDownload,
		Measured: core.Measurement{Bytes: bytes, Kind: core.MeasurementMeasuredLogical}, Location: storeRoot,
	}}}, nil
}

func (p *Provider) Plan(scan core.ScanResult) (core.CleanupPlan, error) {
	if scan.ProviderID != ProviderID || len(scan.Items) != 1 || scan.Items[0].ID != storeItemID {
		return core.CleanupPlan{}, errors.New("invalid pnpm store scan result")
	}
	item := scan.Items[0]
	if item.Risk != core.RiskSafe || item.RecoveryCost != core.RecoveryDownload {
		return core.CleanupPlan{}, errors.New("pnpm store classification does not match the allow-listed profile")
	}
	return core.CleanupPlan{ID: planID, ProviderID: ProviderID, CreatedAt: time.Now().UTC(), Actions: []core.CleanupAction{{
		ID: actionID, ItemID: storeItemID, Location: item.Location, Risk: core.RiskSafe, RecoveryCost: core.RecoveryDownload,
		Consequence: "pnpm will remove only packages it considers unreferenced. The displayed store size is not a reclaim estimate; pruneable bytes are unavailable before execution, and future installs may download removed packages again.",
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
		return core.ExecutionResult{}, errors.New("pnpm is no longer available for pruning")
	}
	currentRoot, err := p.resolveStoreRoot(ctx, detection.ExecutablePath)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	if !common.SamePath(currentRoot, plan.Actions[0].Location) {
		return core.ExecutionResult{}, errors.New("pnpm store path changed after review; inspect again before pruning")
	}
	probeDirectory, cleanup, err := common.NewCommandContext("pnpm-probe")
	if err != nil {
		return core.ExecutionResult{}, fmt.Errorf("create isolated pnpm command context: %w", err)
	}
	defer cleanup()
	if _, err := p.runner.Run(ctx, detection.ExecutablePath, "--dir", probeDirectory, "store", "prune"); err != nil {
		return core.ExecutionResult{}, fmt.Errorf("prune pnpm store: %w", err)
	}
	return core.ExecutionResult{PlanID: plan.ID, Executed: true, Destructive: true, Message: "pnpm store prune completed; removed packages may be downloaded again."}, nil
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

func (p *Provider) resolveStoreRoot(ctx context.Context, executable string) (string, error) {
	probeDirectory, cleanup, err := common.NewCommandContext("pnpm-probe")
	if err != nil {
		return "", fmt.Errorf("create isolated pnpm command context: %w", err)
	}
	defer cleanup()
	output, err := p.runner.Run(ctx, executable, "--dir", probeDirectory, "store", "path")
	if err != nil {
		return "", fmt.Errorf("resolve pnpm store: %w", err)
	}
	return common.ValidateStorageRoot(output, "pnpm")
}

func (p *Provider) hasExplicitStoreDir(ctx context.Context, executable string) (bool, error) {
	probeDirectory, cleanup, err := common.NewCommandContext("pnpm-probe")
	if err != nil {
		return false, fmt.Errorf("create isolated pnpm command context: %w", err)
	}
	defer cleanup()
	output, err := p.runner.Run(ctx, executable, "--dir", probeDirectory, "config", "get", "store-dir")
	if err != nil {
		return false, fmt.Errorf("read pnpm storeDir configuration: %w", err)
	}
	value := strings.TrimSpace(output)
	if value == "" || strings.EqualFold(value, "undefined") || strings.EqualFold(value, "null") {
		return false, nil
	}
	if _, err := common.ValidateStorageRoot(value, "pnpm configured"); err != nil {
		return false, nil
	}
	return true, nil
}

func parseMajorVersion(version string) (int, error) {
	matches := versionPattern.FindStringSubmatch(version)
	if len(matches) != 4 {
		return 0, errors.New("unsupported pnpm version format")
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, errors.New("invalid pnpm major version")
	}
	return major, nil
}

func validatePlan(plan core.CleanupPlan) error {
	if plan.ID != planID || plan.ProviderID != ProviderID || len(plan.Actions) != 1 {
		return errors.New("unrecognised pnpm prune plan")
	}
	action := plan.Actions[0]
	if action.ID != actionID || action.ItemID != storeItemID || action.Risk != core.RiskSafe || action.RecoveryCost != core.RecoveryDownload || action.Estimated.Kind != core.MeasurementUnavailable {
		return errors.New("unrecognised pnpm prune action")
	}
	if _, err := common.ValidateStorageRoot(action.Location, "pnpm"); err != nil {
		return fmt.Errorf("invalid pnpm store target: %w", err)
	}
	return nil
}
