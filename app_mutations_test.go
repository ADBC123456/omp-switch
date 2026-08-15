package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"ompswitch/internal/config"
	"ompswitch/internal/paths"
	"ompswitch/internal/provider"
)

func TestProviderMutationAllocatesDeterministicSuffixAndRedactsKey(t *testing.T) {
	app := mutationTestApp(t, config.SwitchConfig{
		Providers:          []provider.Config{mutationProvider("gateway", "key-one", "model")},
		SelectedProviderID: "gateway",
		ModelRoles:         map[string]string{},
		Settings:           config.AppSettings{OMPCommand: "omp", WorkingDir: t.TempDir()},
	})

	result, err := app.CreateProvider(provider.SaveInput{
		ID: "gateway", BaseURL: "https://example.com/v1", APIKey: "key-two",
		API: "openai-responses", HeaderMode: "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.FinalProviderID != "gateway-2" || !result.Adjusted {
		t.Fatalf("result = %#v", result)
	}
	if got := result.State.Providers[1]; got.APIKey != "" || !got.HasAPIKey {
		t.Fatalf("provider view leaked or lost key state: %#v", got)
	}
}

func TestProviderUpdateBlankKeyPreservesStoredSecretAndRewritesExactRoles(t *testing.T) {
	app := mutationTestApp(t, config.SwitchConfig{
		Providers:          []provider.Config{mutationProvider("old", "secret", "team/model")},
		SelectedProviderID: "old",
		ModelRoles:         map[string]string{"default": "old/team/model:high", "task": "custom/value"},
		Settings:           config.AppSettings{OMPCommand: "omp", WorkingDir: t.TempDir()},
	})

	result, err := app.UpdateProvider("old", provider.SaveInput{
		ID: "new", BaseURL: "https://example.com/v1", API: "openai-responses", HeaderMode: "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.State.ModelRoles["default"] != "new/team/model:high" || result.State.ModelRoles["task"] != "custom/value" {
		t.Fatalf("roles = %#v", result.State.ModelRoles)
	}
	stored, err := app.service.Load()
	if err != nil {
		t.Fatal(err)
	}
	providerConfig, err := stored.ProviderByID("new")
	if err != nil {
		t.Fatal(err)
	}
	if providerConfig.APIKey != "secret" {
		t.Fatalf("stored key = %q", providerConfig.APIKey)
	}
}

func TestModelMutationRejectsConflictAndPropagatesExactSelector(t *testing.T) {
	configured := mutationProvider("gateway", "secret", "one")
	configured.Models = append(configured.Models, provider.ModelInfo{ID: "two"})
	app := mutationTestApp(t, config.SwitchConfig{
		Providers: []provider.Config{configured}, SelectedProviderID: "gateway",
		ModelRoles: map[string]string{"default": "gateway/one:low", "task": "external/custom"},
		Settings:   config.AppSettings{OMPCommand: "omp", WorkingDir: t.TempDir()},
	})

	if _, err := app.SaveModel("gateway", "one", provider.ModelInfo{ID: "two"}); err == nil || !strings.Contains(err.Error(), "two") {
		t.Fatalf("duplicate error = %v", err)
	}
	state, err := app.SaveModel("gateway", "one", provider.ModelInfo{ID: "renamed"})
	if err != nil {
		t.Fatal(err)
	}
	if state.ModelRoles["default"] != "gateway/renamed:low" || state.ModelRoles["task"] != "external/custom" {
		t.Fatalf("roles = %#v", state.ModelRoles)
	}
}
func TestBatchModelDeletionRemovesOnlySelectedModelsAndRoles(t *testing.T) {
	configured := mutationProvider("gateway", "secret", "one")
	configured.Models = append(configured.Models, provider.ModelInfo{ID: "two"}, provider.ModelInfo{ID: "three"})
	configured.SelectedModelID = "two"
	app := mutationTestApp(t, config.SwitchConfig{
		Providers: []provider.Config{configured}, SelectedProviderID: "gateway",
		ModelRoles: map[string]string{"default": "gateway/one:low", "smol": "gateway/two", "task": "gateway/three", "custom": "external/model"},
		Settings:   config.AppSettings{OMPCommand: "omp", WorkingDir: t.TempDir()},
	})

	impact, err := app.GetModelsDeleteImpact("gateway", []string{"one", "two", "one"})
	if err != nil || strings.Join(impact, ",") != "default,smol" {
		t.Fatalf("impact = %v, %v", impact, err)
	}
	if _, err = app.DeleteModels("gateway", []string{"one", "missing"}); err == nil {
		t.Fatal("expected missing model rejection")
	}
	unchanged, err := app.service.Load()
	if err != nil || len(unchanged.Providers[0].Models) != 3 {
		t.Fatalf("failed deletion mutated configuration: %#v, %v", unchanged.Providers[0].Models, err)
	}

	state, err := app.DeleteModels("gateway", []string{"one", "two"})
	if err != nil {
		t.Fatal(err)
	}
	if models := state.Providers[0].Models; len(models) != 1 || models[0].ID != "three" {
		t.Fatalf("models = %#v", models)
	}
	if state.Providers[0].SelectedModelID != "three" {
		t.Fatalf("selected model = %q", state.Providers[0].SelectedModelID)
	}
	if _, exists := state.ModelRoles["default"]; exists {
		t.Fatalf("default role was not cleared: %#v", state.ModelRoles)
	}
	if _, exists := state.ModelRoles["smol"]; exists {
		t.Fatalf("smol role was not cleared: %#v", state.ModelRoles)
	}
	if state.ModelRoles["task"] != "gateway/three" || state.ModelRoles["custom"] != "external/model" {
		t.Fatalf("unrelated roles changed: %#v", state.ModelRoles)
	}
}

func mutationTestApp(t *testing.T, initial config.SwitchConfig) *App {
	t.Helper()
	root := t.TempDir()
	appPaths := paths.AppPaths{
		OMPSwitchConfigPath: filepath.Join(root, ".ompswitch", "config.json"),
		OMPModelsPath:       filepath.Join(root, ".omp", "agent", "models.yml"),
		OMPConfigPath:       filepath.Join(root, ".omp", "agent", "config.yml"),
		OMPSessionsDir:      filepath.Join(root, ".omp", "agent", "sessions"),
		BackupDir:           filepath.Join(root, ".ompswitch", "backups"),
	}
	service := config.NewService(appPaths)
	if err := service.SaveAppOnly(initial); err != nil {
		t.Fatal(err)
	}
	return &App{
		service: service, paths: appPaths,
		discoveryByRequest: map[string]context.CancelFunc{}, discoveryByProvider: map[string]string{},
	}
}

func mutationProvider(id, key, modelID string) provider.Config {
	return provider.Config{
		ID: id, Name: id, BaseURL: "https://example.com/v1", APIKey: key,
		API: "openai-responses", HeaderMode: "none", Headers: map[string]string{}, CustomHeaders: map[string]string{},
		Models: []provider.ModelInfo{{ID: modelID}}, SelectedModelID: modelID,
	}
}
