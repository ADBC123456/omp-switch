package config

import (
	"os"
	"path/filepath"
	"testing"

	"ompswitch/internal/paths"
	"ompswitch/internal/provider"
)

func testPaths(t *testing.T) paths.AppPaths {
	t.Helper()
	root := t.TempDir()
	return paths.AppPaths{
		OMPSwitchConfigPath: filepath.Join(root, ".ompswitch", "config.json"),
		OMPModelsPath:       filepath.Join(root, ".omp", "agent", "models.yml"),
		OMPConfigPath:       filepath.Join(root, ".omp", "agent", "config.yml"),
		BackupDir:           filepath.Join(root, ".ompswitch", "backups"),
	}
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadExistingEmptyProvidersStaysEmptyAndIgnoresYAML(t *testing.T) {
	appPaths := testPaths(t)
	writeTestFile(t, appPaths.OMPSwitchConfigPath, []byte(`{"version":2,"providers":[],"selectedProviderId":"stale","modelRoles":{},"settings":{}}`))
	writeTestFile(t, appPaths.OMPModelsPath, []byte("not: valid-for-models\n"))

	cfg, err := NewService(appPaths).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Providers) != 0 || cfg.SelectedProviderID != "" {
		t.Fatalf("providers=%v selected=%q, want durable empty state", cfg.Providers, cfg.SelectedProviderID)
	}
}

func TestLoadImportsYAMLOnceAndPersistsAppSourceOfTruth(t *testing.T) {
	appPaths := testPaths(t)
	writeTestFile(t, appPaths.OMPModelsPath, []byte("providers:\n  imported:\n    baseUrl: https://example.com/v1\n    apiKey: API_KEY\n    api: openai-completions\n    models:\n      - id: model-a\n"))
	writeTestFile(t, appPaths.OMPConfigPath, []byte("modelRoles:\n  default: imported/model-a\n"))

	service := NewService(appPaths)
	cfg, err := service.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Providers) != 1 || cfg.Providers[0].ID != "imported" || cfg.SelectedProviderID != "imported" {
		t.Fatalf("unexpected import: %+v", cfg)
	}
	if cfg.ModelRoles["default"] != "imported/model-a" {
		t.Fatalf("roles=%v", cfg.ModelRoles)
	}

	writeTestFile(t, appPaths.OMPModelsPath, []byte("invalid after import"))
	reloaded, err := service.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Providers) != 1 || reloaded.Providers[0].ID != "imported" {
		t.Fatalf("later YAML replaced app source of truth: %+v", reloaded.Providers)
	}
}

func TestLoadUsesPresetsOnlyWhenModelsYAMLIsAbsent(t *testing.T) {
	appPaths := testPaths(t)
	cfg, err := NewService(appPaths).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Providers) == 0 {
		t.Fatal("expected first-run presets")
	}
}

func TestNormalizeSettingsSupportsThemeModesAndMigratesDarkMode(t *testing.T) {
	dark := true
	if got := NormalizeSettings(AppSettings{LegacyDarkMode: &dark}); got.Theme != "dark" || got.LegacyDarkMode != nil {
		t.Fatalf("legacy dark mode normalized to %+v", got)
	}
	for _, mode := range []string{"light", "dark", "system"} {
		if got := NormalizeSettings(AppSettings{Theme: mode}); got.Theme != mode {
			t.Fatalf("theme %q normalized to %q", mode, got.Theme)
		}
	}
	if got := NormalizeSettings(AppSettings{Theme: "invalid"}); got.Theme != "system" {
		t.Fatalf("invalid theme normalized to %q", got.Theme)
	}
}

func TestLoadImportsEmptyProviderMapAsDurableEmpty(t *testing.T) {
	appPaths := testPaths(t)
	writeTestFile(t, appPaths.OMPModelsPath, []byte("providers: {}\n"))
	service := NewService(appPaths)
	cfg, err := service.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Providers) != 0 {
		t.Fatalf("providers=%v, want empty", cfg.Providers)
	}
	reloaded, err := service.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Providers) != 0 {
		t.Fatalf("reloaded providers=%v, want durable empty", reloaded.Providers)
	}
}

func TestLoadMalformedYAMLWritesNoAppConfig(t *testing.T) {
	appPaths := testPaths(t)
	writeTestFile(t, appPaths.OMPModelsPath, []byte("providers: [broken\n"))
	if _, err := NewService(appPaths).Load(); err == nil {
		t.Fatal("expected malformed YAML error")
	}
	if _, err := os.Stat(appPaths.OMPSwitchConfigPath); !os.IsNotExist(err) {
		t.Fatalf("app config was written after invalid YAML: %v", err)
	}
}

func TestNewAppStateRedactsProvidersAndProjectsPaths(t *testing.T) {
	appPaths := testPaths(t)
	cfg := SwitchConfig{
		Providers:          []provider.Config{{ID: "p", Name: "P", APIKey: "secret", Models: []provider.ModelInfo{{ID: "m"}}}},
		SelectedProviderID: "missing",
		ModelRoles:         map[string]string{"default": "p/m"},
	}
	state := NewAppState("1.2.3", cfg, appPaths, []string{"ready"})
	if state.Providers[0].APIKey != "" || !state.Providers[0].HasAPIKey {
		t.Fatalf("provider secret was not redacted: %+v", state.Providers[0])
	}
	if state.SelectedProviderID != "p" {
		t.Fatalf("selected=%q, want fixed fallback p", state.SelectedProviderID)
	}
	if state.Paths.OMPSwitchConfigPath != appPaths.OMPSwitchConfigPath || state.Paths.OMPModelsPath != appPaths.OMPModelsPath || state.Paths.OMPConfigPath != appPaths.OMPConfigPath || state.Paths.OMPSessionsDir != appPaths.OMPSessionsDir || state.Paths.BackupDir != appPaths.BackupDir {
		t.Fatalf("paths=%+v", state.Paths)
	}
}

func TestProviderHelpersMaintainSelectedProviderFallback(t *testing.T) {
	cfg := SwitchConfig{Providers: []provider.Config{{ID: "a"}, {ID: "b"}}, SelectedProviderID: "a"}
	cfg.UpsertProvider(provider.Config{ID: "renamed"}, "a")
	if cfg.SelectedProviderID != "renamed" {
		t.Fatalf("selected=%q after rename", cfg.SelectedProviderID)
	}
	cfg.DeleteProvider("renamed")
	if cfg.SelectedProviderID != "b" {
		t.Fatalf("selected=%q after delete", cfg.SelectedProviderID)
	}
}
