package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveNativePathsCreatesMissingFiles(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)

	result, err := new(App).ResolveNativePaths()
	if err != nil {
		t.Fatal(err)
	}
	if !result.Detected {
		t.Fatalf("expected detected=true after auto-create, got %#v", result)
	}
	if result.Mode != "native" {
		t.Fatalf("mode = %q", result.Mode)
	}
	// Files and dirs must now exist with default content.
	modelsPath := result.CustomPaths.OMPModelsPath
	configPath := result.CustomPaths.OMPConfigPath
	sessionsDir := result.CustomPaths.OMPSessionsDir
	for _, path := range []string{modelsPath, configPath} {
		if info, e := os.Stat(path); e != nil || info.IsDir() {
			t.Fatalf("expected file %s to exist after detection: %v", path, e)
		}
	}
	if info, e := os.Stat(sessionsDir); e != nil || !info.IsDir() {
		t.Fatalf("expected sessions dir %s: %v", sessionsDir, e)
	}
	data, e := os.ReadFile(modelsPath)
	if e != nil {
		t.Fatal(e)
	}
	if string(data) != "providers: {}\n" {
		t.Fatalf("models.yml content = %q, want default empty providers", data)
	}
}

func TestResolveNativePathsDoesNotOverwriteExistingFiles(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)

	models := filepath.Join(root, ".omp", "agent", "models.yml")
	if err := os.MkdirAll(filepath.Dir(models), 0o755); err != nil {
		t.Fatal(err)
	}
	custom := "providers:\n  keep-me:\n    baseUrl: https://x/v1\n    api: openai-responses\n    apiKey: k\n"
	if err := os.WriteFile(models, []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := new(App).ResolveNativePaths()
	if err != nil {
		t.Fatal(err)
	}
	data, e := os.ReadFile(models)
	if e != nil {
		t.Fatal(e)
	}
	if string(data) != custom {
		t.Fatalf("existing models.yml was overwritten: %q", data)
	}
	if !result.Detected {
		t.Fatalf("existing file should be detected: %#v", result)
	}
}

func TestEnsureOMPFilesCreatesDefaults(t *testing.T) {
	root := t.TempDir()
	models := filepath.Join(root, ".omp", "agent", "models.yml")
	config := filepath.Join(root, ".omp", "agent", "config.yml")
	sessions := filepath.Join(root, ".omp", "agent", "sessions")
	if err := ensureOMPFiles(models, config, sessions); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{models, config} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s missing: %v", path, err)
		}
	}
	if info, err := os.Stat(sessions); err != nil || !info.IsDir() {
		t.Fatalf("sessions dir missing: %v", err)
	}
	// Second run must not fail nor rewrite.
	if err := ensureOMPFiles(models, config, sessions); err != nil {
		t.Fatal(err)
	}
}