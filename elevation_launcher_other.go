//go:build !windows

package main

import (
	"errors"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/elevation"
)

type unsupportedElevationLauncher struct{}

func newElevationLauncher() elevation.Launcher {
	return unsupportedElevationLauncher{}
}

func (unsupportedElevationLauncher) Launch(string) error {
	return errors.New("Windows elevation is unavailable on this platform")
}
