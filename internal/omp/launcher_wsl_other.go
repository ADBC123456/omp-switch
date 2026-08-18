//go:build !windows

package omp

import "errors"

// StartWSL is only implemented on Windows: OMP Switch drives WSL through
// powershell.exe, which does not exist on macOS/Linux builds of the app.
func StartWSL(ompCommand string, args []string, distro string, workingDir string) error {
	return errors.New("WSL 启动模式仅支持 Windows")
}
