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

type Measurement struct {
	Bytes uint64 `json:"bytes"`
}

type StorageItem struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	StorageClass StorageClass `json:"storageClass"`
	Risk         RiskLevel    `json:"risk"`
	RecoveryCost RecoveryCost `json:"recoveryCost"`
	Measured     Measurement  `json:"measured"`
}

type ScanResult struct {
	ProviderID string        `json:"providerId"`
	ScannedAt  time.Time     `json:"scannedAt"`
	Items      []StorageItem `json:"items"`
}

type CleanupAction struct {
	ID           string       `json:"id"`
	ItemID       string       `json:"itemId"`
	Risk         RiskLevel    `json:"risk"`
	RecoveryCost RecoveryCost `json:"recoveryCost"`
	Consequence  string       `json:"consequence"`
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
