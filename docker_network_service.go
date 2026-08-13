package main

import (
	"context"
	"time"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/dockerinventory"
)

func (s *AppService) InspectDockerNetworkRemoval(networkID string) (core.DockerNetworkRemovalPlan, error) {
	s.dockerMu.Lock()
	defer s.dockerMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return s.dockerRemovalControllerLocked().Inspect(ctx, networkID)
}

func (s *AppService) ExecuteDockerNetworkRemoval(planID string, confirmed bool) (core.DockerNetworkRemovalOutcome, error) {
	s.dockerMu.Lock()
	defer s.dockerMu.Unlock()

	executionContext, cancelExecution := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelExecution()
	refreshContext, cancelRefresh := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancelRefresh()
	outcome, err := s.dockerRemovalControllerLocked().ExecuteWithRefresh(executionContext, refreshContext, planID, confirmed)
	if err != nil {
		return core.DockerNetworkRemovalOutcome{}, err
	}

	if !outcome.VerifiedAbsent {
		return outcome, nil
	}

	historyContext, cancelHistory := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelHistory()
	outcome = dockerinventory.RecordNetworkRemovalOutcome(historyContext, s.history, outcome, time.Now())
	return outcome, nil
}

func (s *AppService) dockerRemovalControllerLocked() *dockerinventory.NetworkRemovalController {
	if s.dockerRemoval == nil {
		s.dockerRemoval = dockerinventory.NewNetworkRemovalController(s.dockerInspector)
	}
	return s.dockerRemoval
}
