package paths

import (
	"path/filepath"
	"testing"
)

func TestDefaultPathsUseOMPDirectories(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	got := DefaultPaths()
	if got.OMPSwitchConfigPath != filepath.Join(home, ".ompswitch", "config.json") {
		t.Fatalf("OMPSwitchConfigPath = %q", got.OMPSwitchConfigPath)
	}
	if got.OMPModelsPath != filepath.Join(home, ".omp", "agent", "models.yml") {
		t.Fatalf("OMPModelsPath = %q", got.OMPModelsPath)
	}
	if got.OMPConfigPath != filepath.Join(home, ".omp", "agent", "config.yml") {
		t.Fatalf("OMPConfigPath = %q", got.OMPConfigPath)
	}
	if got.OMPSessionsDir != filepath.Join(home, ".omp", "agent", "sessions") {
		t.Fatalf("OMPSessionsDir = %q", got.OMPSessionsDir)
	}
	if got.OMPGlobalSkillsDir != filepath.Join(home, ".agents", "skills") {
		t.Fatalf("OMPGlobalSkillsDir = %q", got.OMPGlobalSkillsDir)
	}
	if got.OMPGlobalSkillsLock != filepath.Join(home, ".agents", ".skill-lock.json") {
		t.Fatalf("OMPGlobalSkillsLock = %q", got.OMPGlobalSkillsLock)
	}
	if got.BackupDir != filepath.Join(home, ".ompswitch", "backups") {
		t.Fatalf("BackupDir = %q", got.BackupDir)
	}
}
