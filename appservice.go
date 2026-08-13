package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	dockerinventory "github.com/the-khiem7/PenguinSpaceStudio/internal/dockerinventory"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/elevation"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/history"
	bunprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/bun"
	cargoprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/cargo"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/providers/common"
	cypressprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/cypress"
	gradleprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/gradle"
	mavenprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/maven"
	npmprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/npm"
	nugetprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/nuget"
	playwrightprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/playwright"
	pnpmprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/pnpm"
	uvprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/uv"
	yarnprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/yarn"
)

type issuedProviderPlan struct {
	plan          core.CleanupPlan
	workspaceRoot string
}

type AppService struct {
	orchestrator    *core.Orchestrator
	history         *history.Store
	elevation       *elevation.Controller
	dockerInspector *dockerinventory.Inspector
	providerMu      sync.Mutex
	providers       map[string]core.Provider
	providerOrder   []string
	providerPlans   map[string]issuedProviderPlan
	workspaceRoot   string
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
		orchestrator:    core.NewOrchestrator(core.NewFixtureProvider(), store),
		history:         store,
		elevation:       elevation.NewController(elevationStore, newElevationLauncher(), 30*time.Second),
		dockerInspector: dockerinventory.NewSystemInspector(),
		providers: map[string]core.Provider{
			bunprovider.ProviderID:        bunprovider.NewSystemProvider(),
			cargoprovider.ProviderID:      cargoprovider.NewSystemProvider(),
			cypressprovider.ProviderID:    cypressprovider.NewSystemProvider(),
			gradleprovider.ProviderID:     gradleprovider.NewSystemProvider(),
			mavenprovider.ProviderID:      mavenprovider.NewSystemProvider(),
			nugetprovider.ProviderID:      nugetprovider.NewSystemProvider(),
			npmprovider.ProviderID:        npmprovider.NewSystemProvider(),
			playwrightprovider.ProviderID: playwrightprovider.NewSystemProvider(),
			pnpmprovider.ProviderID:       pnpmprovider.NewSystemProvider(),
			uvprovider.ProviderID:         uvprovider.NewSystemProvider(),
			yarnprovider.ProviderID:       yarnprovider.NewSystemProvider(),
		},
		providerOrder: []string{
			bunprovider.ProviderID,
			npmprovider.ProviderID,
			pnpmprovider.ProviderID,
			uvprovider.ProviderID,
			yarnprovider.ProviderID,
			nugetprovider.ProviderID,
			cypressprovider.ProviderID,
			cargoprovider.ProviderID,
			gradleprovider.ProviderID,
			mavenprovider.ProviderID,
			playwrightprovider.ProviderID,
		},
		providerPlans: make(map[string]issuedProviderPlan),
	}, nil
}

func (s *AppService) Close() error {
	return s.history.Close()
}

func (s *AppService) Dashboard() core.Dashboard {
	return core.Dashboard{
		ApplicationName: "PenguinSpace",
		Stage:           "M3.1 read-only Docker awareness; cleanup remains disabled",
		SafetyMessage:   "Docker resources are observed independently; no prune or volume mutation is available.",
	}
}

func (s *AppService) RunFixtureScenario() (core.Scenario, error) {
	return s.orchestrator.RunFixtureScenario(context.Background())
}

func (s *AppService) RecentHistory() ([]core.HistoryRecord, error) {
	return s.history.List(context.Background(), 20)
}

func (s *AppService) InspectDockerAwareness() core.DockerAwareness {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return s.dockerInspector.Inspect(ctx)
}

func (s *AppService) SetWorkspaceRoot(path string) (core.WorkspaceRoot, error) {
	root, err := common.ValidateWorkspaceRoot(path)
	if err != nil {
		return core.WorkspaceRoot{}, err
	}
	s.providerMu.Lock()
	s.workspaceRoot = root
	s.providerMu.Unlock()
	return core.WorkspaceRoot{Path: root}, nil
}

func (s *AppService) DiscoverDeveloperProviders() []core.ProviderAvailability {
	s.providerMu.Lock()
	defer s.providerMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	availability := make([]core.ProviderAvailability, 0, len(s.providerOrder))
	for _, providerID := range s.providerOrder {
		provider := s.providers[providerID]
		result := core.ProviderAvailability{ProviderID: providerID}
		if scoped, ok := provider.(core.WorkspaceScopedProvider); ok {
			result.WorkspaceScoped = true
			if s.workspaceRoot == "" {
				result.Status = core.ProviderWorkspaceRequired
				result.Message = "Choose an approved workspace root to discover project providers."
				availability = append(availability, result)
				continue
			}
			if discoverable, ok := provider.(core.WorkspaceDiscoverableProvider); ok {
				if err := discoverable.WorkspaceApplicable(s.workspaceRoot); err != nil {
					result.Status = core.ProviderNotApplicable
					result.Message = err.Error()
					availability = append(availability, result)
					continue
				}
			}
			if err := scoped.SetWorkspaceRoot(s.workspaceRoot); err != nil {
				result.Status = core.ProviderUnavailable
				result.Message = fmt.Sprintf("Could not configure the approved workspace: %v", err)
				availability = append(availability, result)
				continue
			}
		}

		detection, err := provider.Detect(ctx)
		if err != nil {
			result.Status = core.ProviderUnavailable
			result.Message = fmt.Sprintf("Could not check provider availability: %v", err)
			availability = append(availability, result)
			continue
		}
		result.Detection = detection
		result.Message = detection.Message
		switch {
		case detection.NeedsConfiguration:
			result.Status = core.ProviderNeedsConfiguration
		case detection.Detected && detection.Supported:
			result.Status = core.ProviderAvailable
		default:
			result.Status = core.ProviderUnavailable
		}
		availability = append(availability, result)
	}
	return availability
}

func (s *AppService) InspectDeveloperProvider(providerID string) (core.ProviderInspection, error) {
	provider, err := s.provider(providerID)
	if err != nil {
		return core.ProviderInspection{}, err
	}

	s.providerMu.Lock()
	defer s.providerMu.Unlock()
	workspaceRoot, err := s.configureWorkspaceLocked(provider)
	if err != nil {
		return core.ProviderInspection{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	inspection, err := core.InspectProvider(ctx, provider)
	if err != nil {
		return core.ProviderInspection{}, err
	}
	s.providerPlans[providerID] = issuedProviderPlan{plan: inspection.Plan, workspaceRoot: workspaceRoot}
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
	issued := s.providerPlans[providerID]
	delete(s.providerPlans, providerID)
	if issued.plan.ID == "" {
		s.providerMu.Unlock()
		return core.ProviderCleanupOutcome{}, errors.New("inspect the provider again before cleanup")
	}
	if issued.workspaceRoot != "" && !common.SamePath(issued.workspaceRoot, s.workspaceRoot) {
		s.providerMu.Unlock()
		return core.ProviderCleanupOutcome{}, errors.New("workspace root changed after review; inspect again before cleanup")
	}
	if _, err := s.configureWorkspaceLocked(provider); err != nil {
		s.providerMu.Unlock()
		return core.ProviderCleanupOutcome{}, err
	}
	s.providerMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	execution, err := provider.Execute(ctx, issued.plan, true)
	if err != nil {
		return core.ProviderCleanupOutcome{}, err
	}
	verification, err := provider.Verify(ctx, issued.plan)
	if err != nil {
		return core.ProviderCleanupOutcome{}, err
	}
	if err := s.history.Append(ctx, core.HistoryRecord{
		ID:             fmt.Sprintf("provider-%d", time.Now().UnixNano()),
		ProviderID:     issued.plan.ProviderID,
		PlanID:         issued.plan.ID,
		ReclaimedBytes: verification.ReclaimedActual.Bytes,
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		return core.ProviderCleanupOutcome{}, err
	}
	return core.ProviderCleanupOutcome{Execution: execution, Verification: verification}, nil
}

func (s *AppService) configureWorkspaceLocked(provider core.Provider) (string, error) {
	scoped, ok := provider.(core.WorkspaceScopedProvider)
	if !ok {
		return "", nil
	}
	if s.workspaceRoot == "" {
		return "", errors.New("select an approved workspace root before inspecting this provider")
	}
	if err := scoped.SetWorkspaceRoot(s.workspaceRoot); err != nil {
		return "", err
	}
	return s.workspaceRoot, nil
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
