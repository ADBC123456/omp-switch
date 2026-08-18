package main

import (
	"os"
	"path/filepath"
	"testing"

	"ompswitch/internal/config"
	"ompswitch/internal/provider"
	"ompswitch/internal/paths"
)

// TestUpdateSettingsSwitchesProvidersToCustomPaths verifies that saving
// custom OMP paths re-imports providers from the new models.yml, so the UI
// immediately shows the target installation's models instead of the old one.
func TestUpdateSettingsSwitchesProvidersToCustomPaths(t *testing.T) {
	app := mutationTestApp(t, config.SwitchConfig{
		Providers:          []provider.Config{mutationProvider("local", "secret", "local-model")},
		SelectedProviderID: "local",
		ModelRoles:         map[string]string{},
		Settings: config.AppSettings{
			OMPCommand: "omp", WorkingDir: "C:\\work",
			LaunchMode: "wsl", WSLDistro: "Ubuntu",
		},
	})

	// Simulate a WSL install's models.yml + config.yml in a separate dir.
	wslRoot := t.TempDir()
	wslModels := filepath.Join(wslRoot, "models.yml")
	wslConfig := filepath.Join(wslRoot, "config.yml")
	if err := os.MkdirAll(filepath.Dir(wslModels), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wslModels, []byte("providers:\n  wsl-provider:\n    baseUrl: https://wsl.example/v1\n    api: openai-responses\n    apiKey: secret\n    models:\n      - id: wsl-model\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wslConfig, []byte("model:\n  default: wsl-provider/wsl-model\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	state, err := app.UpdateSettings(config.AppSettings{
		OMPCommand: "omp", WorkingDir: "/home/user", Theme: "system",
		LaunchMode: "wsl", WSLDistro: "Ubuntu",
		CustomPaths: config.CustomPaths{
			OMPModelsPath: wslModels, OMPConfigPath: wslConfig, OMPSessionsDir: filepath.Join(wslRoot, "sessions"),
		},
	})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if len(state.Providers) != 1 || state.Providers[0].ID != "wsl-provider" {
		t.Fatalf("providers after switch = %#v, want wsl-provider", state.Providers)
	}
	if state.Providers[0].SelectedModelID != "wsl-model" && len(state.Providers[0].Models) == 0 {
		t.Fatalf("model list missing: %#v", state.Providers[0])
	}
	if state.Paths.OMPModelsPath != wslModels {
		t.Fatalf("state.Paths.OMPModelsPath = %q, want %q", state.Paths.OMPModelsPath, wslModels)
	}
	// App paths/service must point at the new OMP install.
	if app.paths.OMPModelsPath != wslModels {
		t.Fatalf("app.paths.OMPModelsPath = %q, want %q", app.paths.OMPModelsPath, wslModels)
	}
	imported, err := app.service.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := imported.ProviderByID("wsl-provider"); err != nil {
		t.Fatalf("wsl-provider not persisted to config.json: %v", err)
	}
}

// TestUpdateSettingsSamePathsDoesNotReimport checks that settings changes
// which leave custom paths untouched do not clobber the provider list.
func TestUpdateSettingsSamePathsDoesNotReimport(t *testing.T) {
	app := mutationTestApp(t, config.SwitchConfig{
		Providers:          []provider.Config{mutationProvider("keep", "secret", "m")},
		SelectedProviderID: "keep",
		ModelRoles:         map[string]string{},
		Settings: config.AppSettings{OMPCommand: "omp", WorkingDir: "C:\\work", LaunchMode: "native"},
	})
	state, err := app.UpdateSettings(config.AppSettings{
		OMPCommand: "omp", WorkingDir: "C:\\new", Theme: "light", LaunchMode: "native",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Providers) != 1 || state.Providers[0].ID != "keep" {
		t.Fatalf("providers changed unexpectedly: %#v", state.Providers)
	}
	if state.Settings.WorkingDir != "C:\\new" {
		t.Fatalf("workingDir = %q", state.Settings.WorkingDir)
	}
}

// TestUpdateSettingsRejectsUnreadableCustomModelsPath ensures a bad custom
// path surfaces an error instead of silently keeping stale providers.
func TestUpdateSettingsRejectsUnreadableCustomModelsPath(t *testing.T) {
	app := mutationTestApp(t, config.SwitchConfig{
		Providers:          []provider.Config{mutationProvider("keep", "secret", "m")},
		SelectedProviderID: "keep",
		ModelRoles:         map[string]string{},
		Settings:           config.AppSettings{OMPCommand: "omp", WorkingDir: "C:\\work", LaunchMode: "native"},
	})
	missing := filepath.Join(t.TempDir(), "nope", "models.yml")
	_, err := app.UpdateSettings(config.AppSettings{
		OMPCommand: "omp", WorkingDir: "C:\\work", Theme: "system", LaunchMode: "native",
		CustomPaths: config.CustomPaths{OMPModelsPath: missing, OMPConfigPath: filepath.Join(t.TempDir(), "c.yml"), OMPSessionsDir: filepath.Join(t.TempDir(), "s")},
	})
	if err == nil {
		t.Fatal("expected error for missing custom models.yml")
	}
	if app.paths.OMPModelsPath != missing {
		t.Fatalf("paths should still update to the configured path even on import failure: %q", app.paths.OMPModelsPath)
	}
}

var _ = paths.AppPaths{} // keep import referenced for future tests