//go:build windows

package common

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func configureCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNoWindow,
		HideWindow:    true,
	}
}
