package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"ompswitch/internal/config"
	"ompswitch/internal/provider"
)

const modelDiscoveryTimeout = 60 * time.Second

func (a *App) FetchModels(providerID, requestID string) (provider.DiscoveryResult, error) {
	providerID = strings.TrimSpace(providerID)
	requestID = strings.TrimSpace(requestID)
	if providerID == "" {
		return provider.DiscoveryResult{}, errors.New("Provider ID 不能为空")
	}
	if requestID == "" {
		return provider.DiscoveryResult{}, errors.New("请求 ID 不能为空")
	}

	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, modelDiscoveryTimeout)

	a.discoveryMu.Lock()
	if a.discoveryByRequest == nil {
		a.discoveryByRequest = make(map[string]context.CancelFunc)
	}
	if a.discoveryByProvider == nil {
		a.discoveryByProvider = make(map[string]string)
	}
	if _, exists := a.discoveryByRequest[requestID]; exists {
		a.discoveryMu.Unlock()
		cancel()
		return provider.DiscoveryResult{}, errors.New("请求 ID 正在获取模型")
	}
	if _, exists := a.discoveryByProvider[providerID]; exists {
		a.discoveryMu.Unlock()
		cancel()
		return provider.DiscoveryResult{}, errors.New("该 Provider 正在获取模型")
	}
	a.discoveryByRequest[requestID] = cancel
	a.discoveryByProvider[providerID] = requestID
	a.discoveryMu.Unlock()

	defer func() {
		cancel()
		a.discoveryMu.Lock()
		delete(a.discoveryByRequest, requestID)
		if activeRequest, exists := a.discoveryByProvider[providerID]; exists && activeRequest == requestID {
			delete(a.discoveryByProvider, providerID)
		}
		a.discoveryMu.Unlock()
	}()

	cfg, err := a.service.Load()
	if err != nil {
		return provider.DiscoveryResult{}, err
	}
	current, err := cfg.ProviderByID(providerID)
	if err != nil {
		return provider.DiscoveryResult{}, err
	}
	key, commandValue := provider.ResolveAPIKey(current.APIKey, os.LookupEnv)
	if commandValue {
		return provider.DiscoveryResult{}, errors.New("API Key 是命令形式，Switch 不会执行命令；请手工添加模型")
	}
	result, err := provider.DiscoverModels(ctx, current, key, provider.DiscoveryOptions{})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return provider.DiscoveryResult{}, errors.New("已取消获取模型")
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return provider.DiscoveryResult{}, errors.New("获取模型超时")
		}
		return provider.DiscoveryResult{}, err
	}
	return result, nil
}

func (a *App) CancelModelDiscovery(requestID string) error {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return errors.New("请求 ID 不能为空")
	}
	a.discoveryMu.Lock()
	cancel, exists := a.discoveryByRequest[requestID]
	a.discoveryMu.Unlock()
	if !exists {
		return errors.New("未找到获取模型请求")
	}
	cancel()
	return nil
}

func (a *App) ImportDiscoveredModels(providerID string, selected []provider.ModelInfo) (config.AppState, error) {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()

	cfg, err := a.service.Load()
	if err != nil {
		return config.AppState{}, err
	}
	current, err := cfg.ProviderByID(strings.TrimSpace(providerID))
	if err != nil {
		return config.AppState{}, err
	}

	validated := make([]provider.ModelInfo, len(selected))
	copy(validated, selected)
	for index := range validated {
		validated[index].ID = strings.TrimSpace(validated[index].ID)
		validated[index].Name = strings.TrimSpace(validated[index].Name)
		validated[index].API = strings.TrimSpace(validated[index].API)
	}
	probe := current
	probe.Models = validated
	probe.SelectedModelID = ""
	if err := config.ValidateProvider(probe); err != nil {
		return config.AppState{}, err
	}

	indexByID := make(map[string]int, len(current.Models))
	for index, model := range current.Models {
		indexByID[model.ID] = index
	}
	for _, model := range validated {
		if index, exists := indexByID[model.ID]; exists {
			current.Models[index] = enrichImportedModel(current.Models[index], model)
			continue
		}
		indexByID[model.ID] = len(current.Models)
		current.Models = append(current.Models, model)
	}
	current = provider.Normalize(current)
	cfg.UpsertProvider(current, current.ID)

	a.markSelfWrite()
	if err := a.service.SaveOMPState(cfg); err != nil {
		return config.AppState{}, err
	}
	return a.state(cfg), nil
}

func enrichImportedModel(existing, discovered provider.ModelInfo) provider.ModelInfo {
	if existing.Name == "" {
		existing.Name = discovered.Name
	}
	if existing.API == "" {
		existing.API = discovered.API
	}
	if existing.Reasoning == nil {
		existing.Reasoning = discovered.Reasoning
	}
	if existing.ContextWindow == 0 {
		existing.ContextWindow = discovered.ContextWindow
	}
	if existing.MaxTokens == 0 {
		existing.MaxTokens = discovered.MaxTokens
	}
	return existing
}
