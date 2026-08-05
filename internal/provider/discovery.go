package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	defaultDiscoveryPageTimeout = 12 * time.Second
	defaultDiscoveryMaxPages    = 20
	maxDiscoveryErrorBody       = 4 << 10
)

type DiscoveryResult struct {
	Models   []ModelInfo `json:"models"`
	Warnings []string    `json:"warnings"`
}

type DiscoveryOptions struct {
	Client      *http.Client
	PageTimeout time.Duration
	MaxPages    int
}

func ResolveAPIKey(value string, lookupEnv func(string) (string, bool)) (key string, commandValue bool) {
	key = strings.TrimSpace(value)
	if lookupEnv != nil {
		if environmentValue, found := lookupEnv(key); found && strings.TrimSpace(environmentValue) != "" {
			key = strings.TrimSpace(environmentValue)
		}
	}
	return key, strings.HasPrefix(key, "!")
}

func DiscoverModels(ctx context.Context, cfg Config, resolvedKey string, options DiscoveryOptions) (DiscoveryResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	parsedURL, err := url.Parse(strings.TrimSpace(cfg.BaseURL))
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return DiscoveryResult{}, errors.New("Base URL 必须是绝对 URL")
	}
	parsedURL.Path = strings.TrimRight(parsedURL.Path, "/") + "/models"
	parsedURL.Fragment = ""

	client := options.Client
	if client == nil {
		client = http.DefaultClient
	}
	pageTimeout := options.PageTimeout
	if pageTimeout <= 0 {
		pageTimeout = defaultDiscoveryPageTimeout
	}
	maxPages := options.MaxPages
	if maxPages <= 0 {
		maxPages = defaultDiscoveryMaxPages
	}

	warnings, headers := discoveryHeaders(cfg.Headers)
	models := make([]ModelInfo, 0)
	indexByID := make(map[string]int)
	seenCursors := make(map[string]struct{})

	for page := 1; page <= maxPages; page++ {
		pageContext, cancelPage := context.WithTimeout(ctx, pageTimeout)
		request, requestErr := http.NewRequestWithContext(pageContext, http.MethodGet, parsedURL.String(), nil)
		if requestErr != nil {
			cancelPage()
			return DiscoveryResult{}, requestErr
		}
		if strings.TrimSpace(resolvedKey) != "" {
			request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(resolvedKey))
		}
		for name, value := range headers {
			request.Header.Set(name, value)
		}

		response, requestErr := client.Do(request)
		if requestErr != nil {
			cancelPage()
			return DiscoveryResult{}, fmt.Errorf("获取模型第 %d 页: %w", page, requestErr)
		}
		body, readErr := readDiscoveryResponse(response)
		cancelPage()
		if readErr != nil {
			return DiscoveryResult{}, fmt.Errorf("获取模型第 %d 页: %w", page, readErr)
		}
		parsedPage, parseErr := parseDiscoveryPage(body)
		if parseErr != nil {
			return DiscoveryResult{}, fmt.Errorf("解析模型第 %d 页: %w", page, parseErr)
		}
		for _, model := range parsedPage.models {
			if index, exists := indexByID[model.ID]; exists {
				models[index] = enrichDiscoveredModel(models[index], model)
				continue
			}
			indexByID[model.ID] = len(models)
			models = append(models, model)
		}
		if parsedPage.cursor == "" {
			return DiscoveryResult{Models: models, Warnings: warnings}, nil
		}
		cursorKey := parsedPage.cursorParameter + "\x00" + parsedPage.cursor
		if _, exists := seenCursors[cursorKey]; exists {
			return DiscoveryResult{}, errors.New("模型分页游标循环")
		}
		seenCursors[cursorKey] = struct{}{}
		if page == maxPages {
			return DiscoveryResult{}, fmt.Errorf("模型分页超过 %d 页上限", maxPages)
		}
		query := parsedURL.Query()
		query.Set(parsedPage.cursorParameter, parsedPage.cursor)
		parsedURL.RawQuery = query.Encode()
	}
	return DiscoveryResult{}, fmt.Errorf("模型分页超过 %d 页上限", maxPages)
}

func discoveryHeaders(configured map[string]string) ([]string, map[string]string) {
	warnings := make([]string, 0)
	headers := make(map[string]string, len(configured))
	for name, value := range configured {
		if strings.HasPrefix(strings.TrimSpace(value), "!") {
			warnings = append(warnings, name)
			continue
		}
		headers[name] = value
	}
	sort.Strings(warnings)
	for index := range warnings {
		warnings[index] = "未发送命令形式 Header：" + warnings[index]
	}
	return warnings, headers
}

func readDiscoveryResponse(response *http.Response) ([]byte, error) {
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, err := io.ReadAll(io.LimitReader(response.Body, maxDiscoveryErrorBody+1))
		if err != nil {
			return nil, err
		}
		if len(body) > maxDiscoveryErrorBody {
			body = body[:maxDiscoveryErrorBody]
		}
		return nil, fmt.Errorf("服务返回 %s：%s", response.Status, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func enrichDiscoveredModel(existing, incoming ModelInfo) ModelInfo {
	if existing.Name == "" {
		existing.Name = incoming.Name
	}
	if existing.API == "" {
		existing.API = incoming.API
	}
	if existing.Reasoning == nil {
		existing.Reasoning = incoming.Reasoning
	}
	if existing.ContextWindow == 0 {
		existing.ContextWindow = incoming.ContextWindow
	}
	if existing.MaxTokens == 0 {
		existing.MaxTokens = incoming.MaxTokens
	}
	return existing
}
