//go:build windows

package omp

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

const consoleProbePathEnv = "OMP_SWITCH_CONSOLE_PROBE_PATH"

func TestManagedCommandChildHasConsole(t *testing.T) {
	if probePath := os.Getenv(consoleProbePathEnv); probePath != "" {
		stdin, err := syscall.GetStdHandle(syscall.STD_INPUT_HANDLE)
		if err == nil {
			var mode uint32
			err = syscall.GetConsoleMode(stdin, &mode)
		}
		result := "console"
		if err != nil {
			result = fmt.Sprintf("no console: %v", err)
		}
		if writeErr := os.WriteFile(probePath, []byte(result), 0o600); writeErr != nil {
			panic(writeErr)
		}
		return
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	probePath := filepath.Join(t.TempDir(), "console-probe.txt")
	preview := LaunchPreview{
		Executable: executable,
		Arguments:  []string{"-test.run=^TestManagedCommandChildHasConsole$"},
	}
	cmd := managedCommand(preview, t.TempDir())
	cmd.Env = append(os.Environ(), consoleProbePathEnv+"="+probePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("managed command failed: %v: %s", err, output)
	}
	result, err := os.ReadFile(probePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != "console" {
		t.Fatalf("interactive child started without a console: %s", result)
	}
}
