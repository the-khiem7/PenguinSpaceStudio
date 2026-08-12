//go:build !windows

package common

import "os/exec"

func configureCommand(_ *exec.Cmd) {}
