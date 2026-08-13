package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/elevation"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/history"
	bunprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/bun"
	cypressprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/cypress"
	npmprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/npm"
	nugetprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/nuget"
	pnpmprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/pnpm"
	uvprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/uv"
	yarnprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/yarn"
)

type AppService struct {
	orchestrator  *core.Orchestrator
	history       *history.Store
	elevation     *elevation.Controller
	providerMu    sync.Mutex
	providers     map[string]core.Provider
	providerPlans map[string]core.CleanupPlan
}

func NewAppService() (*AppService, error) {
	dataDir, err := applicationDataDir()
	if err != nil {
		return nil, err
	}

	store, err := history.Open(filepath.Join(dataDir, "PenguinSpace", "penguinspace.db"))
	if err != nil {
		return nil, err
	}

	elevationStore := elevation.NewStore(filepath.Join(dataDir, "PenguinSpace", "elevation"))
	return &AppService{
		orchestrator: core.NewOrchestrator(core.NewFixtureProvider(), store),
		history:      store,
		elevation:    elevation.NewController(elevationStore, newElevationLauncher(), 30*time.Second),
		providers: map[string]core.Provider{
			bunprovider.ProviderID:     bunprovider.NewSystemProvider(),
			cypressprovider.ProviderID: cypressprovider.NewSystemProvider(),
			nugetprovider.ProviderID:   nugetprovider.NewSystemProvider(),
			npmprovider.ProviderID:     npmprovider.NewSystemProvider(),
			pnpmprovider.ProviderID:    pnpmprovider.NewSystemProvider(),
			uvprovider.ProviderID:      uvprovider.NewSystemProvider(),
			yarnprovider.ProviderID:    yarnprovider.NewSystemProvider(),
		},
		providerPlans: make(map[string]core.CleanupPlan),
	}, nil
}

func (s *AppService) Close() error {
	return s.history.Close()
}

func (s *AppService) Dashboard() core.Dashboard {
	return core.Dashboard{
		ApplicationName: "PenguinSpace",
		Stage:           "M2 Bun, npm, pnpm, uv, Yarn Classic, NuGet, and Cypress providers ready",
		SafetyMessage:   "The fixture is no-op; real cleanup requires an inspected backend plan and explicit confirmation.",
	}
}

func (s *AppService) RunFixtureScenario() (core.Scenario, error) {
	return s.orchestrator.RunFixtureScenario(context.Background())
}

func (s *AppService) RecentHistory() ([]core.HistoryRecord, error) {
	return s.history.List(context.Background(), 20)
}

func (s *AppService) InspectDeveloperProvider(providerID string) (core.ProviderInspection, error) {
	provider, err := s.provider(providerID)
	if err != nil {
		return core.ProviderInspection{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	inspection, err := core.InspectProvider(ctx, provider)
	if err != nil {
		return core.ProviderInspection{}, err
	}
	s.providerMu.Lock()
	s.providerPlans[providerID] = inspection.Plan
	s.providerMu.Unlock()
	return inspection, nil
}

func (s *AppService) ExecuteDeveloperProvider(providerID string, confirmed bool) (core.ProviderCleanupOutcome, error) {
	if !confirmed {
		return core.ProviderCleanupOutcome{}, errors.New("provider cleanup requires explicit confirmation")
	}
	provider, err := s.provider(providerID)
	if err != nil {
		return core.ProviderCleanupOutcome{}, err
	}
	s.providerMu.Lock()
	plan := s.providerPlans[providerID]
	delete(s.providerPlans, providerID)
	s.providerMu.Unlock()
	if plan.ID == "" {
		return core.ProviderCleanupOutcome{}, errors.New("inspect the provider again before cleanup")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	execution, err := provider.Execute(ctx, plan, true)
	if err != nil {
		return core.ProviderCleanupOutcome{}, err
	}
	verification, err := provider.Verify(ctx, plan)
	if err != nil {
		return core.ProviderCleanupOutcome{}, err
	}
	if err := s.history.Append(ctx, core.HistoryRecord{
		ID:             fmt.Sprintf("provider-%d", time.Now().UnixNano()),
		ProviderID:     plan.ProviderID,
		PlanID:         plan.ID,
		ReclaimedBytes: verification.ReclaimedActual.Bytes,
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		return core.ProviderCleanupOutcome{}, err
	}
	return core.ProviderCleanupOutcome{Execution: execution, Verification: verification}, nil
}

func (s *AppService) provider(providerID string) (core.Provider, error) {
	provider, ok := s.providers[providerID]
	if !ok {
		return nil, fmt.Errorf("unknown developer provider %q", providerID)
	}
	return provider, nil
}

func (s *AppService) StartElevationProbe(mode elevation.ProbeMode) (elevation.OperationStatus, error) {
	return s.elevation.StartM1Probe(mode)
}

func (s *AppService) CancelElevationProbe() (elevation.OperationStatus, error) {
	return s.elevation.Cancel()
}

func (s *AppService) ElevationStatus() elevation.OperationStatus {
	return s.elevation.Status()
}
