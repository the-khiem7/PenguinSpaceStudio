package projectcleanup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/providers/common"
)

type VersionParser func(output string) (version string, supported bool, err error)
type FindExecutable func(workspaceRoot string, runner common.CommandRunner) (string, error)
type WorkspaceValidator func(workspaceRoot string) error
type Arguments func(workspaceRoot string) []string

type Config struct {
	ProviderID, ItemID, ItemName, PlanID, ActionID         string
	ExecutableName                                         string
	FindExecutable                                         FindExecutable
	VersionArguments, CleanupArguments, CleanupEnvironment Arguments
	ParseVersion                                           VersionParser
	ValidateWorkspace                                      WorkspaceValidator
	ResolveTarget                                          func(workspaceRoot string) string
	SupportedMessage, UnsupportedMessage, Consequence      string
}

type Provider struct {
	config        Config
	runner        common.CommandRunner
	mu            sync.RWMutex
	workspaceRoot string
}

func NewProvider(config Config, runner common.CommandRunner) *Provider {
	return &Provider{config: config, runner: runner}
}
func (p *Provider) ID() string             { return p.config.ProviderID }
func (p *Provider) ExecutionEnabled() bool { return true }

func (p *Provider) SetWorkspaceRoot(root string) error {
	validated, err := common.ValidateWorkspaceRoot(root)
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.workspaceRoot = validated
	p.mu.Unlock()
	return nil
}

func (p *Provider) workspace() (string, error) {
	p.mu.RLock()
	root := p.workspaceRoot
	p.mu.RUnlock()
	if root == "" {
		return "", errors.New("select an approved workspace root before inspecting this provider")
	}
	return common.ValidateWorkspaceRoot(root)
}

func (p *Provider) WorkspaceApplicable(root string) error {
	validated, err := common.ValidateWorkspaceRoot(root)
	if err != nil {
		return err
	}
	return p.config.ValidateWorkspace(validated)
}

func (p *Provider) Detect(ctx context.Context) (core.ProviderDetection, error) {
	root, err := p.workspace()
	if err != nil {
		return core.ProviderDetection{ProviderID: p.config.ProviderID, Message: err.Error()}, nil
	}
	executable, err := p.config.FindExecutable(root, p.runner)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return core.ProviderDetection{ProviderID: p.config.ProviderID, Message: fmt.Sprintf("%s was not found on PATH.", p.config.ExecutableName)}, nil
		}
		if errors.Is(err, os.ErrNotExist) {
			return core.ProviderDetection{ProviderID: p.config.ProviderID, Message: fmt.Sprintf("%s was not found in the approved workspace.", p.config.ExecutableName)}, nil
		}
		return core.ProviderDetection{}, fmt.Errorf("locate %s: %w", p.config.ItemName, err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return core.ProviderDetection{}, fmt.Errorf("resolve %s executable: %w", p.config.ItemName, err)
	}
	if err := p.config.ValidateWorkspace(root); err != nil {
		return core.ProviderDetection{ProviderID: p.config.ProviderID, Detected: true, ExecutablePath: executable, Message: err.Error()}, nil
	}
	output, err := p.runner.Run(ctx, executable, p.config.VersionArguments(root)...)
	if err != nil {
		return core.ProviderDetection{}, fmt.Errorf("read %s version: %w", p.config.ItemName, err)
	}
	version, supported, err := p.config.ParseVersion(output)
	if err != nil {
		return core.ProviderDetection{ProviderID: p.config.ProviderID, Detected: true, ExecutablePath: executable, Message: fmt.Sprintf("%s was detected, but its version could not be safely classified.", p.config.ItemName)}, nil
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
	root, err := p.workspace()
	if err != nil {
		return core.ScanResult{}, err
	}
	target, err := common.ValidateWorkspaceTarget(root, p.config.ResolveTarget(root), p.config.ItemName)
	if err != nil {
		return core.ScanResult{}, err
	}
	bytes, err := common.MeasureDirectory(ctx, target)
	if err != nil {
		return core.ScanResult{}, fmt.Errorf("measure %s: %w", p.config.ItemName, err)
	}
	return core.ScanResult{ProviderID: p.config.ProviderID, ScannedAt: time.Now().UTC(), Items: []core.StorageItem{{
		ID: p.config.ItemID, Name: p.config.ItemName, StorageClass: core.StorageDisposable,
		Risk: core.RiskReview, RecoveryCost: core.RecoveryRebuild,
		Measured: core.Measurement{Bytes: bytes, Kind: core.MeasurementMeasuredLogical}, Location: target,
	}}}, nil
}

func (p *Provider) Plan(scan core.ScanResult) (core.CleanupPlan, error) {
	if scan.ProviderID != p.config.ProviderID || len(scan.Items) != 1 || scan.Items[0].ID != p.config.ItemID {
		return core.CleanupPlan{}, errors.New("invalid project cleanup scan")
	}
	item := scan.Items[0]
	if item.Risk != core.RiskReview || item.RecoveryCost != core.RecoveryRebuild {
		return core.CleanupPlan{}, errors.New("project cleanup classification does not match the allow-listed profile")
	}
	return core.CleanupPlan{ID: p.config.PlanID, ProviderID: p.config.ProviderID, CreatedAt: time.Now().UTC(), Actions: []core.CleanupAction{{
		ID: p.config.ActionID, ItemID: p.config.ItemID, Location: item.Location, Risk: core.RiskReview, RecoveryCost: core.RecoveryRebuild,
		Consequence: p.config.Consequence, Observed: item.Measured, Estimated: core.Measurement{Bytes: item.Measured.Bytes, Kind: core.MeasurementEstimatedLogical},
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
	root, err := p.workspace()
	if err != nil {
		return core.ExecutionResult{}, err
	}
	target, err := common.ValidateWorkspaceTarget(root, p.config.ResolveTarget(root), p.config.ItemName)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	if !common.SamePath(target, plan.Actions[0].Location) {
		return core.ExecutionResult{}, errors.New("project target changed after review; inspect again before cleanup")
	}
	var cleanupErr error
	var environment []string
	if p.config.CleanupEnvironment != nil {
		environment = p.config.CleanupEnvironment(root)
	}
	if len(environment) > 0 {
		environmentRunner, ok := p.runner.(common.EnvironmentCommandRunner)
		if !ok {
			return core.ExecutionResult{}, errors.New("provider runner does not support the required cleanup environment")
		}
		_, cleanupErr = environmentRunner.RunWithEnv(ctx, detection.ExecutablePath, environment, p.config.CleanupArguments(root)...)
	} else {
		_, cleanupErr = p.runner.Run(ctx, detection.ExecutablePath, p.config.CleanupArguments(root)...)
	}
	if cleanupErr != nil {
		return core.ExecutionResult{}, fmt.Errorf("clean %s: %w", p.config.ItemName, cleanupErr)
	}
	return core.ExecutionResult{PlanID: plan.ID, Executed: true, Destructive: true, Message: fmt.Sprintf("%s cleanup completed; generated project artifacts will be rebuilt.", p.config.ItemName)}, nil
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
		return errors.New("unrecognised project cleanup plan")
	}
	action := plan.Actions[0]
	if action.ID != p.config.ActionID || action.ItemID != p.config.ItemID || action.Risk != core.RiskReview || action.RecoveryCost != core.RecoveryRebuild || action.Estimated.Kind != core.MeasurementEstimatedLogical {
		return errors.New("unrecognised project cleanup action")
	}
	root, err := p.workspace()
	if err != nil {
		return err
	}
	if _, err := common.ValidateWorkspaceTarget(root, action.Location, p.config.ItemName); err != nil {
		return fmt.Errorf("invalid project cleanup target: %w", err)
	}
	return nil
}

func RequireRegularWorkspaceFile(root, relative, label string) error {
	path, err := common.ValidateWorkspaceTarget(root, filepath.Join(root, relative), label)
	if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("approved workspace does not contain %s", relative)
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular workspace file", relative)
	}
	return nil
}

func RequireWorkspaceDirectory(root, relative, label string) error {
	path, err := common.ValidateWorkspaceTarget(root, filepath.Join(root, relative), label)
	if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("approved workspace does not contain %s", relative)
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%s is not a regular workspace directory", relative)
	}
	return nil
}
