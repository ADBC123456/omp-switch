package provider

import (
	"errors"
	"fmt"
	"strings"
)

type Config struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	BaseURL         string            `json:"baseUrl"`
	APIKey          string            `json:"apiKey"`
	API             string            `json:"api"`
	HeaderMode      string            `json:"headerMode"`
	Headers         map[string]string `json:"headers"`
	CustomHeaders   map[string]string `json:"customHeaders,omitempty"`
	Models          []ModelInfo       `json:"models"`
	SelectedModelID string            `json:"selectedModelId"`
}

type ModelInfo struct {
	ID            string `json:"id" yaml:"id"`
	Name          string `json:"name,omitempty" yaml:"name,omitempty"`
	API           string `json:"api,omitempty" yaml:"api,omitempty"`
	Reasoning     *bool  `json:"reasoning,omitempty" yaml:"reasoning,omitempty"`
	ContextWindow int    `json:"contextWindow,omitempty" yaml:"contextWindow,omitempty"`
	MaxTokens     int    `json:"maxTokens,omitempty" yaml:"maxTokens,omitempty"`
}

type SaveInput struct {
	ID            string            `json:"id"`
	BaseURL       string            `json:"baseUrl"`
	APIKey        string            `json:"apiKey"`
	API           string            `json:"api"`
	HeaderMode    string            `json:"headerMode"`
	Headers       map[string]string `json:"headers"`
	CustomHeaders map[string]string `json:"customHeaders"`
}

type View struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	BaseURL         string            `json:"baseUrl"`
	APIKey          string            `json:"apiKey"`
	HasAPIKey       bool              `json:"hasApiKey"`
	API             string            `json:"api"`
	HeaderMode      string            `json:"headerMode"`
	Headers         map[string]string `json:"headers"`
	CustomHeaders   map[string]string `json:"customHeaders"`
	Models          []ModelInfo       `json:"models"`
	SelectedModelID string            `json:"selectedModelId"`
}

func NewView(cfg Config) View {
	return View{
		ID:              cfg.ID,
		Name:            cfg.Name,
		BaseURL:         cfg.BaseURL,
		APIKey:          "",
		HasAPIKey:       strings.TrimSpace(cfg.APIKey) != "",
		API:             cfg.API,
		HeaderMode:      cfg.HeaderMode,
		Headers:         cfg.Headers,
		CustomHeaders:   cfg.CustomHeaders,
		Models:          cfg.Models,
		SelectedModelID: cfg.SelectedModelID,
	}
}

type ConnectionTestResult struct {
	OK    bool     `json:"ok"`
	Title string   `json:"title"`
	Lines []string `json:"lines"`
}

func Normalize(input Config) Config {
	input.ID = strings.TrimSpace(input.ID)
	input.Name = input.ID
	input.BaseURL = strings.TrimSpace(input.BaseURL)
	input.API = strings.TrimSpace(input.API)
	input.HeaderMode = strings.TrimSpace(input.HeaderMode)
	input.Models = NormalizeModels(input.Models)
	if input.Headers == nil {
		input.Headers = map[string]string{}
	}
	if input.CustomHeaders == nil {
		input.CustomHeaders = map[string]string{}
	}
	if input.HeaderMode == "" {
		input.HeaderMode = "none"
	}
	if !hasModel(input.Models, input.SelectedModelID) {
		input.SelectedModelID = ""
		if len(input.Models) > 0 {
			input.SelectedModelID = input.Models[0].ID
		}
	}
	return input
}

func NormalizeModels(models []ModelInfo) []ModelInfo {
	if len(models) == 0 {
		return nil
	}
	normalized := make([]ModelInfo, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		model.ID = strings.TrimSpace(model.ID)
		model.Name = strings.TrimSpace(model.Name)
		model.API = strings.TrimSpace(model.API)
		if model.ID == "" {
			continue
		}
		if _, exists := seen[model.ID]; exists {
			continue
		}
		seen[model.ID] = struct{}{}
		normalized = append(normalized, model)
	}
	return normalized
}

func MergeModels(existing []ModelInfo, incoming []ModelInfo) []ModelInfo {
	merged := NormalizeModels(existing)
	indexByID := make(map[string]int, len(merged))
	for index, model := range merged {
		indexByID[model.ID] = index
	}
	for _, model := range NormalizeModels(incoming) {
		if existingIndex, ok := indexByID[model.ID]; ok {
			merged[existingIndex] = model
			continue
		}
		indexByID[model.ID] = len(merged)
		merged = append(merged, model)
	}
	return merged
}

func EnsureModel(config Config, modelID string) error {
	if hasModel(config.Models, modelID) {
		return nil
	}
	return errors.New("模型不存在：" + modelID)
}

func hasModel(models []ModelInfo, modelID string) bool {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return false
	}
	for _, model := range models {
		if model.ID == modelID {
			return true
		}
	}
	return false
}

func FormatModelCount(count int) string {
	return fmt.Sprintf("%d 个", count)
}
