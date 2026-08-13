package core

import (
	"context"
	"fmt"
	"time"
)

type Orchestrator struct {
	provider Provider
	history  HistoryRecorder
}

func NewOrchestrator(provider Provider, history HistoryRecorder) *Orchestrator {
	return &Orchestrator{provider: provider, history: history}
}

func (o *Orchestrator) RunFixtureScenario(ctx context.Context) (Scenario, error) {
	detection, err := o.provider.Detect(ctx)
	if err != nil {
		return Scenario{}, err
	}
	scan, err := o.provider.Scan(ctx, detection)
	if err != nil {
		return Scenario{}, err
	}
	plan, err := o.provider.Plan(scan)
	if err != nil {
		return Scenario{}, err
	}
	execution, err := o.provider.Execute(ctx, plan, true)
	if err != nil {
		return Scenario{}, err
	}
	verification, err := o.provider.Verify(ctx, plan)
	if err != nil {
		return Scenario{}, err
	}

	if err := o.history.Append(ctx, HistoryRecord{
		ID:             fmt.Sprintf("fixture-%d", time.Now().UnixNano()),
		ProviderID:     scan.ProviderID,
		PlanID:         plan.ID,
		ReclaimedBytes: verification.ReclaimedActual.Bytes,
		ReclaimedKind:  verification.ReclaimedActual.Kind,
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		return Scenario{}, err
	}

	return Scenario{Scan: scan, Plan: plan, Execution: execution, Verification: verification}, nil
}
