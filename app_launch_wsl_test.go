package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ompswitch/internal/config"
	"ompswitch/internal/omp"
	"ompswitch/internal/provider"
)

func TestExecuteLaunchOMPWSLPicksDirAndMapsToWSLPath(t *testing.T) {
	app := mutationTestApp(t, config.SwitchConfig{
		Providers:          []provider.Config{mutationProvider("gw", "secret", "model")},
		SelectedProviderID: "gw",
		ModelRoles:         map[string]string{},
		Settings: config.AppSettings{
			OMPCommand: "C:\\wrong\\omp.exe", // Windows path must be ignored in WSL mode
			WorkingDir: "/home/user/project", // Linux path; used as picker default only
			LaunchMode: "wsl",
			WSLDistro:  "Ubuntu",
		},
	})
	var got wslLaunchConfig
	var pickerDefault string
	app.selectLaunchDir = func(defaultDir string) (string, error) {
		pickerDefault = defaultDir
		return "E:\\woker\\project", nil
	}
	app.startWSLOMP = func(cfg wslLaunchConfig) error {
		got = cfg
		return nil
	}

	if err := app.ExecuteLaunchOMP("gw", "model"); err != nil {
		t.Fatal(err)
	}
	if got.ompCommand != "omp" {
		t.Fatalf("ompCommand=%q, want omp", got.ompCommand)
	}
	if got.workingDir != "/mnt/e/woker/project" {
		t.Fatalf("workingDir=%q, want /mnt/e/woker/project", got.workingDir)
	}
	if got.distro != "Ubuntu" {
		t.Fatalf("distro=%q", got.distro)
	}
	if len(got.args) != 2 || got.args[0] != "--model" || !strings.Contains(got.args[1], "gw/model") {
		t.Fatalf("args=%v", got.args)
	}
	// Persisted working dir must be the mapped Linux path.
	cfg, err := app.service.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Settings.WorkingDir != "/mnt/e/woker/project" {
		t.Fatalf("persisted workingDir=%q", cfg.Settings.WorkingDir)
	}
	// /home/... has no Windows counterpart, so the picker default is empty.
	if pickerDefault != "" {
		t.Fatalf("picker default=%q, want empty for WSL-internal path", pickerDefault)
	}
}

func TestExecuteLaunchOMPWSLMntPathBecomesPickerDefault(t *testing.T) {
	app := mutationTestApp(t, config.SwitchConfig{
		Providers:          []provider.Config{mutationProvider("gw", "secret", "model")},
		SelectedProviderID: "gw",
		ModelRoles:         map[string]string{},
		Settings: config.AppSettings{
			OMPCommand: "omp",
			WorkingDir: "/mnt/e/woker",
			LaunchMode: "wsl",
			WSLDistro:  "Ubuntu",
		},
	})
	var pickerDefault string
	app.selectLaunchDir = func(defaultDir string) (string, error) {
		pickerDefault = defaultDir
		return "E:\\woker", nil
	}
	app.startWSLOMP = func(cfg wslLaunchConfig) error { return nil }
	if err := app.ExecuteLaunchOMP("gw", "model"); err != nil {
		t.Fatal(err)
	}
	if pickerDefault != "E:\\woker" {
		t.Fatalf("picker default=%q, want E:\\woker", pickerDefault)
	}
}

func TestExecuteLaunchOMPWSLCancelAborts(t *testing.T) {
	app := mutationTestApp(t, config.SwitchConfig{
		Providers:          []provider.Config{mutationProvider("gw", "secret", "model")},
		SelectedProviderID: "gw",
		ModelRoles:         map[string]string{},
		Settings: config.AppSettings{
			OMPCommand: "omp",
			WorkingDir: "/home/user",
			LaunchMode: "wsl",
			WSLDistro:  "Ubuntu",
		},
	})
	app.selectLaunchDir = func(string) (string, error) { return "", nil }
	app.startWSLOMP = func(cfg wslLaunchConfig) error {
		t.Fatal("cancelled launch started a process")
		return nil
	}
	err := app.ExecuteLaunchOMP("gw", "model")
	if err == nil || !strings.Contains(err.Error(), "取消") {
		t.Fatalf("expected cancel error, got %v", err)
	}
}

func TestExecuteLaunchOMPNativeStillUsesPickerAndWindowsCommand(t *testing.T) {
	app := mutationTestApp(t, config.SwitchConfig{
		Providers:          []provider.Config{mutationProvider("gw", "secret", "model")},
		SelectedProviderID: "gw",
		ModelRoles:         map[string]string{},
		Settings: config.AppSettings{
			OMPCommand: "omp",
			WorkingDir: "old",
			LaunchMode: "native",
		},
	})
	var picked string
	app.selectLaunchDir = func(defaultDir string) (string, error) {
		picked = defaultDir
		return "C:\\Work", nil
	}
	var previewGot omp.LaunchPreview
	var dirGot string
	app.startManagedOMP = func(preview omp.LaunchPreview, dir string) error {
		previewGot, dirGot = preview, dir
		return nil
	}
	if err := app.ExecuteLaunchOMP("gw", "model"); err != nil {
		t.Fatal(err)
	}
	if picked != "old" {
		t.Fatalf("picker default=%q, want old", picked)
	}
	if dirGot != "C:\\Work" {
		t.Fatalf("native workingDir=%q, want picked value", dirGot)
	}
	if previewGot.Executable != "omp" {
		t.Fatalf("executable=%q", previewGot.Executable)
	}
}
func TestContinueSessionWSLMapsSessionPathToLinux(t *testing.T) {
	app := mutationTestApp(t, config.SwitchConfig{
		Providers:          []provider.Config{mutationProvider("gw", "secret", "model")},
		SelectedProviderID: "gw",
		ModelRoles:         map[string]string{},
		Settings: config.AppSettings{
			OMPCommand: `C:\wrong\omp.exe`, // ignored in WSL mode
			WorkingDir: "/home/user",
			LaunchMode: "wsl",
			WSLDistro:  "Ubuntu",
		},
	})
	// A real session file so sessions.Find resolves a path. The file lives on
	// the Windows host (temp dir); WSL mode must map it to /mnt/... for omp.
	project := filepath.Join(app.paths.OMPSessionsDir, "proj")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	sessionFile := filepath.Join(project, "abc123.jsonl")
	record := `{"type":"session","id":"abc123","cwd":"/home/user","timestamp":"2024-01-01T00:00:00Z"}
`
	if err := os.WriteFile(sessionFile, []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}

	var got wslLaunchConfig
	app.startWSLOMP = func(cfg wslLaunchConfig) error {
		got = cfg
		return nil
	}
	if err := app.ContinueSession("abc123"); err != nil {
		t.Fatal(err)
	}
	if got.ompCommand != "omp" {
		t.Fatalf("ompCommand=%q, want omp", got.ompCommand)
	}
	if len(got.args) != 2 || got.args[0] != "--resume" {
		t.Fatalf("args=%v", got.args)
	}
	if !strings.HasPrefix(got.args[1], "/mnt/") || strings.Contains(got.args[1], "\\") {
		t.Fatalf("resume path not mapped to a Linux path: %q", got.args[1])
	}
	if got.workingDir != "/home/user" {
		t.Fatalf("workingDir=%q", got.workingDir)
	}
	if got.distro != "Ubuntu" {
		t.Fatalf("distro=%q", got.distro)
	}
}
