package core

import (
	"context"
	"fmt"
	"time"
)

type Orchestrator struct {
	provider *FixtureProvider
	history  HistoryRecorder
}

func NewOrchestrator(provider *FixtureProvider, history HistoryRecorder) *Orchestrator {
	return &Orchestrator{provider: provider, history: history}
}

func (o *Orchestrator) RunFixtureScenario(ctx context.Context) (Scenario, error) {
	scan := o.provider.Scan(ctx)
	plan := o.provider.Plan(scan)
	execution, err := o.provider.Execute(ctx, plan, true)
	if err != nil {
		return Scenario{}, err
	}
	verification := o.provider.Verify(ctx, plan)

	if err := o.history.Append(ctx, HistoryRecord{
		ID:             fmt.Sprintf("fixture-%d", time.Now().UnixNano()),
		ProviderID:     scan.ProviderID,
		PlanID:         plan.ID,
		ReclaimedBytes: verification.ReclaimedActual.Bytes,
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		return Scenario{}, err
	}

	return Scenario{Scan: scan, Plan: plan, Execution: execution, Verification: verification}, nil
}
