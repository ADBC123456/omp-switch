//go:build windows

package omp

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestStartWSLChainDeliversPayloadAsSingleArgv runs the real PowerShell helper
// (the same exec the app uses) with wsl.exe swapped for a probe executable and
// verifies the bash -lic payload arrives at wsl.exe as ONE argv token with all
// quotes intact. Skipped unless OMP_SWITCH_PROBE_EXE points at a probe that
// writes os.Args[1:] to <dir>/argv2.txt, one per line.
func TestStartWSLChainDeliversPayloadAsSingleArgv(t *testing.T) {
	probe := os.Getenv("OMP_SWITCH_PROBE_EXE")
	if probe == "" {
		t.Skip("OMP_SWITCH_PROBE_EXE not set")
	}
	outFile := filepath.Join(filepath.Dir(probe), "argv2.txt")
	if err := os.Remove(outFile); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	command := wslPowerShellCommand("omp", []string{"--model", "deepseek/deepseek-v4-flash"}, "Ubuntu-26.04", "/home/user/project")
	command = strings.ReplaceAll(command, "'wsl.exe'", "'"+probe+"'")
	_ = os.Remove(filepath.Join(filepath.Dir(probe), "dump.ps1"))

	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createNoWindowFlag}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("powershell helper failed: %v: %s", err, output)
	}
	// The probe is fire-and-forget; give it a moment to write.
	time.Sleep(700 * time.Millisecond)
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("probe did not run: %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(data)), "\n")
	want := []string{"-d", "Ubuntu-26.04", "--", "bash", "-lic", "export PATH=\"$HOME/.local/bin:$PATH\"; cd '/home/user/project' && 'omp' '--model' 'deepseek/deepseek-v4-flash' || { echo; echo 'OMP 启动失败，按回车键关闭窗口...'; read _; }"}
	if len(args) != len(want) {
		t.Fatalf("argv = %q, want %q", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("argv[%d] = %q, want %q\nfull argv = %q", i, args[i], want[i], args)
		}
	}
}
