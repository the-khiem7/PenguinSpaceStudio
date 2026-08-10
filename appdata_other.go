//go:build !windows

package main

import "os"

func applicationDataDir() (string, error) {
	return os.UserConfigDir()
}
