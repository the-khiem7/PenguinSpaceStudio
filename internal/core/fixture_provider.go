package core

import (
	"context"
	"errors"
	"sync"
	"time"
)

const fixtureBytes uint64 = 3_145_728

type FixtureProvider struct {
	mu      sync.Mutex
	cleared bool
}

func NewFixtureProvider() *FixtureProvider {
	return &FixtureProvider{}
}

func (p *FixtureProvider) ID() string { return "fixture.cache" }

func (p *FixtureProvider) ExecutionEnabled() bool { return true }

func (p *FixtureProvider) Detect(_ context.Context) (ProviderDetection, error) {
	return ProviderDetection{
		ProviderID: p.ID(),
		Detected:   true,
		Supported:  true,
		Version:    "fixture-1",
		Message:    "In-memory fixture provider is available.",
	}, nil
}

func (p *FixtureProvider) Scan(_ context.Context, detection ProviderDetection) (ScanResult, error) {
	if detection.ProviderID != p.ID() || !detection.Detected || !detection.Supported {
		return ScanResult{}, errors.New("fixture provider is not available for scanning")
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	bytes := fixtureBytes
	if p.cleared {
		bytes = 0
	}

	return ScanResult{
		ProviderID: p.ID(),
		ScannedAt:  time.Now().UTC(),
		Items: []StorageItem{{
			ID:           "fixture-cache",
			Name:         "Fixture cache",
			StorageClass: StorageDisposable,
			Risk:         RiskSafe,
			RecoveryCost: RecoveryInstant,
			Measured:     Measurement{Bytes: bytes, Kind: MeasurementMeasuredLogical},
		}},
	}, nil
}

func (p *FixtureProvider) Plan(scan ScanResult) (CleanupPlan, error) {
	if scan.ProviderID != p.ID() || len(scan.Items) != 1 || scan.Items[0].ID != "fixture-cache" {
		return CleanupPlan{}, errors.New("invalid fixture scan result")
	}
	return CleanupPlan{
		ID:         "fixture-plan",
		ProviderID: scan.ProviderID,
		CreatedAt:  time.Now().UTC(),
		Actions: []CleanupAction{{
			ID:           "fixture-clear",
			ItemID:       "fixture-cache",
			Risk:         RiskSafe,
			RecoveryCost: RecoveryInstant,
			Consequence:  "Clears only in-memory fixture data; no filesystem or tool command runs.",
			Observed:     scan.Items[0].Measured,
			Estimated:    Measurement{Bytes: scan.Items[0].Measured.Bytes, Kind: MeasurementEstimatedLogical},
		}},
	}, nil
}

func (p *FixtureProvider) Execute(_ context.Context, plan CleanupPlan, confirmed bool) (ExecutionResult, error) {
	if !confirmed {
		return ExecutionResult{}, errors.New("cleanup plan requires confirmation")
	}
	if plan.ID != "fixture-plan" || plan.ProviderID != "fixture.cache" {
		return ExecutionResult{}, errors.New("unrecognised fixture plan")
	}

	p.mu.Lock()
	p.cleared = true
	p.mu.Unlock()

	return ExecutionResult{
		PlanID:      plan.ID,
		Executed:    true,
		Destructive: false,
		Message:     "Fixture execution completed without touching the filesystem.",
	}, nil
}

func (p *FixtureProvider) Verify(ctx context.Context, plan CleanupPlan) (VerificationResult, error) {
	detection, err := p.Detect(ctx)
	if err != nil {
		return VerificationResult{}, err
	}
	after, err := p.Scan(ctx, detection)
	if err != nil {
		return VerificationResult{}, err
	}
	return VerificationResult{
		PlanID:          plan.ID,
		MeasuredAfter:   after.Items[0].Measured,
		ReclaimedActual: Measurement{Bytes: fixtureBytes - after.Items[0].Measured.Bytes, Kind: MeasurementMeasuredLogical},
	}, nil
}
