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
		BackupDir:           filepath.Join(home, ".ompswitch", "backups"),
	}
}
