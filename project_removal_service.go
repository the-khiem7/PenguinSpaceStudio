package main

import (
	"context"
	"errors"
	"time"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/projectinventory"
)

// InspectProjectArtifactRemoval reviews one exact claimed generated directory below
// one exact discovered project for removal. Both paths must already appear in a
// fresh discovery pass; there is no way to name an arbitrary filesystem path here.
func (s *AppService) InspectProjectArtifactRemoval(projectPath, artifactPath string) (core.ProjectArtifactRemovalPlan, error) {
	s.providerMu.Lock()
	root := s.workspaceRoot
	s.providerMu.Unlock()
	if root == "" {
		return core.ProjectArtifactRemovalPlan{}, errors.New("approve a workspace root before reviewing project artifact removal")
	}

	s.projectMu.Lock()
	defer s.projectMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.projectRemovalControllerLocked().Inspect(ctx, root, projectPath, artifactPath)
}

// ExecuteProjectArtifactRemoval consumes the retained plan by its exact ID after
// explicit confirmation. It removes only this one filesystem-fallback artifact and
// verifies absence before recording history.
func (s *AppService) ExecuteProjectArtifactRemoval(planID string, confirmed bool) (core.ProjectArtifactRemovalOutcome, error) {
	s.projectMu.Lock()
	defer s.projectMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	outcome, err := s.projectRemovalControllerLocked().Execute(ctx, planID, confirmed)
	if err != nil {
		return core.ProjectArtifactRemovalOutcome{}, err
	}
	if !outcome.VerifiedAbsent {
		return outcome, nil
	}

	historyContext, cancelHistory := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelHistory()
	outcome = projectinventory.RecordArtifactRemovalOutcome(historyContext, s.history, outcome, time.Now())
	return outcome, nil
}

func (s *AppService) projectRemovalControllerLocked() *projectinventory.RemovalController {
	if s.projectRemoval == nil {
		s.projectRemoval = projectinventory.NewRemovalController(s.projectInspect)
	}
	return s.projectRemoval
}
