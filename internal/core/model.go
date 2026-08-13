package core

import (
	"context"
	"time"
)

type RiskLevel string

const (
	RiskSafe   RiskLevel = "Safe"
	RiskReview RiskLevel = "Review"
	RiskDanger RiskLevel = "Danger"
)

type RecoveryCost string

const (
	RecoveryInstant   RecoveryCost = "Instant"
	RecoveryDownload  RecoveryCost = "Download"
	RecoveryRebuild   RecoveryCost = "Rebuild"
	RecoveryStateLoss RecoveryCost = "State Loss"
)

type StorageClass string

const (
	StorageDisposable StorageClass = "Disposable cache"
)

type MeasurementKind string

const (
	MeasurementMeasuredLogical  MeasurementKind = "measured-logical"
	MeasurementEstimatedLogical MeasurementKind = "estimated-logical"
	MeasurementMeasuredPhysical MeasurementKind = "measured-physical"
	MeasurementUnavailable      MeasurementKind = "unavailable"
)

type Measurement struct {
	Bytes uint64          `json:"bytes"`
	Kind  MeasurementKind `json:"kind"`
}

type StorageItem struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	StorageClass StorageClass `json:"storageClass"`
	Risk         RiskLevel    `json:"risk"`
	RecoveryCost RecoveryCost `json:"recoveryCost"`
	Measured     Measurement  `json:"measured"`
	Location     string       `json:"location,omitempty"`
}

type ProviderDetection struct {
	ProviderID         string `json:"providerId"`
	Detected           bool   `json:"detected"`
	Supported          bool   `json:"supported"`
	NeedsConfiguration bool   `json:"needsConfiguration"`
	Version            string `json:"version,omitempty"`
	ExecutablePath     string `json:"executablePath,omitempty"`
	Message            string `json:"message"`
}

type ProviderAvailabilityStatus string

const (
	ProviderAvailable          ProviderAvailabilityStatus = "available"
	ProviderNeedsConfiguration ProviderAvailabilityStatus = "needs-configuration"
	ProviderUnavailable        ProviderAvailabilityStatus = "unavailable"
	ProviderNotApplicable      ProviderAvailabilityStatus = "not-applicable"
	ProviderWorkspaceRequired  ProviderAvailabilityStatus = "workspace-required"
)

type ProviderAvailability struct {
	ProviderID      string                     `json:"providerId"`
	Status          ProviderAvailabilityStatus `json:"status"`
	WorkspaceScoped bool                       `json:"workspaceScoped"`
	Detection       ProviderDetection          `json:"detection"`
	Message         string                     `json:"message"`
}

type ScanResult struct {
	ProviderID string        `json:"providerId"`
	ScannedAt  time.Time     `json:"scannedAt"`
	Items      []StorageItem `json:"items"`
}

type CleanupAction struct {
	ID           string       `json:"id"`
	ItemID       string       `json:"itemId"`
	Location     string       `json:"location,omitempty"`
	Risk         RiskLevel    `json:"risk"`
	RecoveryCost RecoveryCost `json:"recoveryCost"`
	Consequence  string       `json:"consequence"`
	Observed     Measurement  `json:"observed"`
	Estimated    Measurement  `json:"estimated"`
}

type CleanupPlan struct {
	ID         string          `json:"id"`
	ProviderID string          `json:"providerId"`
	CreatedAt  time.Time       `json:"createdAt"`
	Actions    []CleanupAction `json:"actions"`
}

type ExecutionResult struct {
	PlanID      string `json:"planId"`
	Executed    bool   `json:"executed"`
	Destructive bool   `json:"destructive"`
	Message     string `json:"message"`
}

type VerificationResult struct {
	PlanID          string      `json:"planId"`
	MeasuredAfter   Measurement `json:"measuredAfter"`
	ReclaimedActual Measurement `json:"reclaimedActual"`
}

type Scenario struct {
	Scan         ScanResult         `json:"scan"`
	Plan         CleanupPlan        `json:"plan"`
	Execution    ExecutionResult    `json:"execution"`
	Verification VerificationResult `json:"verification"`
}

type ProviderInspection struct {
	Detection        ProviderDetection `json:"detection"`
	Scan             ScanResult        `json:"scan"`
	Plan             CleanupPlan       `json:"plan"`
	ExecutionEnabled bool              `json:"executionEnabled"`
}

type ProviderCleanupOutcome struct {
	Execution    ExecutionResult    `json:"execution"`
	Verification VerificationResult `json:"verification"`
}

type WorkspaceRoot struct {
	Path string `json:"path"`
}

type WorkspaceScopedProvider interface {
	Provider
	SetWorkspaceRoot(root string) error
}

type WorkspaceDiscoverableProvider interface {
	WorkspaceScopedProvider
	WorkspaceApplicable(root string) error
}

type Provider interface {
	ID() string
	ExecutionEnabled() bool
	Detect(ctx context.Context) (ProviderDetection, error)
	Scan(ctx context.Context, detection ProviderDetection) (ScanResult, error)
	Plan(scan ScanResult) (CleanupPlan, error)
	Execute(ctx context.Context, plan CleanupPlan, confirmed bool) (ExecutionResult, error)
	Verify(ctx context.Context, plan CleanupPlan) (VerificationResult, error)
}

type DockerDaemonStatus struct {
	CLIAvailable    bool   `json:"cliAvailable"`
	Available       bool   `json:"available"`
	ExecutablePath  string `json:"executablePath,omitempty"`
	Version         string `json:"version,omitempty"`
	OperatingSystem string `json:"operatingSystem,omitempty"`
	Architecture    string `json:"architecture,omitempty"`
	Message         string `json:"message"`
}

type DockerResourceSummary struct {
	Kind           string      `json:"kind"`
	Name           string      `json:"name"`
	Count          uint64      `json:"count"`
	CountAvailable bool        `json:"countAvailable"`
	Size           Measurement `json:"size"`
	Reclaimable    Measurement `json:"reclaimable"`
	Stateful       bool        `json:"stateful"`
	Boundary       string      `json:"boundary"`
}

type DockerAwareness struct {
	Daemon      DockerDaemonStatus      `json:"daemon"`
	InspectedAt time.Time               `json:"inspectedAt"`
	Resources   []DockerResourceSummary `json:"resources"`
	Warnings    []string                `json:"warnings"`
}

type Dashboard struct {
	ApplicationName string `json:"applicationName"`
	Stage           string `json:"stage"`
	SafetyMessage   string `json:"safetyMessage"`
}

type HistoryRecord struct {
	ID             string    `json:"id"`
	ProviderID     string    `json:"providerId"`
	PlanID         string    `json:"planId"`
	ReclaimedBytes uint64    `json:"reclaimedBytes"`
	CreatedAt      time.Time `json:"createdAt"`
}

type HistoryRecorder interface {
	Append(ctx context.Context, record HistoryRecord) error
}
