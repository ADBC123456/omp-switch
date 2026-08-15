package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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

var appVersion = "1.1.0"

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
	watcher             *fsnotify.Watcher
	watcherStop         chan struct{}
	writeMu             sync.Mutex
	lastSelfWrite       time.Time
}

func NewApp() *App {
	p := paths.DefaultPaths()
	app := &App{service: config.NewService(p), paths: p, discoveryByRequest: map[string]context.CancelFunc{}, discoveryByProvider: map[string]string{}}
	app.startManagedOMP = func(preview omp.LaunchPreview, workingDir string) error {
		_, err := omp.StartManaged(preview, workingDir)
		return err
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
	_, _ = a.service.Load()
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
	cfg, e := a.service.Load()
	if e != nil {
		return config.AppState{}, e
	}
	return a.state(cfg), nil
}
func (a *App) ListProviders() ([]provider.View, error) {
	state, e := a.GetAppState()
	return state.Providers, e
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
	cfg.Settings = config.NormalizeSettings(input)
	if e = a.saveAppOnly(cfg); e != nil {
		return config.AppState{}, e
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
	preview, e := omp.BuildLaunch(cfg.Settings.OMPCommand, providerID, modelID)
	if e != nil {
		a.mutationMu.Unlock()
		return e
	}
	workingDir, e := a.chooseLaunchDirectory(cfg.Settings.WorkingDir)
	if e != nil || workingDir == "" {
		a.mutationMu.Unlock()
		return e
	}
	p.SelectedModelID = modelID
	cfg.UpsertProvider(p, providerID)
	cfg.SelectedProviderID = providerID
	cfg.Settings.WorkingDir = workingDir
	if e = a.saveAppOnly(cfg); e != nil {
		a.mutationMu.Unlock()
		return e
	}
	a.mutationMu.Unlock()

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
	preview, err := omp.BuildResumeLaunch(cfg.Settings.OMPCommand, sessionPath)
	if err != nil {
		return err
	}
	return a.startManagedOMP(preview, info.WorkingDir)
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
func (a *App) OpenConfigFolder() error {
	runtime.BrowserOpenURL(a.ctx, "file:///"+filepath.ToSlash(filepath.Dir(a.paths.OMPSwitchConfigPath)))
	return nil
}
func (a *App) CheckEnvVar(name string) (system.EnvCheckResult, error) {
	return system.CheckEnvVar(name), nil
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
