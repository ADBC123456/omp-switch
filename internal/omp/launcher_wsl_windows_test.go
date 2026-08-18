//go:build windows

package omp

import (
	"os"
	"strings"
	"testing"
)

func TestWSLPowerShellCommandQuotesLoginShellPayload(t *testing.T) {
	got := wslPowerShellCommand("omp", []string{"--model", "deepseek/deepseek-v4-flash"}, "Ubuntu-26.04", "/home/user/project")
	expectedDir := os.Getenv("WINDIR")
	if expectedDir == "" {
		expectedDir = "C:\\Windows"
	}
	want := `Start-Process -FilePath 'wsl.exe' -ArgumentList '-d Ubuntu-26.04 -- bash -lic "export PATH=\"$HOME/.local/bin:$PATH\"; cd ''/home/user/project'' && ''omp'' ''--model'' ''deepseek/deepseek-v4-flash'' || { echo; echo ''OMP 启动失败，按回车键关闭窗口...''; read _; }"' -WorkingDirectory '` + expectedDir + `'`
	if got != want {
		t.Fatalf("command = %q\nwant %q", got, want)
	}
}

func TestWSLPowerShellCommandWorkingDirMapping(t *testing.T) {
	got := wslPowerShellCommand("omp", []string{"--model", "p/m"}, "Ubuntu", "/mnt/e/woker")
	if !strings.Contains(got, "-WorkingDirectory 'E:\\woker'") {
		t.Fatalf("working dir not mapped to Windows path: %s", got)
	}
	if strings.Contains(got, "--cd ") {
		t.Fatalf("unexpected --cd option: %s", got)
	}
}

func TestWSLPowerShellCommandEmptyDistroAndDir(t *testing.T) {
	got := wslPowerShellCommand("omp", []string{"--model", "p/m"}, "", "")
	for _, fragment := range []string{"-d ", "--cd "} {
		if strings.Contains(got, fragment) {
			t.Fatalf("unexpected %q in command: %s", fragment, got)
		}
	}
	prefix := `Start-Process -FilePath 'wsl.exe' -ArgumentList '-- bash -lic "export PATH=\"$HOME/.local/bin:$PATH\"; ''omp'' ''--model'' ''p/m'' || { echo; echo ''OMP 启动失败，按回车键关闭窗口...''; read _; }"' -WorkingDirectory '`
	if !strings.HasPrefix(got, prefix) {
		t.Fatalf("payload not a single quoted token: %s", got)
	}
}

func TestWSLHelperWorkingDir(t *testing.T) {
	if got := wslHelperWorkingDir("/mnt/e/woker"); got != "E:\\woker" {
		t.Fatalf("mapped dir = %q", got)
	}
	want := os.Getenv("WINDIR")
	if want == "" {
		want = "C:\\Windows"
	}
	if got := wslHelperWorkingDir("/home/user"); got != want {
		t.Fatalf("fallback dir = %q, want %q", got, want)
	}
}
