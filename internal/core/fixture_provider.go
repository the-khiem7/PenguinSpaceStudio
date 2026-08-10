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

func (p *FixtureProvider) Scan(_ context.Context) ScanResult {
	p.mu.Lock()
	defer p.mu.Unlock()

	bytes := fixtureBytes
	if p.cleared {
		bytes = 0
	}

	return ScanResult{
		ProviderID: "fixture.cache",
		ScannedAt:  time.Now().UTC(),
		Items: []StorageItem{{
			ID:           "fixture-cache",
			Name:         "Fixture cache",
			StorageClass: StorageDisposable,
			Risk:         RiskSafe,
			RecoveryCost: RecoveryInstant,
			Measured:     Measurement{Bytes: bytes},
		}},
	}
}

func (p *FixtureProvider) Plan(scan ScanResult) CleanupPlan {
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
			Estimated:    scan.Items[0].Measured,
		}},
	}
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

func (p *FixtureProvider) Verify(ctx context.Context, plan CleanupPlan) VerificationResult {
	after := p.Scan(ctx)
	return VerificationResult{
		PlanID:          plan.ID,
		MeasuredAfter:   after.Items[0].Measured,
		ReclaimedActual: Measurement{Bytes: fixtureBytes - after.Items[0].Measured.Bytes},
	}
}
