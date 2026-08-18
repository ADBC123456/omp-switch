package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"ompswitch/internal/omp"
	"ompswitch/internal/paths"
	"ompswitch/internal/provider"
)

const switchConfigVersion = 2

type AppSettings struct {
	OMPCommand            string      `json:"ompCommand"`
	Theme                 string      `json:"theme"`
	LegacyDarkMode        *bool       `json:"darkMode,omitempty"`
	WorkingDir            string      `json:"workingDir"`
	LastUpdateCheckAtUnix int64       `json:"lastUpdateCheckAt,omitempty"`
	CustomPaths           CustomPaths `json:"customPaths,omitempty"`
	LaunchMode            string      `json:"launchMode,omitempty"`
	WSLDistro             string      `json:"wslDistro,omitempty"`
}

// CustomPaths holds user-configurable overrides for OMP file locations.
// Empty fields fall back to DefaultPaths(). Only OMP-related paths are
// overridable; config.json and backups stay on the Windows host.
type CustomPaths struct {
	OMPModelsPath  string `json:"ompModelsPath,omitempty"`
	OMPConfigPath  string `json:"ompConfigPath,omitempty"`
	OMPSessionsDir string `json:"ompSessionsDir,omitempty"`
}

// IsEmpty reports whether all custom path fields are unset.
func (c CustomPaths) IsEmpty() bool {
	return c.OMPModelsPath == "" && c.OMPConfigPath == "" && c.OMPSessionsDir == ""
}

type SwitchConfig struct {
	Version            int               `json:"version"`
	Providers          []provider.Config `json:"providers"`
	SelectedProviderID string            `json:"selectedProviderId"`
	ModelRoles         map[string]string `json:"modelRoles"`
	Settings           AppSettings       `json:"settings"`
}

type PathView struct {
	OMPSwitchConfigPath string `json:"ompSwitchConfigPath"`
	OMPModelsPath       string `json:"ompModelsPath"`
	OMPConfigPath       string `json:"ompConfigPath"`
	OMPSessionsDir      string `json:"ompSessionsDir"`
	BackupDir           string `json:"backupDir"`
}

type AppState struct {
	Version            string            `json:"version"`
	Providers          []provider.View   `json:"providers"`
	SelectedProviderID string            `json:"selectedProviderId"`
	ModelRoles         map[string]string `json:"modelRoles"`
	Settings           AppSettings       `json:"settings"`
	Paths              PathView          `json:"paths"`
	Logs               []string          `json:"logs"`
}

type RoleUpdate struct {
	Role     string `json:"role"`
	Selector string `json:"selector"`
	Clear    bool   `json:"clear"`
}

type Service struct {
	paths paths.AppPaths
	files fileOperations
}

func NewService(appPaths paths.AppPaths) *Service {
	return &Service{paths: appPaths, files: osFileOperations()}
}

func (s *Service) Load() (SwitchConfig, error) {
	if _, err := s.files.stat(s.paths.OMPSwitchConfigPath); err == nil {
		data, err := s.files.readFile(s.paths.OMPSwitchConfigPath)
		if err != nil {
			return SwitchConfig{}, fmt.Errorf("读取 %s: %w", s.paths.OMPSwitchConfigPath, err)
		}
		var cfg SwitchConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			return SwitchConfig{}, fmt.Errorf("解析 %s: %w", s.paths.OMPSwitchConfigPath, err)
		}
		return normalizeConfig(cfg), nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return SwitchConfig{}, fmt.Errorf("检查 %s: %w", s.paths.OMPSwitchConfigPath, err)
	}

	cfg, err := s.initialConfig()
	if err != nil {
		return SwitchConfig{}, err
	}
	if err := s.SaveAppOnly(cfg); err != nil {
		return SwitchConfig{}, fmt.Errorf("初始化 %s: %w", s.paths.OMPSwitchConfigPath, err)
	}
	return cfg, nil
}

func (s *Service) initialConfig() (SwitchConfig, error) {
	cfg := SwitchConfig{Version: switchConfigVersion, ModelRoles: map[string]string{}, Settings: NormalizeSettings(AppSettings{})}
	if _, err := s.files.stat(s.paths.OMPModelsPath); errors.Is(err, os.ErrNotExist) {
		cfg.Providers = provider.Presets()
		return normalizeConfig(cfg), nil
	} else if err != nil {
		return SwitchConfig{}, fmt.Errorf("检查 %s: %w", s.paths.OMPModelsPath, err)
	}

	modelsData, err := s.files.readFile(s.paths.OMPModelsPath)
	if err != nil {
		return SwitchConfig{}, fmt.Errorf("读取 %s: %w", s.paths.OMPModelsPath, err)
	}
	cfg.Providers, err = omp.DecodeModels(modelsData)
	if err != nil {
		return SwitchConfig{}, fmt.Errorf("解析 %s: %w", s.paths.OMPModelsPath, err)
	}

	if _, err := s.files.stat(s.paths.OMPConfigPath); err == nil {
		configData, readErr := s.files.readFile(s.paths.OMPConfigPath)
		if readErr != nil {
			return SwitchConfig{}, fmt.Errorf("读取 %s: %w", s.paths.OMPConfigPath, readErr)
		}
		cfg.ModelRoles, err = omp.DecodeManagedRoles(configData)
		if err != nil {
			return SwitchConfig{}, fmt.Errorf("解析 %s: %w", s.paths.OMPConfigPath, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return SwitchConfig{}, fmt.Errorf("检查 %s: %w", s.paths.OMPConfigPath, err)
	}
	return normalizeConfig(cfg), nil
}

func NormalizeSettings(input AppSettings) AppSettings {
	input.OMPCommand = strings.TrimSpace(input.OMPCommand)
	if input.OMPCommand == "" {
		input.OMPCommand = "omp"
	}
	if input.Theme == "" && input.LegacyDarkMode != nil {
		if *input.LegacyDarkMode {
			input.Theme = "dark"
		} else {
			input.Theme = "light"
		}
	}
	if input.Theme != "light" && input.Theme != "dark" && input.Theme != "system" {
		input.Theme = "system"
	}
	input.LegacyDarkMode = nil
	if input.LaunchMode != "native" && input.LaunchMode != "wsl" {
		input.LaunchMode = "native"
	}
	input.WSLDistro = strings.TrimSpace(input.WSLDistro)
	if input.WorkingDir == "" && input.LaunchMode != "wsl" {
		// Native mode defaults to the Windows home directory; WSL mode keeps
		// it empty so startup can demand an explicit Linux path.
		if home, err := os.UserHomeDir(); err == nil {
			input.WorkingDir = home
		}
	}
	input.CustomPaths.OMPModelsPath = strings.TrimSpace(input.CustomPaths.OMPModelsPath)
	input.CustomPaths.OMPConfigPath = strings.TrimSpace(input.CustomPaths.OMPConfigPath)
	input.CustomPaths.OMPSessionsDir = strings.TrimSpace(input.CustomPaths.OMPSessionsDir)
	return input
}

func normalizeConfig(input SwitchConfig) SwitchConfig {
	input.Version = switchConfigVersion
	input.Settings = NormalizeSettings(input.Settings)
	for index := range input.Providers {
		input.Providers[index] = provider.Normalize(input.Providers[index])
	}
	if input.Providers == nil {
		input.Providers = []provider.Config{}
	}
	if input.ModelRoles == nil {
		input.ModelRoles = map[string]string{}
	}
	if !input.hasProvider(input.SelectedProviderID) {
		input.SelectedProviderID = ""
		if len(input.Providers) > 0 {
			input.SelectedProviderID = input.Providers[0].ID
		}
	}
	return input
}

func NewAppState(version string, cfg SwitchConfig, appPaths paths.AppPaths, logs []string) AppState {
	cfg = normalizeConfig(cfg)
	views := make([]provider.View, len(cfg.Providers))
	for index, item := range cfg.Providers {
		views[index] = provider.NewView(item)
	}
	return AppState{
		Version: version, Providers: views, SelectedProviderID: cfg.SelectedProviderID,
		ModelRoles: cloneRoles(cfg.ModelRoles), Settings: cfg.Settings,
		Paths: PathView{
			OMPSwitchConfigPath: appPaths.OMPSwitchConfigPath,
			OMPModelsPath:       appPaths.OMPModelsPath,
			OMPConfigPath:       appPaths.OMPConfigPath,
			OMPSessionsDir:      appPaths.OMPSessionsDir,
			BackupDir:           appPaths.BackupDir,
		},
		Logs: append([]string(nil), logs...),
	}
}

func cloneRoles(input map[string]string) map[string]string {
	result := make(map[string]string, len(input))
	for role, selector := range input {
		result[role] = selector
	}
	return result
}

func (cfg *SwitchConfig) hasProvider(id string) bool {
	if id == "" {
		return false
	}
	for _, item := range cfg.Providers {
		if item.ID == id {
			return true
		}
	}
	return false
}

func (cfg *SwitchConfig) ProviderByID(id string) (provider.Config, error) {
	for _, item := range cfg.Providers {
		if item.ID == id {
			return item, nil
		}
	}
	return provider.Config{}, errors.New("未找到 Provider：" + id)
}

func (cfg *SwitchConfig) UpsertProvider(input provider.Config, oldID string) {
	input = provider.Normalize(input)
	for index, item := range cfg.Providers {
		if item.ID == oldID || (oldID == "" && item.ID == input.ID) {
			cfg.Providers[index] = input
			if cfg.SelectedProviderID == oldID || cfg.SelectedProviderID == "" {
				cfg.SelectedProviderID = input.ID
			}
			return
		}
	}
	cfg.Providers = append(cfg.Providers, input)
	if cfg.SelectedProviderID == "" {
		cfg.SelectedProviderID = input.ID
	}
}

func (cfg *SwitchConfig) DeleteProvider(id string) {
	next := make([]provider.Config, 0, len(cfg.Providers))
	for _, item := range cfg.Providers {
		if item.ID != id {
			next = append(next, item)
		}
	}
	cfg.Providers = next
	if cfg.SelectedProviderID == id || !cfg.hasProvider(cfg.SelectedProviderID) {
		cfg.SelectedProviderID = ""
		if len(cfg.Providers) > 0 {
			cfg.SelectedProviderID = cfg.Providers[0].ID
		}
	}
}
