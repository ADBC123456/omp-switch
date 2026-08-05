package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ompswitch/internal/provider"
)

func transactionConfig() SwitchConfig {
	return SwitchConfig{
		Providers: []provider.Config{{
			ID: "p", Name: "P", BaseURL: "https://example.com/v1", APIKey: "API_KEY", API: "openai-completions",
			Headers: map[string]string{}, CustomHeaders: map[string]string{}, Models: []provider.ModelInfo{{ID: "m"}}, SelectedModelID: "m",
		}},
		SelectedProviderID: "p", ModelRoles: map[string]string{"default": "p/m"},
	}
}

func TestSaveAppOnlyDoesNotTouchYAML(t *testing.T) {
	appPaths := testPaths(t)
	writeTestFile(t, appPaths.OMPModelsPath, []byte("models-before"))
	writeTestFile(t, appPaths.OMPConfigPath, []byte("config-before"))
	if err := NewService(appPaths).SaveAppOnly(transactionConfig()); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{appPaths.OMPModelsPath: "models-before", appPaths.OMPConfigPath: "config-before"} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("%s=%q, %v; want %q", path, got, err, want)
		}
	}
}

func TestSaveAppOnlyBacksUpExistingAppInConfiguredDirectory(t *testing.T) {
	appPaths := testPaths(t)
	writeTestFile(t, appPaths.OMPSwitchConfigPath, []byte("before"))
	if err := NewService(appPaths).SaveAppOnly(transactionConfig()); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(appPaths.BackupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), "config.json-") {
		t.Fatalf("backups=%v", entries)
	}
	data, err := os.ReadFile(filepath.Join(appPaths.BackupDir, entries[0].Name()))
	if err != nil || string(data) != "before" {
		t.Fatalf("backup=%q, %v", data, err)
	}
}

func TestSaveOMPStateReplacementOrder(t *testing.T) {
	appPaths := testPaths(t)
	service := NewService(appPaths)
	originalReplace := service.files.replace
	var order []string
	service.files.replace = func(oldPath, newPath string) error {
		order = append(order, newPath)
		return originalReplace(oldPath, newPath)
	}
	if err := service.SaveOMPState(transactionConfig()); err != nil {
		t.Fatal(err)
	}
	want := []string{appPaths.OMPModelsPath, appPaths.OMPConfigPath, appPaths.OMPSwitchConfigPath}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("replace order=%v, want %v", order, want)
	}
}

func TestSaveOMPStateRollsBackFailuresAndCleansTemps(t *testing.T) {
	for failAt := 1; failAt <= 3; failAt++ {
		t.Run(string(rune('0'+failAt)), func(t *testing.T) {
			appPaths := testPaths(t)
			before := map[string]string{
				appPaths.OMPModelsPath:       "providers: {}\n",
				appPaths.OMPConfigPath:       "theme: dark\n",
				appPaths.OMPSwitchConfigPath: `{"version":2,"providers":[]}`,
			}
			for path, data := range before {
				writeTestFile(t, path, []byte(data))
			}
			service := NewService(appPaths)
			originalReplace := service.files.replace
			calls := 0
			var replacePaths []string
			service.files.replace = func(oldPath, newPath string) error {
				replacePaths = append(replacePaths, newPath)
				calls++
				if calls == failAt {
					return errors.New("injected replace failure")
				}
				return originalReplace(oldPath, newPath)
			}
			if err := service.SaveOMPState(transactionConfig()); err == nil || !strings.Contains(err.Error(), "injected replace failure") {
				t.Fatalf("error=%v", err)
			}
			for path, want := range before {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != want {
					t.Fatalf("%s=%q, %v; want %q", path, got, err, want)
				}
			}
			if failAt == 3 {
				wantOrder := []string{appPaths.OMPModelsPath, appPaths.OMPConfigPath, appPaths.OMPSwitchConfigPath, appPaths.OMPConfigPath, appPaths.OMPModelsPath}
				if !reflect.DeepEqual(replacePaths, wantOrder) {
					t.Fatalf("replace/rollback order=%v, want %v", replacePaths, wantOrder)
				}
			}
			assertNoTransactionTemps(t, filepath.Dir(appPaths.OMPModelsPath))
			assertNoTransactionTemps(t, filepath.Dir(appPaths.OMPSwitchConfigPath))
		})
	}
}

func TestSaveOMPStateRemovesNewTargetsDuringRollback(t *testing.T) {
	appPaths := testPaths(t)
	service := NewService(appPaths)
	originalReplace := service.files.replace
	calls := 0
	service.files.replace = func(oldPath, newPath string) error {
		calls++
		if calls == 3 {
			return errors.New("app replace failed")
		}
		return originalReplace(oldPath, newPath)
	}
	if err := service.SaveOMPState(transactionConfig()); err == nil {
		t.Fatal("expected failure")
	}
	for _, path := range []string{appPaths.OMPModelsPath, appPaths.OMPConfigPath, appPaths.OMPSwitchConfigPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("new target %s survived rollback: %v", path, err)
		}
	}
}

func TestSaveOMPStateAggregatesRollbackError(t *testing.T) {
	appPaths := testPaths(t)
	writeTestFile(t, appPaths.OMPModelsPath, []byte("providers: {}\n"))
	service := NewService(appPaths)
	originalReplace := service.files.replace
	calls := 0
	service.files.replace = func(oldPath, newPath string) error {
		calls++
		if calls == 2 {
			return errors.New("original failure")
		}
		if calls == 3 {
			return errors.New("rollback failure")
		}
		return originalReplace(oldPath, newPath)
	}
	err := service.SaveOMPState(transactionConfig())
	if err == nil || !strings.Contains(err.Error(), "original failure") || !strings.Contains(err.Error(), "rollback failure") {
		t.Fatalf("error=%v", err)
	}
}

func TestSaveOMPStateInvalidConfigYAMLWritesNothing(t *testing.T) {
	appPaths := testPaths(t)
	writeTestFile(t, appPaths.OMPModelsPath, []byte("models-before"))
	writeTestFile(t, appPaths.OMPConfigPath, []byte("modelRoles: [broken\n"))
	writeTestFile(t, appPaths.OMPSwitchConfigPath, []byte("app-before"))
	if err := NewService(appPaths).SaveOMPState(transactionConfig()); err == nil {
		t.Fatal("expected invalid config YAML error")
	}
	for path, want := range map[string]string{appPaths.OMPModelsPath: "models-before", appPaths.OMPConfigPath: "modelRoles: [broken\n", appPaths.OMPSwitchConfigPath: "app-before"} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("%s=%q, %v; want unchanged %q", path, got, err, want)
		}
	}
}

func assertNoTransactionTemps(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".ompswitch-") {
			t.Fatalf("temporary file was not cleaned: %s", filepath.Join(dir, entry.Name()))
		}
	}
}
