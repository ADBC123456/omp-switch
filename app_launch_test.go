package main

import (
	"testing"

	"ompswitch/internal/config"
	"ompswitch/internal/omp"
	"ompswitch/internal/provider"
)

func TestExecuteLaunchOMPChoosesDirectoryAndStartsSelectedModel(t *testing.T) {
	app := mutationTestApp(t, config.SwitchConfig{
		Providers:          []provider.Config{mutationProvider("gateway", "secret", "model")},
		SelectedProviderID: "gateway",
		ModelRoles:         map[string]string{},
		Settings:           config.AppSettings{OMPCommand: "omp", WorkingDir: "old-directory"},
	})

	var selectedDefault string
	app.selectLaunchDir = func(defaultDirectory string) (string, error) {
		selectedDefault = defaultDirectory
		return t.TempDir(), nil
	}
	app.startManagedOMP = func(preview omp.LaunchPreview, workingDir string) error {
		if preview.Executable == "" || workingDir == "" {
			t.Fatalf("preview=%#v workingDir=%q", preview, workingDir)
		}
		return nil
	}

	if err := app.ExecuteLaunchOMP("gateway", "model"); err != nil {
		t.Fatal(err)
	}
	if selectedDefault != "old-directory" {
		t.Fatalf("selected default=%q", selectedDefault)
	}
}

func TestExecuteLaunchOMPCancelDoesNotStartOrPersistDirectory(t *testing.T) {
	app := mutationTestApp(t, config.SwitchConfig{
		Providers:          []provider.Config{mutationProvider("gateway", "secret", "model")},
		SelectedProviderID: "gateway",
		ModelRoles:         map[string]string{},
		Settings:           config.AppSettings{OMPCommand: "omp", WorkingDir: "keep-me"},
	})
	app.selectLaunchDir = func(string) (string, error) { return "", nil }
	app.startManagedOMP = func(omp.LaunchPreview, string) error {
		t.Fatal("cancelled launch started a process")
		return nil
	}

	if err := app.ExecuteLaunchOMP("gateway", "model"); err != nil {
		t.Fatal(err)
	}
	cfg, err := app.service.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Settings.WorkingDir != "keep-me" {
		t.Fatalf("working directory changed to %q", cfg.Settings.WorkingDir)
	}
}
