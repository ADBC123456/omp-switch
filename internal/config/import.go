package config

import (
	"errors"
	"fmt"
	"os"

	"ompswitch/internal/omp"
)

// ImportFromOMP reloads providers and model roles from the OMP files this
// service points at (models.yml + config.yml), replacing whatever was stored
// in the app configuration. Settings on the passed-in config are preserved.
//
// This is used after custom OMP paths change, so the app switches wholesale
// to the providers of the newly targeted OMP install (e.g. a WSL distro)
// instead of continuing to show the previous installation's models.
func (s *Service) ImportFromOMP(cfg SwitchConfig) (SwitchConfig, error) {
	modelsData, err := s.files.readFile(s.paths.OMPModelsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SwitchConfig{}, fmt.Errorf("OMP Models 文件不存在：%s（请先用「检测 OMP 路径」确认路径正确）", s.paths.OMPModelsPath)
		}
		return SwitchConfig{}, fmt.Errorf("读取 %s: %w", s.paths.OMPModelsPath, err)
	}
	providers, err := omp.DecodeModels(modelsData)
	if err != nil {
		return SwitchConfig{}, fmt.Errorf("解析 %s: %w", s.paths.OMPModelsPath, err)
	}
	cfg.Providers = providers

	configData, err := s.files.readFile(s.paths.OMPConfigPath)
	if err == nil {
		roles, roleErr := omp.DecodeManagedRoles(configData)
		if roleErr != nil {
			return SwitchConfig{}, fmt.Errorf("解析 %s: %w", s.paths.OMPConfigPath, roleErr)
		}
		cfg.ModelRoles = roles
	} else if !errors.Is(err, os.ErrNotExist) {
		return SwitchConfig{}, fmt.Errorf("读取 %s: %w", s.paths.OMPConfigPath, err)
	}
	return normalizeConfig(cfg), nil
}