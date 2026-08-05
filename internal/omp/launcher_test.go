package omp

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestBuildLaunchUsesFixedSafeArguments(t *testing.T) {
	preview, err := BuildLaunch(`C:\Program Files\omp.exe`, "gateway", "team/model:low")
	if err != nil {
		t.Fatal(err)
	}
	if preview.Executable != `C:\Program Files\omp.exe` {
		t.Fatalf("executable = %q", preview.Executable)
	}
	want := []string{"--model", "gateway/team/model:low"}
	if !reflect.DeepEqual(preview.Arguments, want) {
		t.Fatalf("arguments = %#v, want %#v", preview.Arguments, want)
	}
}

func TestBuildResumeLaunchUsesSessionFile(t *testing.T) {
	preview, err := BuildResumeLaunch("omp", `C:\Users\me\.omp\agent\sessions\project\session.jsonl`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--resume", `C:\Users\me\.omp\agent\sessions\project\session.jsonl`}
	if !reflect.DeepEqual(preview.Arguments, want) {
		t.Fatalf("arguments = %#v, want %#v", preview.Arguments, want)
	}
}

func TestBuildLaunchRejectsInvalidValues(t *testing.T) {
	for _, test := range []struct{ command, provider, model string }{
		{"", "p", "m"}, {"omp\nrm", "p", "m"}, {"omp", "", "m"}, {"omp", "p", ""},
	} {
		if _, err := BuildLaunch(test.command, test.provider, test.model); err == nil {
			t.Fatalf("BuildLaunch(%q, %q, %q) succeeded", test.command, test.provider, test.model)
		}
	}
}

func TestPowerShellCommandStartsInteractiveChildWithQuotedValues(t *testing.T) {
	preview := LaunchPreview{Executable: `C:\Program Files\o'mp.exe`, Arguments: []string{"--model", `pro/ma"in; Remove-Item x`}}
	command := powershellCommand(preview, `C:\work dir`)
	for _, want := range []string{"Start-Process -FilePath 'C:\\Program Files\\o''mp.exe'", `-ArgumentList '--model "pro/ma\"in; Remove-Item x"'`, `-WorkingDirectory 'C:\work dir'`, "-Wait -PassThru"} {
		if !strings.Contains(command, want) {
			t.Fatalf("PowerShell command %q does not contain %q", command, want)
		}
	}
}

func TestTerminalCommandQuotesAppleScriptValues(t *testing.T) {
	preview := LaunchPreview{Executable: `C:\Program Files\o'mp.exe`, Arguments: []string{"--model", `pro/ma"in; Remove-Item x`}}
	appleScript := appleScriptCommand(preview)
	if !strings.Contains(appleScript, `o'\"'\"'mp.exe`) || !strings.Contains(appleScript, `\"`) {
		t.Fatalf("unsafe AppleScript command: %s", appleScript)
	}
}

func TestResolveExecutableUsesLocalOMPInstallOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows fallback")
	}
	root := t.TempDir()
	directory := filepath.Join(root, "omp")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(directory, "omp.exe")
	if err := os.WriteFile(want, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "")
	t.Setenv("LOCALAPPDATA", root)
	got, err := resolveExecutable("omp")
	if err != nil || got != want {
		t.Fatalf("resolveExecutable = %q, %v; want %q", got, err, want)
	}
}
