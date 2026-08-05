package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"golang.org/x/net/http/httpguts"

	"ompswitch/internal/provider"
)

func ValidateProvider(input provider.Config) error {
	if err := validateProviderID(input.ID); err != nil {
		return err
	}
	parsedURL, err := url.Parse(strings.TrimSpace(input.BaseURL))
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" || parsedURL.Fragment != "" {
		return errors.New("Base URL 必须是无 fragment 的绝对 HTTP(S) URL")
	}
	if !supportedAPI(input.API) {
		return errors.New("API 模式不受支持")
	}
	if err := validateHeaders(input.Headers); err != nil {
		return err
	}
	if err := validateHeaders(input.CustomHeaders); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(input.Models))
	for _, model := range input.Models {
		modelID := strings.TrimSpace(model.ID)
		if modelID == "" {
			return errors.New("模型 ID 不能为空")
		}
		if _, exists := seen[modelID]; exists {
			return fmt.Errorf("模型 ID 重复：%s", modelID)
		}
		seen[modelID] = struct{}{}
		if model.API != "" && !supportedAPI(model.API) {
			return fmt.Errorf("模型 %s 的 API 模式不受支持", modelID)
		}
		if model.ContextWindow < 0 {
			return fmt.Errorf("模型 %s 的 contextWindow 不能为负数", modelID)
		}
		if model.MaxTokens < 0 {
			return fmt.Errorf("模型 %s 的 maxTokens 不能为负数", modelID)
		}
	}
	return nil
}

func validateProviderID(id string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("Provider ID 不能为空")
	}
	for _, character := range id {
		if character == '/' || unicode.IsControl(character) {
			return errors.New("Provider ID 不能包含 / 或控制字符")
		}
	}
	return nil
}

func supportedAPI(api string) bool {
	switch strings.TrimSpace(api) {
	case "anthropic-messages", "openai-completions", "openai-responses", "google-generative-ai":
		return true
	default:
		return false
	}
}

func validateHeaders(headers map[string]string) error {
	for name, value := range headers {
		if !httpguts.ValidHeaderFieldName(name) || !httpguts.ValidHeaderFieldValue(value) {
			return fmt.Errorf("Header 无效：%s", name)
		}
	}
	return nil
}
