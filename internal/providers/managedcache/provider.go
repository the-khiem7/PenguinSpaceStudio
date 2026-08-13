package managedcache

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/providers/common"
)

type VersionParser func(output string) (string, bool, error)
type RootResolver func(ctx context.Context, runner common.CommandRunner, executable string) (string, error)
type Cleaner func(ctx context.Context, runner common.CommandRunner, executable string) error

type Config struct {
	ProviderID, Executable, ItemID, ItemName, PlanID, ActionID string
	VersionArguments                                           []string
	ParseVersion                                               VersionParser
	SupportedMessage, UnsupportedMessage, Consequence          string
	ResolveRoot                                                RootResolver
	Clean                                                      Cleaner
	Risk                                                       core.RiskLevel
	RecoveryCost                                               core.RecoveryCost
	EstimatedKind                                              core.MeasurementKind
}

type Provider struct {
	config Config
	runner common.CommandRunner
}

func NewProvider(config Config, runner common.CommandRunner) *Provider {
	return &Provider{config: config, runner: runner}
}

func (p *Provider) ID() string             { return p.config.ProviderID }
func (p *Provider) ExecutionEnabled() bool { return true }

func (p *Provider) Detect(ctx context.Context) (core.ProviderDetection, error) {
	executable, err := p.runner.LookPath(p.config.Executable)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return core.ProviderDetection{ProviderID: p.config.ProviderID, Message: fmt.Sprintf("%s was not found on PATH.", p.config.Executable)}, nil
		}
		return core.ProviderDetection{}, fmt.Errorf("locate %s executable: %w", p.config.Executable, err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return core.ProviderDetection{}, fmt.Errorf("resolve %s executable path: %w", p.config.Executable, err)
	}
	output, err := p.runner.Run(ctx, executable, p.config.VersionArguments...)
	if err != nil {
		return core.ProviderDetection{}, fmt.Errorf("read %s version: %w", p.config.Executable, err)
	}
	version, supported, err := p.config.ParseVersion(strings.TrimSpace(output))
	if err != nil {
		return core.ProviderDetection{ProviderID: p.config.ProviderID, Detected: true, Version: strings.TrimSpace(output), ExecutablePath: executable, Message: fmt.Sprintf("%s was detected, but its version could not be safely classified.", p.config.Executable)}, nil
	}
	message := p.config.SupportedMessage
	if !supported {
		message = p.config.UnsupportedMessage
	}
	return core.ProviderDetection{ProviderID: p.config.ProviderID, Detected: true, Supported: supported, Version: version, ExecutablePath: executable, Message: message}, nil
}

func (p *Provider) Scan(ctx context.Context, detection core.ProviderDetection) (core.ScanResult, error) {
	if detection.ProviderID != p.config.ProviderID || !detection.Detected || !detection.Supported {
		return core.ScanResult{}, errors.New("provider is not available for scanning")
	}
	root, err := p.config.ResolveRoot(ctx, p.runner, detection.ExecutablePath)
	if err != nil {
		return core.ScanResult{}, err
	}
	bytes, err := common.MeasureDirectory(ctx, root)
	if err != nil {
		return core.ScanResult{}, fmt.Errorf("measure %s: %w", p.config.ItemName, err)
	}
	return core.ScanResult{ProviderID: p.config.ProviderID, ScannedAt: time.Now().UTC(), Items: []core.StorageItem{{
		ID: p.config.ItemID, Name: p.config.ItemName, StorageClass: core.StorageDisposable,
		Risk: p.config.Risk, RecoveryCost: p.config.RecoveryCost,
		Measured: core.Measurement{Bytes: bytes, Kind: core.MeasurementMeasuredLogical}, Location: root,
	}}}, nil
}

func (p *Provider) Plan(scan core.ScanResult) (core.CleanupPlan, error) {
	if scan.ProviderID != p.config.ProviderID || len(scan.Items) != 1 || scan.Items[0].ID != p.config.ItemID {
		return core.CleanupPlan{}, errors.New("invalid cache scan result")
	}
	item := scan.Items[0]
	if item.StorageClass != core.StorageDisposable || item.Risk != p.config.Risk || item.RecoveryCost != p.config.RecoveryCost {
		return core.CleanupPlan{}, errors.New("cache classification does not match the allow-listed profile")
	}
	estimated := core.Measurement{Kind: p.config.EstimatedKind}
	if p.config.EstimatedKind == core.MeasurementEstimatedLogical {
		estimated.Bytes = item.Measured.Bytes
	}
	return core.CleanupPlan{ID: p.config.PlanID, ProviderID: p.config.ProviderID, CreatedAt: time.Now().UTC(), Actions: []core.CleanupAction{{
		ID: p.config.ActionID, ItemID: p.config.ItemID, Location: item.Location,
		Risk: p.config.Risk, RecoveryCost: p.config.RecoveryCost, Consequence: p.config.Consequence,
		Observed: item.Measured, Estimated: estimated,
	}}}, nil
}

func (p *Provider) Execute(ctx context.Context, plan core.CleanupPlan, confirmed bool) (core.ExecutionResult, error) {
	if !confirmed {
		return core.ExecutionResult{}, errors.New("cleanup plan requires confirmation")
	}
	if err := p.validatePlan(plan); err != nil {
		return core.ExecutionResult{}, err
	}
	detection, err := p.Detect(ctx)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	if !detection.Detected || !detection.Supported {
		return core.ExecutionResult{}, errors.New("provider is no longer available for cleanup")
	}
	root, err := p.config.ResolveRoot(ctx, p.runner, detection.ExecutablePath)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	if !common.SamePath(root, plan.Actions[0].Location) {
		return core.ExecutionResult{}, errors.New("cache path changed after review; inspect again before cleanup")
	}
	if err := p.config.Clean(ctx, p.runner, detection.ExecutablePath); err != nil {
		return core.ExecutionResult{}, err
	}
	return core.ExecutionResult{PlanID: plan.ID, Executed: true, Destructive: true, Message: fmt.Sprintf("%s cleanup completed; future dependency or binary downloads may be required.", p.config.ItemName)}, nil
}

func (p *Provider) Verify(ctx context.Context, plan core.CleanupPlan) (core.VerificationResult, error) {
	if err := p.validatePlan(plan); err != nil {
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

func (p *Provider) validatePlan(plan core.CleanupPlan) error {
	if plan.ID != p.config.PlanID || plan.ProviderID != p.config.ProviderID || len(plan.Actions) != 1 {
		return errors.New("unrecognised cleanup plan")
	}
	action := plan.Actions[0]
	if action.ID != p.config.ActionID || action.ItemID != p.config.ItemID || action.Risk != p.config.Risk || action.RecoveryCost != p.config.RecoveryCost || action.Estimated.Kind != p.config.EstimatedKind {
		return errors.New("unrecognised cleanup action")
	}
	if _, err := common.ValidateStorageRoot(action.Location, p.config.ItemName); err != nil {
		return fmt.Errorf("invalid cleanup target: %w", err)
	}
	return nil
}
