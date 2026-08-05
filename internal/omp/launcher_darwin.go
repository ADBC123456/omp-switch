//go:build darwin

package omp

import (
	"fmt"
	"os/exec"
)

func managedCommand(preview LaunchPreview, workingDir string) *exec.Cmd {
	cmd := exec.Command(preview.Executable, preview.Arguments...)
	if workingDir != "" {
		cmd.Dir = workingDir
	}
	return cmd
}

func terminateProcessTree(pid int) error { return exec.Command("kill", "-TERM", fmt.Sprint(pid)).Run() }
