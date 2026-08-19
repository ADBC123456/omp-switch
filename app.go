package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"strconv"
	"sync"
	"time"
	"unicode/utf16"
	"encoding/binary"

	"github.com/fsnotify/fsnotify"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ompswitch/internal/config"
	"ompswitch/internal/omp"
	"ompswitch/internal/paths"
	"ompswitch/internal/provider"
	"ompswitch/internal/sessions"
	"ompswitch/internal/skills"
	"ompswitch/internal/system"
	"ompswitch/internal/updater"
)

var appVersion = "1.3.0"

const configChangedEvent = "omp:config-changed"
const updateAvailableEvent = "omp:update-available"

type ProviderMutationResult struct {
	State           config.AppState `json:"state"`
	FinalProviderID string          `json:"finalProviderId"`
	Adjusted        bool            `json:"adjusted"`
}

type App struct {
	ctx                 context.Context
	mutationMu          sync.Mutex
	service             *config.Service
	paths               paths.AppPaths
	discoveryMu         sync.Mutex
	discoveryByRequest  map[string]context.CancelFunc
	discoveryByProvider map[string]string
	selectLaunchDir     func(string) (string, error)
	startManagedOMP     func(omp.LaunchPreview, string) error
	startWSLOMP         func(cfg wslLaunchConfig) error
	watcher             *fsnotify.Watcher
	watcherStop         chan struct{}
	writeMu             sync.Mutex
	lastSelfWrite       time.Time
}

// wslLaunchConfig carries all parameters needed to start OMP inside WSL.
type wslLaunchConfig struct {
	ompCommand string
	args       []string
	distro     string
	workingDir string
}

func NewApp() *App {
	p := paths.DefaultPaths()
	app := &App{service: config.NewService(p), paths: p, discoveryByRequest: map[string]context.CancelFunc{}, discoveryByProvider: map[string]string{}}
	app.startManagedOMP = func(preview omp.LaunchPreview, workingDir string) error {
		_, err := omp.StartManaged(preview, workingDir)
		return err
	}
	app.startWSLOMP = func(cfg wslLaunchConfig) error {
		return omp.StartWSL(cfg.ompCommand, cfg.args, cfg.distro, cfg.workingDir)
	}
	return app
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.paths.OMPSwitchConfigPath == "" {
		a.paths = paths.DefaultPaths()
	}
	if a.service == nil {
		a.service = config.NewService(a.paths)
	}
	if a.discoveryByRequest == nil {
		a.discoveryByRequest = map[string]context.CancelFunc{}
	}
	if a.discoveryByProvider == nil {
		a.discoveryByProvider = map[string]string{}
	}
	if a.startManagedOMP == nil {
		a.startManagedOMP = func(preview omp.LaunchPreview, workingDir string) error {
			_, err := omp.StartManaged(preview, workingDir)
			return err
		}
	}
	if a.startWSLOMP == nil {
		a.startWSLOMP = func(cfg wslLaunchConfig) error {
			return omp.StartWSL(cfg.ompCommand, cfg.args, cfg.distro, cfg.workingDir)
		}
	}
	// Load config once to read customPaths, then rebuild paths+service if overrides exist.
	cfg, _ := a.service.Load()
	if !cfg.Settings.CustomPaths.IsEmpty() {
		a.paths = paths.ApplyCustomPaths(a.paths, cfg.Settings.CustomPaths.OMPModelsPath, cfg.Settings.CustomPaths.OMPConfigPath, cfg.Settings.CustomPaths.OMPSessionsDir)
		a.service = config.NewService(a.paths)
	}
	a.startConfigWatcher()
	a.startBackgroundUpdateCheck()
}

func (a *App) shutdown(context.Context) { a.stopConfigWatcher() }
func (a *App) markSelfWrite()           { a.writeMu.Lock(); a.lastSelfWrite = time.Now(); a.writeMu.Unlock() }
func (a *App) recentSelfWrite() bool {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	return time.Since(a.lastSelfWrite) < 800*time.Millisecond
}

func (a *App) startConfigWatcher() {
	if a.paths.OMPSwitchConfigPath == "" {
		return
	}
	target := filepath.Clean(a.paths.OMPSwitchConfigPath)
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	if err = w.Add(filepath.Dir(target)); err != nil {
		_ = w.Close()
		return
	}
	a.watcher, a.watcherStop = w, make(chan struct{})
	go func(stop <-chan struct{}) {
		var debounce *time.Timer
		var debounceC <-chan time.Time
		for {
			select {
			case <-stop:
				if debounce != nil {
					debounce.Stop()
				}
				return
			case e, ok := <-w.Events:
				if !ok {
					return
				}
				if filepath.Clean(e.Name) != target || e.Op&(fsnotify.Write|fsnotify.Create) == 0 {
					continue
				}
				if debounce == nil {
					debounce = time.NewTimer(300 * time.Millisecond)
				} else {
					if !debounce.Stop() {
						select {
						case <-debounce.C:
						default:
						}
					}
					debounce.Reset(300 * time.Millisecond)
				}
				debounceC = debounce.C
			case <-debounceC:
				debounceC = nil
				if !a.recentSelfWrite() {
					runtime.EventsEmit(a.ctx, configChangedEvent)
				}
			case _, ok := <-w.Errors:
				if !ok {
					return
				}
			}
		}
	}(a.watcherStop)
}

func (a *App) stopConfigWatcher() {
	if a.watcherStop != nil {
		close(a.watcherStop)
		a.watcherStop = nil
	}
	if a.watcher != nil {
		_ = a.watcher.Close()
		a.watcher = nil
	}
}

func (a *App) state(cfg config.SwitchConfig) config.AppState {
	return config.NewAppState(appVersion, cfg, a.paths, []string{"就绪。"})
}

func (a *App) GetAppState() (config.AppState, error) {
	cfg, err := a.service.Load()
	if err != nil {
		return config.AppState{}, err
	}
	return a.state(cfg), nil
}

func (a *App) ListProviders() ([]provider.View, error) {
	cfg, err := a.service.Load()
	if err != nil {
		return nil, err
	}
	views := make([]provider.View, len(cfg.Providers))
	for index, item := range cfg.Providers {
		views[index] = provider.NewView(item)
	}
	return views, nil
}

func (a *App) CreateProvider(input provider.SaveInput) (ProviderMutationResult, error) {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	cfg, e := a.loadClone()
	if e != nil {
		return ProviderMutationResult{}, e
	}
	p := providerFromInput(input)
	if p.APIKey == "" {
		return ProviderMutationResult{}, errors.New("API Key 不能为空")
	}
	requested := strings.TrimSpace(p.ID)
	p.ID = availableProviderID(requested, cfg.Providers, "")
	p = provider.Normalize(p)
	if e = config.ValidateProvider(p); e != nil {
		return ProviderMutationResult{}, e
	}
	cfg.UpsertProvider(p, "")
	if e = a.saveOMP(cfg); e != nil {
		return ProviderMutationResult{}, e
	}
	return ProviderMutationResult{a.state(cfg), p.ID, p.ID != requested}, nil
}

func (a *App) UpdateProvider(id string, input provider.SaveInput) (ProviderMutationResult, error) {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	cfg, e := a.loadClone()
	if e != nil {
		return ProviderMutationResult{}, e
	}
	old, e := cfg.ProviderByID(id)
	if e != nil {
		return ProviderMutationResult{}, e
	}
	p := providerFromInput(input)
	requested := strings.TrimSpace(p.ID)
	p.ID = availableProviderID(requested, cfg.Providers, id)
	if p.APIKey == "" {
		p.APIKey = old.APIKey
	}
	p.Models = cloneModels(old.Models)
	p.SelectedModelID = old.SelectedModelID
	p = provider.Normalize(p)
	if e = config.ValidateProvider(p); e != nil {
		return ProviderMutationResult{}, e
	}
	if p.ID != id {
		omp.RewriteManagedSelectors(cfg.ModelRoles, cfg.Providers, id, "", p.ID, "")
	}
	cfg.UpsertProvider(p, id)
	if e = a.saveOMP(cfg); e != nil {
		return ProviderMutationResult{}, e
	}
	return ProviderMutationResult{a.state(cfg), p.ID, p.ID != requested}, nil
}

func (a *App) GetProviderDeleteImpact(id string) ([]string, error) {
	cfg, e := a.service.Load()
	if e != nil {
		return nil, e
	}
	if _, e = cfg.ProviderByID(id); e != nil {
		return nil, e
	}
	return omp.ManagedRoleImpact(cfg.ModelRoles, cfg.Providers, id, ""), nil
}

func (a *App) DeleteProvider(id string) (config.AppState, error) {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	cfg, e := a.loadClone()
	if e != nil {
		return config.AppState{}, e
	}
	if _, e = cfg.ProviderByID(id); e != nil {
		return config.AppState{}, e
	}
	omp.RewriteManagedSelectors(cfg.ModelRoles, cfg.Providers, id, "", "", "")
	cfg.DeleteProvider(id)
	if e = a.saveOMP(cfg); e != nil {
		return config.AppState{}, e
	}
	return a.state(cfg), nil
}

func (a *App) SaveModel(providerID, originalID string, input provider.ModelInfo) (config.AppState, error) {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	cfg, e := a.loadClone()
	if e != nil {
		return config.AppState{}, e
	}
	p, e := cfg.ProviderByID(providerID)
	if e != nil {
		return config.AppState{}, e
	}
	p.Models = cloneModels(p.Models)
	input.ID = strings.TrimSpace(input.ID)
	input.Name = strings.TrimSpace(input.Name)
	input.API = strings.TrimSpace(input.API)
	if input.ID == "" {
		return config.AppState{}, errors.New("模型 ID 不能为空")
	}
	originalID = strings.TrimSpace(originalID)
	idx := -1
	for i, m := range p.Models {
		if m.ID == originalID {
			idx = i
		}
		if m.ID == input.ID && (originalID == "" || m.ID != originalID) {
			return config.AppState{}, fmt.Errorf("模型 ID 冲突：%s", input.ID)
		}
	}
	if originalID != "" && idx < 0 {
		return config.AppState{}, errors.New("模型不存在：" + originalID)
	}
	if originalID == "" {
		p.Models = append(p.Models, input)
	} else {
		p.Models[idx] = input
		if p.SelectedModelID == originalID {
			p.SelectedModelID = input.ID
		}
	}
	p = provider.Normalize(p)
	if e = config.ValidateProvider(p); e != nil {
		return config.AppState{}, e
	}
	if originalID != "" && originalID != input.ID {
		omp.RewriteManagedSelectors(cfg.ModelRoles, cfg.Providers, providerID, originalID, providerID, input.ID)
	}
	cfg.UpsertProvider(p, providerID)
	if e = a.saveOMP(cfg); e != nil {
		return config.AppState{}, e
	}
	return a.state(cfg), nil
}

func selectedModelIDs(p provider.Config, modelIDs []string) ([]string, map[string]struct{}, error) {
	ids := make([]string, 0, len(modelIDs))
	selected := make(map[string]struct{}, len(modelIDs))
	for _, modelID := range modelIDs {
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			return nil, nil, errors.New("模型 ID 不能为空")
		}
		if _, exists := selected[modelID]; exists {
			continue
		}
		if err := provider.EnsureModel(p, modelID); err != nil {
			return nil, nil, err
		}
		ids = append(ids, modelID)
		selected[modelID] = struct{}{}
	}
	if len(ids) == 0 {
		return nil, nil, errors.New("至少选择一个模型")
	}
	return ids, selected, nil
}

func (a *App) GetModelsDeleteImpact(providerID string, modelIDs []string) ([]string, error) {
	cfg, err := a.service.Load()
	if err != nil {
		return nil, err
	}
	p, err := cfg.ProviderByID(providerID)
	if err != nil {
		return nil, err
	}
	_, selected, err := selectedModelIDs(p, modelIDs)
	if err != nil {
		return nil, err
	}
	roles := make([]string, 0)
	for _, role := range omp.ManagedRoles {
		matchedProvider, matchedModel, _, matched := omp.ParseManagedSelector(cfg.ModelRoles[role], cfg.Providers)
		if matched && matchedProvider == providerID {
			if _, exists := selected[matchedModel]; exists {
				roles = append(roles, role)
			}
		}
	}
	return roles, nil
}

func (a *App) DeleteModels(providerID string, modelIDs []string) (config.AppState, error) {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	cfg, err := a.loadClone()
	if err != nil {
		return config.AppState{}, err
	}
	p, err := cfg.ProviderByID(providerID)
	if err != nil {
		return config.AppState{}, err
	}
	ids, selected, err := selectedModelIDs(p, modelIDs)
	if err != nil {
		return config.AppState{}, err
	}
	for _, modelID := range ids {
		omp.RewriteManagedSelectors(cfg.ModelRoles, cfg.Providers, providerID, modelID, "", "")
	}
	models := make([]provider.ModelInfo, 0, len(p.Models)-len(selected))
	for _, model := range p.Models {
		if _, remove := selected[model.ID]; !remove {
			models = append(models, model)
		}
	}
	p.Models = models
	if _, removed := selected[p.SelectedModelID]; removed {
		p.SelectedModelID = ""
	}
	p = provider.Normalize(p)
	cfg.UpsertProvider(p, providerID)
	if err = a.saveOMP(cfg); err != nil {
		return config.AppState{}, err
	}
	return a.state(cfg), nil
}

func (a *App) SetSelectedProvider(id string) (config.AppState, error) {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	cfg, e := a.loadClone()
	if e != nil {
		return config.AppState{}, e
	}
	if _, e = cfg.ProviderByID(id); e != nil {
		return config.AppState{}, e
	}
	cfg.SelectedProviderID = id
	if e = a.saveAppOnly(cfg); e != nil {
		return config.AppState{}, e
	}
	return a.state(cfg), nil
}

func (a *App) SetSelectedModel(providerID, modelID string) (config.AppState, error) {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	return a.setSelectedModelLocked(providerID, modelID)
}

func (a *App) setSelectedModelLocked(providerID, modelID string) (config.AppState, error) {
	cfg, e := a.loadClone()
	if e != nil {
		return config.AppState{}, e
	}
	p, e := cfg.ProviderByID(providerID)
	if e != nil {
		return config.AppState{}, e
	}
	if e = provider.EnsureModel(p, modelID); e != nil {
		return config.AppState{}, e
	}
	p.SelectedModelID = modelID
	cfg.UpsertProvider(p, providerID)
	cfg.SelectedProviderID = providerID
	if e = a.saveAppOnly(cfg); e != nil {
		return config.AppState{}, e
	}
	return a.state(cfg), nil
}

func (a *App) UpdateModelRoles(updates []config.RoleUpdate) (config.AppState, error) {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	cfg, e := a.loadClone()
	if e != nil {
		return config.AppState{}, e
	}
	for _, u := range updates {
		if !omp.IsManagedRole(u.Role) {
			return config.AppState{}, errors.New("未知模型角色：" + u.Role)
		}
		if u.Clear {
			delete(cfg.ModelRoles, u.Role)
			continue
		}
		if _, _, _, ok := omp.ParseManagedSelector(u.Selector, cfg.Providers); !ok {
			return config.AppState{}, fmt.Errorf("模型角色 %s 的 selector 无效", u.Role)
		}
		cfg.ModelRoles[u.Role] = u.Selector
	}
	if e = a.saveOMP(cfg); e != nil {
		return config.AppState{}, e
	}
	return a.state(cfg), nil
}

func (a *App) UpdateSettings(input config.AppSettings) (config.AppState, error) {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	cfg, e := a.loadClone()
	if e != nil {
		return config.AppState{}, e
	}
	input.LastUpdateCheckAtUnix = cfg.Settings.LastUpdateCheckAtUnix
	normalized := config.NormalizeSettings(input)
	pathsChanged := normalized.CustomPaths != cfg.Settings.CustomPaths
	cfg.Settings = normalized
	if e = a.saveAppOnly(cfg); e != nil {
		return config.AppState{}, e
	}
	// Custom paths changed: rebuild paths + service so the new OMP locations
	// take effect immediately (no restart required). config.json and backups
	// stay on the Windows host, so the watcher keeps working.
	if pathsChanged {
		a.paths = paths.ApplyCustomPaths(a.paths, cfg.Settings.CustomPaths.OMPModelsPath, cfg.Settings.CustomPaths.OMPConfigPath, cfg.Settings.CustomPaths.OMPSessionsDir)
		a.service = config.NewService(a.paths)
		// Re-import providers and roles from the newly targeted OMP install
		// (e.g. a WSL distro) so the UI immediately shows its models instead
		// of the previous installation's.
		imported, importErr := a.service.ImportFromOMP(cfg)
		if importErr != nil {
			return config.AppState{}, fmt.Errorf("路径已保存，但无法从新路径导入 OMP 配置：%w", importErr)
		}
		cfg = imported
		if e = a.saveOMP(cfg); e != nil {
			return config.AppState{}, e
		}
	}
	return a.state(cfg), nil
}

func (a *App) LaunchOMP(providerID, modelID string) (omp.LaunchPreview, error) {
	cfg, e := a.service.Load()
	if e != nil {
		return omp.LaunchPreview{}, e
	}
	p, e := cfg.ProviderByID(providerID)
	if e != nil {
		return omp.LaunchPreview{}, e
	}
	if e = provider.EnsureModel(p, modelID); e != nil {
		return omp.LaunchPreview{}, e
	}
	return omp.BuildLaunch(cfg.Settings.OMPCommand, providerID, modelID)
}

func (a *App) ExecuteLaunchOMP(providerID, modelID string) error {
	a.mutationMu.Lock()
	cfg, e := a.loadClone()
	if e != nil {
		a.mutationMu.Unlock()
		return e
	}
	p, e := cfg.ProviderByID(providerID)
	if e != nil {
		a.mutationMu.Unlock()
		return e
	}
	if e = provider.EnsureModel(p, modelID); e != nil {
		a.mutationMu.Unlock()
		return e
	}
	wslMode := cfg.Settings.LaunchMode == "wsl"
	ompCommand := cfg.Settings.OMPCommand
	if wslMode {
		// Inside a WSL distro OMP is invoked by its shell name, never by a
		// Windows path. Ignore the Windows-side command string.
		ompCommand = "omp"
	}
	preview, e := omp.BuildLaunch(ompCommand, providerID, modelID)
	if e != nil {
		a.mutationMu.Unlock()
		return e
	}
	var workingDir string
	if wslMode {
		// WSL mode still asks the user to pick a working directory, but the
		// Windows picker result is mapped to the WSL mount point (/mnt/...).
		defaultDir := omp.WSLToWindowsPath(cfg.Settings.WorkingDir)
		picked, pickErr := a.pickWSLDirectory(defaultDir)
		if pickErr != nil {
			a.mutationMu.Unlock()
			return pickErr
		}
		if picked == "" {
			a.mutationMu.Unlock()
			return errors.New("已取消启动")
		}
		workingDir = omp.WindowsToWSLPath(picked)
		cfg.Settings.WorkingDir = workingDir
	} else {
		workingDir, e = a.chooseLaunchDirectory(cfg.Settings.WorkingDir)
		if e != nil || workingDir == "" {
			a.mutationMu.Unlock()
			return e
		}
		cfg.Settings.WorkingDir = workingDir
	}
	p.SelectedModelID = modelID
	cfg.UpsertProvider(p, providerID)
	cfg.SelectedProviderID = providerID
	if e = a.saveAppOnly(cfg); e != nil {
		a.mutationMu.Unlock()
		return e
	}
	a.mutationMu.Unlock()

	if wslMode {
		return a.startWSLOMP(wslLaunchConfig{
			ompCommand: ompCommand,
			args:       preview.Arguments,
			distro:     cfg.Settings.WSLDistro,
			workingDir: workingDir,
		})
	}
	return a.startManagedOMP(preview, workingDir)
}

func (a *App) ListSessions() ([]sessions.Info, error) {
	return sessions.NewManager(a.paths.OMPSessionsDir).List()
}

func (a *App) DeleteSession(id string) ([]sessions.Info, error) {
	manager := sessions.NewManager(a.paths.OMPSessionsDir)
	if err := manager.Delete(id); err != nil {
		return nil, err
	}
	return manager.List()
}

func (a *App) ContinueSession(id string) error {
	cfg, err := a.service.Load()
	if err != nil {
		return err
	}
	info, sessionPath, err := sessions.NewManager(a.paths.OMPSessionsDir).Find(id)
	if err != nil {
		return err
	}
	wslMode := cfg.Settings.LaunchMode == "wsl"
	ompCommand := cfg.Settings.OMPCommand
	if wslMode {
		ompCommand = "omp"
		// Sessions are stored under a UNC path (\\wsl.localhost\...); omp
		// inside the distro needs the Linux path (/home/...).
		linuxPath := omp.WSLUNCToWSLPath(sessionPath, cfg.Settings.WSLDistro)
		if linuxPath == "" {
			linuxPath = omp.WindowsToWSLPath(sessionPath)
		}
		if linuxPath == "" || strings.HasPrefix(strings.ToLower(linuxPath), "//") {
			return errors.New("无法将会话路径映射到 WSL 发行版：" + sessionPath)
		}
		sessionPath = linuxPath
	}
	preview, err := omp.BuildResumeLaunch(ompCommand, sessionPath)
	if err != nil {
		return err
	}
	if wslMode {
		return a.startWSLOMP(wslLaunchConfig{
			ompCommand: ompCommand,
			args:       preview.Arguments,
			distro:     cfg.Settings.WSLDistro,
			workingDir: info.WorkingDir,
		})
	}
	return a.startManagedOMP(preview, info.WorkingDir)
}

func (a *App) pickWSLDirectory(defaultDirectory string) (string, error) {
	if a.selectLaunchDir != nil {
		return a.selectLaunchDir(defaultDirectory)
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择 OMP 启动目录（将自动映射为 WSL 路径）", DefaultDirectory: defaultDirectory})
}

func (a *App) chooseLaunchDirectory(defaultDirectory string) (string, error) {
	if a.selectLaunchDir != nil {
		return a.selectLaunchDir(defaultDirectory)
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择 OMP 启动目录", DefaultDirectory: defaultDirectory})
}

func (a *App) ListGlobalSkills() (skills.Inventory, error) {
	return skills.Manager{Root: a.paths.OMPGlobalSkillsDir, LockPath: a.paths.OMPGlobalSkillsLock}.List()
}

func (a *App) DeleteGlobalSkill(name string) (skills.Inventory, error) {
	return skills.Manager{Root: a.paths.OMPGlobalSkillsDir, LockPath: a.paths.OMPGlobalSkillsLock}.Delete(name)
}

func (a *App) TestModel(providerID, modelID string) (provider.ConnectionTestResult, error) {
	cfg, err := a.service.Load()
	if err != nil {
		return provider.ConnectionTestResult{}, err
	}
	configured, err := cfg.ProviderByID(strings.TrimSpace(providerID))
	if err != nil {
		return provider.ConnectionTestResult{}, err
	}
	modelID = strings.TrimSpace(modelID)
	if err = provider.EnsureModel(configured, modelID); err != nil {
		return provider.ConnectionTestResult{}, err
	}
	var model provider.ModelInfo
	for _, candidate := range configured.Models {
		if candidate.ID == modelID {
			model = candidate
			break
		}
	}
	key, commandValue := provider.ResolveAPIKey(configured.APIKey, os.LookupEnv)
	if commandValue {
		return provider.ConnectionTestResult{}, errors.New("API Key 是命令形式，Switch 不会执行命令")
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	requestContext, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	return provider.TestModel(requestContext, configured, model, key, provider.ModelTestOptions{})
}

// PathDetectResult reports detected OMP file locations for a launch mode.
type PathDetectResult struct {
	Mode        string            `json:"mode"`
	HomeDir     string            `json:"homeDir"`
	CustomPaths config.CustomPaths `json:"customPaths"`
	Detected    bool              `json:"detected"`
	Message     string            `json:"message"`
}

// WSLDistro describes one installed WSL distribution. ID is the stable
// identifier passed to wsl.exe -d; Name is the display name.
type WSLDistro struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Version   int    `json:"version"`
	IsDefault bool   `json:"isDefault"`
}

// ensureOMPFiles creates the OMP directory layout (agent dir, sessions dir)
// and default models.yml/config.yml when they are missing. Existing files are
// never overwritten — an OMP install that already wrote its own config keeps
// it untouched.
func ensureOMPFiles(modelsPath, configPath, sessionsDir string) error {
	if err := os.MkdirAll(filepath.Dir(modelsPath), 0o755); err != nil {
		return fmt.Errorf("创建 OMP 目录: %w", err)
	}
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		return fmt.Errorf("创建会话目录: %w", err)
	}
	if _, err := os.Stat(modelsPath); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(modelsPath, []byte("providers: {}\n"), 0o644); err != nil {
			return fmt.Errorf("创建 %s: %w", modelsPath, err)
		}
	}
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(configPath, []byte("{}\n"), 0o644); err != nil {
			return fmt.Errorf("创建 %s: %w", configPath, err)
		}
	}
	return nil
}

// ResolveWSLPaths detects the default OMP file locations inside the given WSL
// distro. It queries the distro for its HOME directory via wsl.exe -d <distro> -- printenv HOME, then
// constructs UNC paths (\\wsl.localhost\<distro>\home\...). Paths whose
// files actually exist are returned in CustomPaths, ready to save.
func (a *App) ResolveWSLPaths(distro string) (PathDetectResult, error) {
	distro = strings.TrimSpace(distro)
	if distro == "" {
		return PathDetectResult{}, errors.New("请先选择或填写 WSL 发行版名称")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "wsl.exe", "-d", distro, "--", "printenv", "HOME")
	raw, err := cmd.Output()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return PathDetectResult{Mode: "wsl", Message: "检测超时，请确认 WSL 发行版可用"}, nil
		}
		return PathDetectResult{Mode: "wsl", Message: "无法连接 WSL 发行版：" + strings.TrimSpace(string(raw))}, nil
	}
	home := strings.TrimSpace(string(raw))
	if home == "" || !strings.HasPrefix(home, "/") {
		return PathDetectResult{Mode: "wsl", Message: "无法读取 WSL 主目录"}, nil
	}
	result := PathDetectResult{Mode: "wsl", HomeDir: home}
	// Convert /home/admin to home\admin and build UNC paths. The
	// double backslash prefix is required for a Windows UNC path
	// (\\wsl.localhost\...). filepath.Join collapses leading separators,
	// so build the path by hand to preserve the UNC prefix exactly.
	rel := filepath.FromSlash(strings.TrimPrefix(home, "/"))
	base := "\\\\wsl.localhost\\" + distro + "\\" + filepath.Join(rel, ".omp", "agent")
	result.CustomPaths.OMPModelsPath = base + "\\models.yml"
	result.CustomPaths.OMPConfigPath = base + "\\config.yml"
	result.CustomPaths.OMPSessionsDir = base + "\\sessions"
	if info, e := os.Stat(result.CustomPaths.OMPModelsPath); e == nil && !info.IsDir() {
		result.Detected = true
		result.Message = "已检测到 OMP Models（models.yml）"
	} else {
		if ensureErr := ensureOMPFiles(result.CustomPaths.OMPModelsPath, result.CustomPaths.OMPConfigPath, result.CustomPaths.OMPSessionsDir); ensureErr != nil {
			return PathDetectResult{Mode: "wsl", HomeDir: home, CustomPaths: result.CustomPaths, Message: "创建 OMP 配置失败：" + ensureErr.Error()}, nil
		}
		result.Detected = true
		result.Message = "未找到 models.yml，已自动创建默认配置（空供应商列表）"
	}
	return result, nil
}

func (a *App) OpenConfigFolder() error {
	runtime.BrowserOpenURL(a.ctx, "file:///"+filepath.ToSlash(filepath.Dir(a.paths.OMPSwitchConfigPath)))
	return nil
}

func (a *App) CheckEnvVar(name string) (system.EnvCheckResult, error) {
	return system.CheckEnvVar(name), nil
}

// ResolveNativePaths detects OMP file locations for a local Windows install.
// It fills the default ~/.omp/agent/ paths and marks detected when models.yml
// actually exists.
func (a *App) ResolveNativePaths() (PathDetectResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return PathDetectResult{}, fmt.Errorf("获取用户主目录: %w", err)
	}
	rec := paths.DefaultPaths()
	result := PathDetectResult{Mode: "native", HomeDir: home}
	result.CustomPaths.OMPModelsPath = rec.OMPModelsPath
	result.CustomPaths.OMPConfigPath = rec.OMPConfigPath
	result.CustomPaths.OMPSessionsDir = rec.OMPSessionsDir
	if info, e := os.Stat(rec.OMPModelsPath); e == nil && !info.IsDir() {
		result.Detected = true
		result.Message = "已检测到本地 OMP Models（models.yml）"
	} else {
		if ensureErr := ensureOMPFiles(rec.OMPModelsPath, rec.OMPConfigPath, rec.OMPSessionsDir); ensureErr != nil {
			return PathDetectResult{Mode: "native", HomeDir: home, CustomPaths: result.CustomPaths, Message: "创建 OMP 配置失败：" + ensureErr.Error()}, nil
		}
		result.Detected = true
		result.Message = "本地未找到 models.yml，已自动创建默认配置（空供应商列表）"
	}
	return result, nil
}

// ListWSLDistros enumerates installed WSL distributions. It prefers the
// verbose table (ID, WSL version, default marker) and falls back to the
// quiet name list on older WSL versions. Returns an empty slice if WSL is
// not installed or the command times out (3s).
func (a *App) ListWSLDistros() ([]WSLDistro, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "wsl.exe", "--list", "--verbose")
	raw, err := cmd.Output()
	if err == nil {
		if distros := parseWSLDistroVerbose(raw); len(distros) > 0 {
			return distros, nil
		}
	}
	// Fallback: silent name list.
	cmd2 := exec.CommandContext(ctx, "wsl.exe", "--list", "--quiet")
	raw2, err2 := cmd2.Output()
	if err2 != nil {
		return []WSLDistro{}, nil
	}
	names := parseWSLDistroOutput(raw2)
	distros := make([]WSLDistro, 0, len(names))
	for _, name := range names {
		distros = append(distros, WSLDistro{ID: name, Name: name})
	}
	return distros, nil
}

// decodeUTF16LE decodes UTF-16LE bytes (optionally with a BOM) to a string.
func decodeUTF16LE(raw []byte) string {
	if len(raw) < 2 {
		return ""
	}
	start := 0
	if raw[0] == 0xFF && raw[1] == 0xFE {
		start = 2
	}
	if (len(raw)-start)%2 != 0 {
		return ""
	}
	codes := make([]uint16, 0, (len(raw)-start)/2)
	for i := start; i+1 < len(raw); i += 2 {
		codes = append(codes, binary.LittleEndian.Uint16(raw[i:i+2]))
	}
	return string(utf16.Decode(codes))
}

// parseWSLDistroOutput decodes UTF-16LE output from \`wsl.exe --list --quiet\`
// and returns trimmed distro names. Returns empty slice on decode failure.
func parseWSLDistroOutput(raw []byte) []string {
	decoded := decodeUTF16LE(raw)
	if decoded == "" {
		return []string{}
	}
	lines := strings.Split(decoded, "\n")
	distros := make([]string, 0, len(lines))
	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name != "" {
			distros = append(distros, name)
		}
	}
	return distros
}

// parseWSLDistroVerbose parses the table from \`wsl.exe --list --verbose\`:
//
//	NAME                   STATE           VERSION
//	* Ubuntu               Running         2
//	  Debian               Stopped         1
//
// The ID equals the NAME column (the value passed to wsl.exe -d).
func parseWSLDistroVerbose(raw []byte) []WSLDistro {
	decoded := decodeUTF16LE(raw)
	if decoded == "" {
		return []WSLDistro{}
	}
	var distros []WSLDistro
	for _, line := range strings.Split(decoded, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// Skip table header row. wsl.exe localizes it: "NAME STATE VERSION"
		// (en) or "名称 状态 版本" (zh-CN). A valid data row always has the
		// distro name followed by at least STATE and VERSION; the header has
		// no numeric version field, so require one instead of matching text.
		hasVersion := false
		for _, f := range fields {
			if _, err := strconv.Atoi(f); err == nil {
				hasVersion = true
				break
			}
		}
		if strings.HasPrefix(fields[0], "*") {
			if len(fields) < 4 || !hasVersion {
				continue
			}
		} else if len(fields) < 3 || !hasVersion {
			continue
		}
		distro := WSLDistro{}
		rest := fields
		if rest[0] == "*" {
			distro.IsDefault = true
			rest = rest[1:]
		}
		if len(rest) < 1 {
			continue
		}
		distro.ID = rest[0]
		distro.Name = rest[0]
		for _, field := range rest[1:] {
			if n, err := strconv.Atoi(field); err == nil {
				distro.Version = n
				break
			}
		}
		distros = append(distros, distro)
	}
	return distros
}

func (a *App) startBackgroundUpdateCheck() {
	cfg, e := a.service.Load()
	if e != nil || !updater.NeedsCheck(cfg.Settings) {
		return
	}
	go func() {
		r, e := updater.CheckLatest(appVersion)
		_ = a.recordUpdateCheck()
		if e == nil && r.HasUpdate {
			runtime.EventsEmit(a.ctx, updateAvailableEvent, r)
		}
	}()
}

func (a *App) recordUpdateCheck() error {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	cfg, e := a.loadClone()
	if e != nil {
		return e
	}
	cfg.Settings.LastUpdateCheckAtUnix = time.Now().Unix()
	return a.saveAppOnly(cfg)
}

func (a *App) CheckForUpdate() (updater.CheckResult, error) {
	r, e := updater.CheckLatest(appVersion)
	_ = a.recordUpdateCheck()
	return r, e
}

func (a *App) MarkUpdateChecked() error { return a.recordUpdateCheck() }

func (a *App) InstallUpdate() error {
	r, e := updater.CheckLatest(appVersion)
	if e != nil {
		return e
	}
	if !r.HasUpdate || r.AssetURL == "" {
		return errors.New("当前已是最新版本，无需更新")
	}
	f, e := os.CreateTemp("", "OMPSwitch-update-*.exe")
	if e != nil {
		return e
	}
	p := f.Name()
	_ = f.Close()
	if e = updater.Download(r.AssetURL, p); e != nil {
		_ = os.Remove(p)
		return e
	}
	return updater.Install(p)
}

func (a *App) loadClone() (config.SwitchConfig, error) {
	cfg, e := a.service.Load()
	if e != nil {
		return config.SwitchConfig{}, e
	}
	return cloneSwitchConfig(cfg), nil
}

func (a *App) saveOMP(cfg config.SwitchConfig) error {
	if e := validateSwitchConfig(cfg); e != nil {
		return e
	}
	a.markSelfWrite()
	return a.service.SaveOMPState(cfg)
}

func (a *App) saveAppOnly(cfg config.SwitchConfig) error {
	if e := validateSwitchConfig(cfg); e != nil {
		return e
	}
	a.markSelfWrite()
	return a.service.SaveAppOnly(cfg)
}

func providerFromInput(i provider.SaveInput) provider.Config {
	return provider.Config{ID: i.ID, Name: i.ID, BaseURL: i.BaseURL, APIKey: strings.TrimSpace(i.APIKey), API: i.API, HeaderMode: i.HeaderMode, Headers: cloneStringMap(i.Headers), CustomHeaders: cloneStringMap(i.CustomHeaders), Models: []provider.ModelInfo{}}
}

func availableProviderID(requested string, providers []provider.Config, exclude string) string {
	used := map[string]struct{}{}
	for _, p := range providers {
		if p.ID != exclude {
			used[p.ID] = struct{}{}
		}
	}
	if _, ok := used[requested]; !ok {
		return requested
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", requested, n)
		if _, ok := used[candidate]; !ok {
			return candidate
		}
	}
}

func cloneSwitchConfig(in config.SwitchConfig) config.SwitchConfig {
	out := in
	out.Providers = make([]provider.Config, len(in.Providers))
	for i, p := range in.Providers {
		p.Headers = cloneStringMap(p.Headers)
		p.CustomHeaders = cloneStringMap(p.CustomHeaders)
		p.Models = cloneModels(p.Models)
		out.Providers[i] = p
	}
	out.ModelRoles = cloneStringMap(in.ModelRoles)
	return out
}

func cloneModels(in []provider.ModelInfo) []provider.ModelInfo {
	out := make([]provider.ModelInfo, len(in))
	for i, m := range in {
		if m.Reasoning != nil {
			v := *m.Reasoning
			m.Reasoning = &v
		}
		out[i] = m
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func validateSwitchConfig(cfg config.SwitchConfig) error {
	for _, p := range cfg.Providers {
		if e := config.ValidateProvider(p); e != nil {
			return fmt.Errorf("Provider %s: %w", p.ID, e)
		}
	}
	return nil
}

func init() { _ = os.Setenv("WAILS_SAVE_FILE_OVERWRITE_PROMPT", "false") }
