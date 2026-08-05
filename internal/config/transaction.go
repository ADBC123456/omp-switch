package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"ompswitch/internal/omp"
	"ompswitch/internal/system"
)

type fileOperations struct {
	stat       func(string) (os.FileInfo, error)
	readFile   func(string) ([]byte, error)
	mkdirAll   func(string, os.FileMode) error
	createTemp func(string, string) (*os.File, error)
	replace    func(string, string) error
	remove     func(string) error
}

func osFileOperations() fileOperations {
	return fileOperations{
		stat: os.Stat, readFile: os.ReadFile, mkdirAll: os.MkdirAll,
		createTemp: os.CreateTemp, replace: os.Rename, remove: os.Remove,
	}
}

type targetState struct {
	path     string
	data     []byte
	existed  bool
	tempPath string
}

func (s *Service) SaveAppOnly(cfg SwitchConfig) error {
	appData, err := encodeAppConfig(cfg)
	if err != nil {
		return err
	}
	target, err := s.captureTarget(s.paths.OMPSwitchConfigPath)
	if err != nil {
		return err
	}
	if target.existed {
		if err := system.BackupFile(target.path, s.paths.BackupDir); err != nil {
			return fmt.Errorf("备份 %s: %w", target.path, err)
		}
	}
	if err := s.prepareTarget(&target, appData); err != nil {
		return err
	}
	defer s.cleanupTemps([]targetState{target})
	if err := s.files.replace(target.tempPath, target.path); err != nil {
		return fmt.Errorf("替换 %s: %w", target.path, err)
	}
	target.tempPath = ""
	return nil
}

func (s *Service) SaveOMPState(cfg SwitchConfig) error {
	cfg = normalizeConfig(cfg)
	modelsData, err := omp.EncodeModels(cfg.Providers)
	if err != nil {
		return fmt.Errorf("生成 %s: %w", s.paths.OMPModelsPath, err)
	}
	if _, err := omp.DecodeModels(modelsData); err != nil {
		return fmt.Errorf("校验 %s: %w", s.paths.OMPModelsPath, err)
	}

	currentConfig, configExisted, err := s.readOptional(s.paths.OMPConfigPath)
	if err != nil {
		return err
	}
	configData, err := omp.MergeManagedRoles(currentConfig, cfg.ModelRoles)
	if err != nil {
		return fmt.Errorf("生成 %s: %w", s.paths.OMPConfigPath, err)
	}
	if _, err := omp.DecodeManagedRoles(configData); err != nil {
		return fmt.Errorf("校验 %s: %w", s.paths.OMPConfigPath, err)
	}
	appData, err := encodeAppConfig(cfg)
	if err != nil {
		return err
	}

	modelsTarget, err := s.captureTarget(s.paths.OMPModelsPath)
	if err != nil {
		return err
	}
	configTarget := targetState{path: s.paths.OMPConfigPath, data: currentConfig, existed: configExisted}
	appTarget, err := s.captureTarget(s.paths.OMPSwitchConfigPath)
	if err != nil {
		return err
	}
	targets := []targetState{modelsTarget, configTarget, appTarget}
	contents := [][]byte{modelsData, configData, appData}

	for _, target := range targets {
		if target.existed {
			if err := system.BackupFile(target.path, s.paths.BackupDir); err != nil {
				return fmt.Errorf("备份 %s: %w", target.path, err)
			}
		}
	}
	for index := range targets {
		if err := s.prepareTarget(&targets[index], contents[index]); err != nil {
			s.cleanupTemps(targets)
			return err
		}
	}
	defer s.cleanupTemps(targets)

	replaced := make([]int, 0, len(targets))
	for index := range targets {
		if err := s.files.replace(targets[index].tempPath, targets[index].path); err != nil {
			originalErr := fmt.Errorf("替换 %s: %w", targets[index].path, err)
			return errors.Join(originalErr, s.rollback(targets, replaced))
		}
		targets[index].tempPath = ""
		replaced = append(replaced, index)
	}
	return nil
}

func encodeAppConfig(cfg SwitchConfig) ([]byte, error) {
	cfg = normalizeConfig(cfg)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("生成 app config: %w", err)
	}
	var decoded SwitchConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("校验 app config: %w", err)
	}
	return append(data, '\n'), nil
}

func (s *Service) captureTarget(path string) (targetState, error) {
	data, existed, err := s.readOptional(path)
	if err != nil {
		return targetState{}, err
	}
	return targetState{path: path, data: data, existed: existed}, nil
}

func (s *Service) readOptional(path string) ([]byte, bool, error) {
	if _, err := s.files.stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, fmt.Errorf("检查 %s: %w", path, err)
	}
	data, err := s.files.readFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("读取 %s: %w", path, err)
	}
	return data, true, nil
}

func (s *Service) prepareTarget(target *targetState, data []byte) error {
	dir := filepath.Dir(target.path)
	if err := s.files.mkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建目录 %s: %w", dir, err)
	}
	file, err := s.files.createTemp(dir, ".ompswitch-*")
	if err != nil {
		return fmt.Errorf("创建 %s 临时文件: %w", target.path, err)
	}
	target.tempPath = file.Name()
	writeErr := writeAndSync(file, data)
	if writeErr != nil {
		_ = s.files.remove(target.tempPath)
		target.tempPath = ""
		return fmt.Errorf("写入 %s 临时文件: %w", target.path, writeErr)
	}
	return nil
}

func writeAndSync(file *os.File, data []byte) error {
	_, writeErr := file.Write(data)
	modeErr := file.Chmod(0o644)
	syncErr := file.Sync()
	closeErr := file.Close()
	return errors.Join(writeErr, modeErr, syncErr, closeErr)
}

func (s *Service) rollback(targets []targetState, replaced []int) error {
	var rollbackErr error
	for index := len(replaced) - 1; index >= 0; index-- {
		target := targets[replaced[index]]
		if !target.existed {
			if err := s.files.remove(target.path); err != nil && !errors.Is(err, os.ErrNotExist) {
				rollbackErr = errors.Join(rollbackErr, fmt.Errorf("回滚删除 %s: %w", target.path, err))
			}
			continue
		}
		restore := targetState{path: target.path}
		if err := s.prepareTarget(&restore, target.data); err != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("回滚 %s: %w", target.path, err))
			continue
		}
		if err := s.files.replace(restore.tempPath, restore.path); err != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("回滚替换 %s: %w", target.path, err))
			_ = s.files.remove(restore.tempPath)
		}
	}
	return rollbackErr
}

func (s *Service) cleanupTemps(targets []targetState) {
	for _, target := range targets {
		if target.tempPath != "" {
			_ = s.files.remove(target.tempPath)
		}
	}
}
