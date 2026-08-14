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
	StorageDisposable  StorageClass = "Disposable cache"
	StorageRebuildable StorageClass = "Rebuildable artifact"
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

type DockerComposeLabels struct {
	Project string `json:"project,omitempty"`
	Service string `json:"service,omitempty"`
	Network string `json:"network,omitempty"`
	Volume  string `json:"volume,omitempty"`
}

type DockerRelationshipObservation struct {
	Kind      string `json:"kind"`
	Count     uint64 `json:"count"`
	Available bool   `json:"available"`
}

type DockerScopedResource struct {
	ID                string                          `json:"id"`
	Kind              string                          `json:"kind"`
	Name              string                          `json:"name"`
	Scope             string                          `json:"scope"`
	Labels            DockerComposeLabels             `json:"labels"`
	Relationships     []DockerRelationshipObservation `json:"relationships"`
	RelatedResourceID string                          `json:"relatedResourceId,omitempty"`
	Stateful          bool                            `json:"stateful"`
	Risk              RiskLevel                       `json:"risk"`
}

type DockerOwnershipGroup struct {
	Scope     string                 `json:"scope"`
	Project   string                 `json:"project,omitempty"`
	Resources []DockerScopedResource `json:"resources"`
}

type DockerBuildCacheRecord struct {
	ID          string `json:"id"`
	Shared      bool   `json:"shared"`
	Mutable     bool   `json:"mutable"`
	Reclaimable bool   `json:"reclaimable"`
}

type DockerBuilderScope struct {
	Scope          string                   `json:"scope"`
	Name           string                   `json:"name"`
	Count          uint64                   `json:"count"`
	CountAvailable bool                     `json:"countAvailable"`
	SharedCount    uint64                   `json:"sharedCount"`
	Records        []DockerBuildCacheRecord `json:"records"`
	Boundary       string                   `json:"boundary"`
}

type DockerAwareness struct {
	Daemon            DockerDaemonStatus      `json:"daemon"`
	InspectedAt       time.Time               `json:"inspectedAt"`
	Resources         []DockerResourceSummary `json:"resources"`
	OwnershipGroups   []DockerOwnershipGroup  `json:"ownershipGroups"`
	OwnershipComplete bool                    `json:"ownershipComplete"`
	Builder           DockerBuilderScope      `json:"builder"`
	Warnings          []string                `json:"warnings"`
}

const DockerNetworkRemovalProviderID = "docker.network"

type DockerNetworkRemovalPlan struct {
	ID           string    `json:"id"`
	NetworkID    string    `json:"networkId"`
	NetworkName  string    `json:"networkName"`
	Project      string    `json:"project"`
	NetworkLabel string    `json:"networkLabel"`
	Risk         RiskLevel `json:"risk"`
	Consequence  string    `json:"consequence"`
	CreatedAt    time.Time `json:"createdAt"`
}

type DockerNetworkRemovalOutcome struct {
	PlanID                  string          `json:"planId"`
	NetworkID               string          `json:"networkId"`
	NetworkName             string          `json:"networkName"`
	RemovalCommandAttempted bool            `json:"removalCommandAttempted"`
	RemovalCommandCompleted bool            `json:"removalCommandCompleted"`
	VerifiedAbsent          bool            `json:"verifiedAbsent"`
	AwarenessRefreshed      bool            `json:"awarenessRefreshed"`
	HistoryRecorded         bool            `json:"historyRecorded"`
	ReclaimedActual         Measurement     `json:"reclaimedActual"`
	Message                 string          `json:"message"`
	Failure                 string          `json:"failure,omitempty"`
	Awareness               DockerAwareness `json:"awareness"`
}

type WSLDistributionState string

const (
	WSLStateRunning WSLDistributionState = "running"
	WSLStateStopped WSLDistributionState = "stopped"
	WSLStateUnknown WSLDistributionState = "unknown"
)

type WSLVHDXObservation struct {
	Path          string      `json:"path,omitempty"`
	PathAvailable bool        `json:"pathAvailable"`
	PhysicalSize  Measurement `json:"physicalSize"`
	LogicalUsage  Measurement `json:"logicalUsage"`
	Compactable   Measurement `json:"compactable"`
	Message       string      `json:"message"`
}

type WSLDistribution struct {
	Name             string               `json:"name"`
	State            WSLDistributionState `json:"state"`
	Version          uint32               `json:"version"`
	VersionAvailable bool                 `json:"versionAvailable"`
	VHDX             WSLVHDXObservation   `json:"vhdx"`
}

type WSLAwareness struct {
	CLIAvailable   bool              `json:"cliAvailable"`
	Available      bool              `json:"available"`
	ExecutablePath string            `json:"executablePath,omitempty"`
	InspectedAt    time.Time         `json:"inspectedAt"`
	Distributions  []WSLDistribution `json:"distributions"`
	Warnings       []string          `json:"warnings"`
	Message        string            `json:"message"`
}

type ProjectEcosystem string

const (
	EcosystemNode   ProjectEcosystem = "node"
	EcosystemRust   ProjectEcosystem = "rust"
	EcosystemPython ProjectEcosystem = "python"
	EcosystemGradle ProjectEcosystem = "gradle"
	EcosystemMaven  ProjectEcosystem = "maven"
)

type ProjectSkipKind string

const (
	ProjectSkipReparsePoint     ProjectSkipKind = "reparse-point"
	ProjectSkipExcludedMetadata ProjectSkipKind = "excluded-metadata"
	ProjectSkipUnclaimedName    ProjectSkipKind = "unclaimed-generated-name"
	ProjectSkipDepthLimit       ProjectSkipKind = "depth-limit"
	ProjectSkipUnreadable       ProjectSkipKind = "unreadable"
	ProjectSkipExcludedByRule   ProjectSkipKind = "excluded-by-rule"
	ProjectSkipNonRegular       ProjectSkipKind = "non-regular"
)

type ProjectArtifactObservation struct {
	Name         string           `json:"name"`
	Path         string           `json:"path"`
	RelativePath string           `json:"relativePath"`
	Ecosystem    ProjectEcosystem `json:"ecosystem"`
	StorageClass StorageClass     `json:"storageClass"`
	Risk         RiskLevel        `json:"risk"`
	RecoveryCost RecoveryCost     `json:"recoveryCost"`
	Measured     Measurement      `json:"measured"`
	Boundary     string           `json:"boundary"`
}

type ProjectObservation struct {
	Name         string                       `json:"name"`
	Path         string                       `json:"path"`
	RelativePath string                       `json:"relativePath"`
	Ecosystems   []ProjectEcosystem           `json:"ecosystems"`
	Markers      []string                     `json:"markers"`
	Artifacts    []ProjectArtifactObservation `json:"artifacts"`
}

type ProjectSkippedPath struct {
	RelativePath string          `json:"relativePath"`
	Kind         ProjectSkipKind `json:"kind"`
	Reason       string          `json:"reason"`
}

type ProjectDiscovery struct {
	Root         string               `json:"root"`
	RootApproved bool                 `json:"rootApproved"`
	InspectedAt  time.Time            `json:"inspectedAt"`
	Complete     bool                 `json:"complete"`
	Truncated    bool                 `json:"truncated"`
	Projects     []ProjectObservation `json:"projects"`
	Skipped      []ProjectSkippedPath `json:"skipped"`
	Warnings     []string             `json:"warnings"`
	Message      string               `json:"message"`
	Boundary     string               `json:"boundary"`
}

type ProjectExclusionRule struct {
	Rule         string `json:"rule"`
	RelativePath string `json:"relativePath"`
	Matched      bool   `json:"matched"`
}

type ProjectArtifactMeasurement struct {
	Name         string               `json:"name"`
	Path         string               `json:"path"`
	RelativePath string               `json:"relativePath"`
	Ecosystem    ProjectEcosystem     `json:"ecosystem"`
	StorageClass StorageClass         `json:"storageClass"`
	Risk         RiskLevel            `json:"risk"`
	RecoveryCost RecoveryCost         `json:"recoveryCost"`
	Measured     Measurement          `json:"measured"`
	Reclaimable  Measurement          `json:"reclaimable"`
	Files        uint64               `json:"files"`
	Directories  uint64               `json:"directories"`
	Complete     bool                 `json:"complete"`
	Truncated    bool                 `json:"truncated"`
	Skipped      []ProjectSkippedPath `json:"skipped"`
	Boundary     string               `json:"boundary"`
}

type ProjectMeasurement struct {
	Name         string                       `json:"name"`
	Path         string                       `json:"path"`
	RelativePath string                       `json:"relativePath"`
	Root         string                       `json:"root"`
	MeasuredAt   time.Time                    `json:"measuredAt"`
	Artifacts    []ProjectArtifactMeasurement `json:"artifacts"`
	Total        Measurement                  `json:"total"`
	Reclaimable  Measurement                  `json:"reclaimable"`
	Complete     bool                         `json:"complete"`
	Truncated    bool                         `json:"truncated"`
	// Cancelled is true only when an explicit CancelProjectMeasurement request
	// interrupted this pass. It is always false for a natural completion or a
	// timeout, so the UI can distinguish a user-requested stop from either of those.
	Cancelled  bool                   `json:"cancelled"`
	Exclusions []ProjectExclusionRule `json:"exclusions"`
	Warnings   []string               `json:"warnings"`
	Message    string                 `json:"message"`
	Boundary   string                 `json:"boundary"`
}

type Dashboard struct {
	ApplicationName string `json:"applicationName"`
	Stage           string `json:"stage"`
	SafetyMessage   string `json:"safetyMessage"`
}

type HistoryRecord struct {
	ID             string          `json:"id"`
	ProviderID     string          `json:"providerId"`
	PlanID         string          `json:"planId"`
	ReclaimedBytes uint64          `json:"reclaimedBytes"`
	ReclaimedKind  MeasurementKind `json:"reclaimedKind"`
	CreatedAt      time.Time       `json:"createdAt"`
}

type HistoryRecorder interface {
	Append(ctx context.Context, record HistoryRecord) error
}
