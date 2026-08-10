//go:build windows

package main

import (
	"fmt"
	"os"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/elevation"
	"golang.org/x/sys/windows"
)

type windowsElevationLauncher struct{}

func newElevationLauncher() elevation.Launcher {
	return windowsElevationLauncher{}
}

func (windowsElevationLauncher) Launch(requestID string) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find PenguinSpace executable: %w", err)
	}
	arguments := "--elevated-helper --elevation-request-id " + requestID
	return windows.ShellExecute(
		0,
		windows.StringToUTF16Ptr("runas"),
		windows.StringToUTF16Ptr(executable),
		windows.StringToUTF16Ptr(arguments),
		nil,
		windows.SW_SHOWNORMAL,
	)
}
