package main

import (
	"context"
	"path/filepath"
	"time"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/elevation"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/history"
)

type AppService struct {
	orchestrator *core.Orchestrator
	history      *history.Store
	elevation    *elevation.Controller
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
	}, nil
}

func (s *AppService) Close() error {
	return s.history.Close()
}

func (s *AppService) Dashboard() core.Dashboard {
	return core.Dashboard{
		ApplicationName: "PenguinSpace",
		Stage:           "M1 fixture and elevation probe ready",
		SafetyMessage:   "No filesystem or tool command is executed by the fixture or elevation probe.",
	}
}

func (s *AppService) RunFixtureScenario() (core.Scenario, error) {
	return s.orchestrator.RunFixtureScenario(context.Background())
}

func (s *AppService) RecentHistory() ([]core.HistoryRecord, error) {
	return s.history.List(context.Background(), 20)
}

func (s *AppService) StartElevationProbe() (elevation.OperationStatus, error) {
	return s.elevation.StartM1Probe()
}

func (s *AppService) CancelElevationProbe() (elevation.OperationStatus, error) {
	return s.elevation.Cancel()
}

func (s *AppService) ElevationStatus() elevation.OperationStatus {
	return s.elevation.Status()
}
