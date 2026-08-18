package paths

import (
	"os"
	"path/filepath"
)

type AppPaths struct {
	OMPSwitchConfigPath string
	OMPModelsPath       string
	OMPConfigPath       string
	OMPSessionsDir      string
	OMPGlobalSkillsDir  string
	OMPGlobalSkillsLock string
	BackupDir           string
}

func DefaultPaths() AppPaths {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return AppPaths{
		OMPSwitchConfigPath: filepath.Join(home, ".ompswitch", "config.json"),
		OMPModelsPath:       filepath.Join(home, ".omp", "agent", "models.yml"),
		OMPConfigPath:       filepath.Join(home, ".omp", "agent", "config.yml"),
		OMPSessionsDir:      filepath.Join(home, ".omp", "agent", "sessions"),
		OMPGlobalSkillsDir:  filepath.Join(home, ".agents", "skills"),
		OMPGlobalSkillsLock: filepath.Join(home, ".agents", ".skill-lock.json"),
		BackupDir:           filepath.Join(home, ".ompswitch", "backups"),
	}
}

// ApplyCustomPaths returns a copy of base with the three OMP-related paths
// overridden by non-empty values. config.json and backups are never
// overridden — they stay on the Windows host as bootstrap anchors.
func ApplyCustomPaths(base AppPaths, ompModelsPath, ompConfigPath, ompSessionsDir string) AppPaths {
	result := base
	if ompModelsPath != "" {
		result.OMPModelsPath = ompModelsPath
	}
	if ompConfigPath != "" {
		result.OMPConfigPath = ompConfigPath
	}
	if ompSessionsDir != "" {
		result.OMPSessionsDir = ompSessionsDir
	}
	return result
}
