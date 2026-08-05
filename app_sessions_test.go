package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ompswitch/internal/config"
	"ompswitch/internal/omp"
)

func writeAppSession(t *testing.T, root, id, title, workingDir string) string {
	t.Helper()
	path := filepath.Join(root, "project", id+".jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	entries := []map[string]any{
		{"type": "title", "title": title, "updatedAt": "2026-08-05T04:00:20Z"},
		{"type": "session", "id": id, "timestamp": "2026-08-05T04:00:19Z", "cwd": workingDir},
		{"type": "model_change", "model": "provider/model"},
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(file)
	for _, entry := range entries {
		if err = encoder.Encode(entry); err != nil {
			file.Close()
			t.Fatal(err)
		}
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestContinueSessionLaunchesResumeInOriginalDirectory(t *testing.T) {
	app := mutationTestApp(t, config.SwitchConfig{Settings: config.AppSettings{OMPCommand: "omp"}})
	workingDir := t.TempDir()
	sessionPath := writeAppSession(t, app.paths.OMPSessionsDir, "session-id", "Session title", workingDir)
	var preview omp.LaunchPreview
	var launchedDirectory string
	app.startManagedOMP = func(input omp.LaunchPreview, directory string) (managedOMPProcess, error) {
		preview = input
		launchedDirectory = directory
		return &fakeManagedOMP{running: true}, nil
	}

	if err := app.ContinueSession("session-id"); err != nil {
		t.Fatal(err)
	}
	if launchedDirectory != workingDir || len(preview.Arguments) != 2 || preview.Arguments[0] != "--resume" || preview.Arguments[1] != sessionPath {
		t.Fatalf("preview=%#v directory=%q", preview, launchedDirectory)
	}
	if app.lastOMPLaunch == nil || app.managedOMP == nil {
		t.Fatal("continued session was not retained for restart")
	}
}

func TestListAndDeleteSession(t *testing.T) {
	app := mutationTestApp(t, config.SwitchConfig{})
	path := writeAppSession(t, app.paths.OMPSessionsDir, "session-id", "Session title", t.TempDir())
	items, err := app.ListSessions()
	if err != nil || len(items) != 1 || items[0].ID != "session-id" {
		t.Fatalf("sessions=%#v err=%v", items, err)
	}
	items, err = app.DeleteSession("session-id")
	if err != nil || len(items) != 0 {
		t.Fatalf("sessions after delete=%#v err=%v", items, err)
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("session file still exists: %v", err)
	}
}
