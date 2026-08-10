//go:build windows

package main

import (
	"errors"
	"os"
)

func applicationDataDir() (string, error) {
	path := os.Getenv("LOCALAPPDATA")
	if path == "" {
		return "", errors.New("LOCALAPPDATA is unavailable")
	}
	return path, nil
}
