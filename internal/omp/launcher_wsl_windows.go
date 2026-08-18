//go:build windows

package omp

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// createNoWindowFlag is CREATE_NO_WINDOW: keeps the helper powershell.exe
// invisible when the GUI app spawns it. Same flag launcher_windows.go uses
// for the native launch.
const createNoWindowFlag = 0x08000000

// StartWSL launches OMP inside a WSL distro in its own visible console
// window. It returns once the PowerShell helper has handed the launch to
// wsl.exe — the OMP console keeps running independently.
//
// The launch is a single PowerShell command:
//
//	Start-Process -FilePath 'wsl.exe' -ArgumentList '<tokens>' -WorkingDirectory '<dir>'
//
// and the working directory is applied through three cooperating layers:
//
//  1. Start-Process -WorkingDirectory: wsl.exe starts from the Windows path
//     that maps to the requested WSL directory (E:... for /mnt/e/...), so WSL
//     begins there on every WSL version — no --cd option required.
//  2. bash -lic: a login AND interactive shell, so every user PATH addition
//     (~/.profile, ~/.bash_profile, ~/.bashrc, /etc/profile) applies. A plain
//     "sh -lc" misses ~/.bashrc-only PATH lines (dash login never sources
//     ~/.bashrc) and OMP then fails with "command not found".
//  3. cd '<dir>' inside the shell command: works for any WSL path
//     (/mnt/..., /home/...), independent of the WSL version, and fails
//     visibly when the directory does not exist.
//
// Values embedded in the shell command are single-quoted (see
// buildWSLShellCommand), which is safe against both shell injection and
// Windows command-line mangling.
func StartWSL(ompCommand string, args []string, distro string, workingDir string) error {
	if err := validateWSLInput(ompCommand, args, distro, workingDir); err != nil {
		return err
	}
	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", wslPowerShellCommand(ompCommand, args, distro, workingDir))
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createNoWindowFlag}
	// The helper exits right after Start-Process returns (no -Wait), so
	// CombinedOutput surfaces launch errors (e.g. wsl.exe not found) without
	// blocking on the OMP session itself.
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("启动 WSL OMP: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// wslPowerShellCommand builds the PowerShell snippet that opens wsl.exe in a
// new console window. ArgumentList receives one pre-quoted string (PowerShell
// 5.1 does not quote array elements itself); each token is escaped with
// windowsCommandLineArgument so the bash -lic payload stays a single argv
// element on wsl.exe's command line.
func wslPowerShellCommand(ompCommand string, args []string, distro string, workingDir string) string {
	var tokens []string
	if distro != "" {
		tokens = append(tokens, "-d", distro)
	}
	tokens = append(tokens, "--", "bash", "-lic", buildWSLShellCommand(ompCommand, args, workingDir))

	quoted := make([]string, len(tokens))
	for i, token := range tokens {
		quoted[i] = windowsCommandLineArgument(token)
	}
	return "Start-Process -FilePath " + powerShellQuote("wsl.exe") +
		" -ArgumentList " + powerShellQuote(strings.Join(quoted, " ")) +
		" -WorkingDirectory " + powerShellQuote(wslHelperWorkingDir(workingDir))
}

// wslHelperWorkingDir picks the Windows directory handed to Start-Process so
// wsl.exe (and therefore the WSL session) starts in the requested directory
// even on WSL versions without --cd. The /mnt/... mapping of workingDir is
// used when available; otherwise WINDIR is pinned to keep CreateProcess away
// from UNC current directories.
func wslHelperWorkingDir(workingDir string) string {
	if win := WSLToWindowsPath(workingDir); win != "" {
		return win
	}
	windir := os.Getenv("WINDIR")
	if windir == "" {
		windir = "C:\\Windows"
	}
	return windir
}
