package main

import (
	"errors"
	"testing"

	"ompswitch/internal/config"
	"ompswitch/internal/omp"
	"ompswitch/internal/provider"
)

type fakeManagedOMP struct {
	running bool
	stops   int
}

func (process *fakeManagedOMP) Running() bool { return process.running }
func (process *fakeManagedOMP) Stop() error {
	process.stops++
	process.running = false
	return nil
}

func TestExecuteLaunchOMPChoosesDirectoryAndRestartReusesOwnedLaunch(t *testing.T) {
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
	var previews []omp.LaunchPreview
	var directories []string
	var processes []*fakeManagedOMP
	app.startManagedOMP = func(preview omp.LaunchPreview, workingDir string) (managedOMPProcess, error) {
		process := &fakeManagedOMP{running: true}
		previews = append(previews, preview)
		directories = append(directories, workingDir)
		processes = append(processes, process)
		return process, nil
	}

	if err := app.ExecuteLaunchOMP("gateway", "model"); err != nil {
		t.Fatal(err)
	}
	if selectedDefault != "old-directory" || len(processes) != 1 {
		t.Fatalf("selected default=%q, starts=%d", selectedDefault, len(processes))
	}
	if err := app.RestartOMP(); err != nil {
		t.Fatal(err)
	}
	if processes[0].stops != 1 || len(processes) != 2 {
		t.Fatalf("first stops=%d, starts=%d", processes[0].stops, len(processes))
	}
	if directories[0] != directories[1] || previews[0].Executable != previews[1].Executable {
		t.Fatalf("restart changed launch: dirs=%v previews=%+v", directories, previews)
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
	app.startManagedOMP = func(omp.LaunchPreview, string) (managedOMPProcess, error) {
		t.Fatal("cancelled launch started a process")
		return nil, nil
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

func TestRestartOMPRequiresApplicationOwnedLaunch(t *testing.T) {
	app := mutationTestApp(t, config.SwitchConfig{})
	app.startManagedOMP = func(omp.LaunchPreview, string) (managedOMPProcess, error) {
		return nil, errors.New("must not start")
	}
	if err := app.RestartOMP(); err == nil {
		t.Fatal("restart without an application-owned launch succeeded")
	}
}
