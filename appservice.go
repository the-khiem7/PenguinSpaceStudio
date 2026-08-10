package main

import (
	"context"
	"path/filepath"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/history"
)

type AppService struct {
	orchestrator *core.Orchestrator
	history      *history.Store
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

	return &AppService{
		orchestrator: core.NewOrchestrator(core.NewFixtureProvider(), store),
		history:      store,
	}, nil
}

func (s *AppService) Close() error {
	return s.history.Close()
}

func (s *AppService) Dashboard() core.Dashboard {
	return core.Dashboard{
		ApplicationName: "PenguinSpace",
		Stage:           "M1 fixture ready",
		SafetyMessage:   "No filesystem or tool command is executed by this fixture.",
	}
}

func (s *AppService) RunFixtureScenario() (core.Scenario, error) {
	return s.orchestrator.RunFixtureScenario(context.Background())
}

func (s *AppService) RecentHistory() ([]core.HistoryRecord, error) {
	return s.history.List(context.Background(), 20)
}
