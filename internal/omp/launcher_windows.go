//go:build windows

package omp

import (
	"fmt"
	"os/exec"
	"syscall"
)

func managedCommand(preview LaunchPreview, workingDir string) *exec.Cmd {
	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", powershellCommand(preview, workingDir))
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x08000000}
	return cmd
}

func terminateProcessTree(pid int) error {
	output, err := exec.Command("taskkill.exe", "/PID", fmt.Sprint(pid), "/T", "/F").CombinedOutput()
	if err != nil {
		return fmt.Errorf("停止 OMP 进程树: %w: %s", err, output)
	}
	return nil
}
