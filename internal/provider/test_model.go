package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxModelTestErrorBody = 4 << 10

type ModelTestOptions struct {
	Client *http.Client
}

func TestModel(ctx context.Context, cfg Config, model ModelInfo, resolvedKey string, options ModelTestOptions) (ConnectionTestResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	api := strings.TrimSpace(model.API)
	if api == "" {
		api = strings.TrimSpace(cfg.API)
	}
	endpoint, payload, err := modelTestRequest(cfg.BaseURL, api, model.ID, resolvedKey)
	if err != nil {
		return ConnectionTestResult{}, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ConnectionTestResult{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ConnectionTestResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	switch api {
	case "anthropic-messages", "anthropic":
		if strings.TrimSpace(resolvedKey) != "" {
			request.Header.Set("x-api-key", strings.TrimSpace(resolvedKey))
		}
		request.Header.Set("anthropic-version", anthropicVersion)
	case "google-generative-ai":
	default:
		if strings.TrimSpace(resolvedKey) != "" {
			request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(resolvedKey))
		}
	}
	for name, value := range cfg.Headers {
		if strings.HasPrefix(strings.TrimSpace(value), "!") {
			return ConnectionTestResult{}, fmt.Errorf("Header %s 是命令形式，Switch 不会执行命令", name)
		}
		request.Header.Set(name, value)
	}
	client := options.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	started := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return ConnectionTestResult{}, fmt.Errorf("请求上游模型: %w", err)
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, maxModelTestErrorBody+1))
	if readErr != nil {
		return ConnectionTestResult{}, fmt.Errorf("读取上游响应: %w", readErr)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		if len(responseBody) > maxModelTestErrorBody {
			responseBody = responseBody[:maxModelTestErrorBody]
		}
		message := strings.TrimSpace(string(responseBody))
		if message == "" {
			message = "无响应正文"
		}
		return ConnectionTestResult{}, fmt.Errorf("上游返回 %s：%s", response.Status, message)
	}
	return ConnectionTestResult{
		OK: true, Title: "模型测试成功",
		Lines: []string{fmt.Sprintf("%s / %s", cfg.ID, model.ID), fmt.Sprintf("上游响应 %s · %d ms", response.Status, time.Since(started).Milliseconds())},
	}, nil
}

func modelTestRequest(baseURL, api, modelID, resolvedKey string) (string, map[string]any, error) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return "", nil, errors.New("模型 ID 不能为空")
	}
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", nil, errors.New("Base URL 必须是绝对 URL")
	}
	basePath := strings.TrimRight(parsed.Path, "/")
	withV1 := func(suffix string) string {
		if strings.HasSuffix(basePath, "/v1") || strings.HasSuffix(basePath, "/v1beta") {
			return basePath + suffix
		}
		return basePath + "/v1" + suffix
	}
	var payload map[string]any
	switch api {
	case "anthropic-messages", "anthropic":
		parsed.Path = withV1("/messages")
		payload = map[string]any{"model": modelID, "max_tokens": 16, "messages": []map[string]string{{"role": "user", "content": "Hi"}}}
	case "openai-responses":
		parsed.Path = withV1("/responses")
		payload = map[string]any{"model": modelID, "max_output_tokens": 16, "input": "Hi", "stream": false}
	case "google-generative-ai":
		if !strings.HasSuffix(basePath, "/v1beta") && !strings.HasSuffix(basePath, "/v1") {
			basePath += "/v1beta"
		}
		parsed.Path = basePath + "/models/" + url.PathEscape(modelID) + ":generateContent"
		query := parsed.Query()
		if strings.TrimSpace(resolvedKey) != "" {
			query.Set("key", strings.TrimSpace(resolvedKey))
		}
		parsed.RawQuery = query.Encode()
		payload = map[string]any{"contents": []map[string]any{{"parts": []map[string]string{{"text": "Hi"}}}}, "generationConfig": map[string]int{"maxOutputTokens": 16}}
	default:
		parsed.Path = withV1("/chat/completions")
		payload = map[string]any{"model": modelID, "max_tokens": 16, "messages": []map[string]string{{"role": "user", "content": "Hi"}}, "stream": false}
	}
	parsed.Fragment = ""
	return parsed.String(), payload, nil
}
