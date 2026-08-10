package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/elevation"
)

func runElevatedHelper(arguments []string) (bool, error) {
	if len(arguments) == 0 || arguments[0] != "--elevated-helper" {
		return false, nil
	}
	if len(arguments) != 3 || arguments[1] != "--elevation-request-id" {
		return true, errors.New("invalid elevated helper arguments")
	}

	dataDir, err := applicationDataDir()
	if err != nil {
		return true, fmt.Errorf("resolve application data directory: %w", err)
	}
	store := elevation.NewStore(filepath.Join(dataDir, "PenguinSpace", "elevation"))
	if _, err := store.RunM1Probe(context.Background(), arguments[2]); err != nil {
		return true, fmt.Errorf("run elevated helper: %w", err)
	}
	return true, nil
}
