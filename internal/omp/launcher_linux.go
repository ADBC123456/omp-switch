//go:build linux

package omp

import (
	"os/exec"
	"syscall"
)

func managedCommand(preview LaunchPreview, workingDir string) *exec.Cmd {
	arguments := make([]string, 0, 2+len(preview.Arguments))
	arguments = append(arguments, "-e", preview.Executable)
	arguments = append(arguments, preview.Arguments...)
	cmd := exec.Command("x-terminal-emulator", arguments...)
	if workingDir != "" {
		cmd.Dir = workingDir
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

func terminateProcessTree(pid int) error { return syscall.Kill(-pid, syscall.SIGTERM) }
