package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type discoveryPage struct {
	models          []ModelInfo
	cursor          string
	cursorParameter string
}

type discoveredItem struct {
	model           ModelInfo
	classifications []string
	geminiMethods   []string
	methodsPresent  bool
}

func parseDiscoveryPage(data []byte) (discoveryPage, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return discoveryPage{}, err
	}
	if root == nil {
		return discoveryPage{}, errors.New("模型响应根必须是对象")
	}
	dataRaw, hasData := root["data"]
	modelsRaw, hasModels := root["models"]
	if hasData == hasModels {
		return discoveryPage{}, errors.New("模型响应必须且只能包含 data 或 models 数组")
	}
	if hasData {
		return parseDataEnvelope(root, dataRaw)
	}
	return parseGeminiEnvelope(root, modelsRaw)
}

func parseDataEnvelope(root map[string]json.RawMessage, arrayRaw json.RawMessage) (discoveryPage, error) {
	items, err := decodeObjectArray(arrayRaw, "data")
	if err != nil {
		return discoveryPage{}, err
	}
	parsed := make([]discoveredItem, 0, len(items))
	validIDs := 0
	for index, item := range items {
		discovered, valid, err := parseDataItem(item)
		if err != nil {
			return discoveryPage{}, fmt.Errorf("data[%d]: %w", index, err)
		}
		if !valid {
			continue
		}
		validIDs++
		parsed = append(parsed, discovered)
	}
	if validIDs == 0 {
		return discoveryPage{}, errors.New("模型响应中没有有效 ID")
	}

	hasMore, present, err := optionalBool(root, "has_more")
	if err != nil {
		return discoveryPage{}, err
	}
	lastID, lastIDPresent, err := optionalString(root, "last_id")
	if err != nil {
		return discoveryPage{}, err
	}
	if present && hasMore {
		if !lastIDPresent || strings.TrimSpace(lastID) == "" {
			return discoveryPage{}, errors.New("has_more 为 true 时 last_id 必须为非空字符串")
		}
		return discoveryPage{models: filteredModels(parsed), cursor: strings.TrimSpace(lastID), cursorParameter: "after"}, nil
	}
	return discoveryPage{models: filteredModels(parsed)}, nil
}

func parseDataItem(item map[string]json.RawMessage) (discoveredItem, bool, error) {
	id, present, err := requiredString(item, "id")
	if err != nil {
		return discoveredItem{}, false, err
	}
	if !present {
		return discoveredItem{}, false, errors.New("id 必须存在且为字符串")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return discoveredItem{}, false, nil
	}
	name, err := firstOptionalString(item, "name", "display_name")
	if err != nil {
		return discoveredItem{}, false, err
	}
	reasoning, err := optionalBoolPointer(item, "reasoning")
	if err != nil {
		return discoveredItem{}, false, err
	}
	contextWindow, err := firstOptionalPositiveInteger(item, "context_window", "context_length", "input_token_limit")
	if err != nil {
		return discoveredItem{}, false, err
	}
	maxTokens, err := firstOptionalPositiveInteger(item, "max_output_tokens", "max_tokens", "output_token_limit")
	if err != nil {
		return discoveredItem{}, false, err
	}
	classifications := make([]string, 0)
	for _, key := range []string{"type", "model_type", "task"} {
		value, present, err := optionalString(item, key)
		if err != nil {
			return discoveredItem{}, false, err
		}
		if present {
			classifications = append(classifications, value)
		}
	}
	capabilities, _, err := optionalStringArray(item, "capabilities")
	if err != nil {
		return discoveredItem{}, false, err
	}
	classifications = append(classifications, capabilities...)
	return discoveredItem{
		model:           ModelInfo{ID: id, Name: strings.TrimSpace(name), Reasoning: reasoning, ContextWindow: contextWindow, MaxTokens: maxTokens},
		classifications: classifications,
	}, true, nil
}

func parseGeminiEnvelope(root map[string]json.RawMessage, arrayRaw json.RawMessage) (discoveryPage, error) {
	items, err := decodeObjectArray(arrayRaw, "models")
	if err != nil {
		return discoveryPage{}, err
	}
	parsed := make([]discoveredItem, 0, len(items))
	validIDs := 0
	for index, item := range items {
		discovered, valid, err := parseGeminiItem(item)
		if err != nil {
			return discoveryPage{}, fmt.Errorf("models[%d]: %w", index, err)
		}
		if !valid {
			continue
		}
		validIDs++
		parsed = append(parsed, discovered)
	}
	if validIDs == 0 {
		return discoveryPage{}, errors.New("模型响应中没有有效 ID")
	}
	token, present, err := optionalString(root, "nextPageToken")
	if err != nil {
		return discoveryPage{}, err
	}
	page := discoveryPage{models: filteredModels(parsed)}
	if present && strings.TrimSpace(token) != "" {
		page.cursor = strings.TrimSpace(token)
		page.cursorParameter = "pageToken"
	}
	return page, nil
}

func parseGeminiItem(item map[string]json.RawMessage) (discoveredItem, bool, error) {
	nameValue, present, err := requiredString(item, "name")
	if err != nil {
		return discoveredItem{}, false, err
	}
	if !present {
		return discoveredItem{}, false, errors.New("name 必须存在且为字符串")
	}
	id := strings.TrimSpace(nameValue)
	id = strings.TrimPrefix(id, "models/")
	if id == "" {
		return discoveredItem{}, false, nil
	}
	displayName, _, err := optionalString(item, "displayName")
	if err != nil {
		return discoveredItem{}, false, err
	}
	contextWindow, err := firstOptionalPositiveInteger(item, "inputTokenLimit")
	if err != nil {
		return discoveredItem{}, false, err
	}
	maxTokens, err := firstOptionalPositiveInteger(item, "outputTokenLimit")
	if err != nil {
		return discoveredItem{}, false, err
	}
	methods, methodsPresent, err := optionalStringArray(item, "supportedGenerationMethods")
	if err != nil {
		return discoveredItem{}, false, err
	}
	return discoveredItem{
		model:         ModelInfo{ID: id, Name: strings.TrimSpace(displayName), ContextWindow: contextWindow, MaxTokens: maxTokens},
		geminiMethods: methods, methodsPresent: methodsPresent,
	}, true, nil
}

func filteredModels(items []discoveredItem) []ModelInfo {
	models := make([]ModelInfo, 0, len(items))
	for _, item := range items {
		if item.methodsPresent {
			generateContent := false
			for _, method := range item.geminiMethods {
				if normalizeClassification(method) == "generatecontent" {
					generateContent = true
					break
				}
			}
			if !generateContent {
				continue
			}
		}
		if shouldFilterClassification(item.classifications) {
			continue
		}
		models = append(models, item.model)
	}
	return models
}

func shouldFilterClassification(values []string) bool {
	chat := map[string]struct{}{"chat": {}, "text-generation": {}, "messages": {}, "responses": {}, "generatecontent": {}}
	nonChat := map[string]struct{}{"embedding": {}, "embeddings": {}, "image": {}, "audio": {}, "speech": {}, "rerank": {}, "moderation": {}}
	hasChat, hasNonChat := false, false
	for _, value := range values {
		normalized := normalizeClassification(value)
		if _, found := chat[normalized]; found {
			hasChat = true
		}
		if _, found := nonChat[normalized]; found {
			hasNonChat = true
		}
	}
	return hasNonChat && !hasChat
}

func normalizeClassification(value string) string {
	return strings.ToLower(strings.NewReplacer(" ", "-", "_", "-").Replace(strings.TrimSpace(value)))
}

func decodeObjectArray(raw json.RawMessage, field string) ([]map[string]json.RawMessage, error) {
	var array []json.RawMessage
	if err := json.Unmarshal(raw, &array); err != nil || array == nil {
		return nil, fmt.Errorf("%s 必须是数组", field)
	}
	objects := make([]map[string]json.RawMessage, len(array))
	for index, item := range array {
		if err := json.Unmarshal(item, &objects[index]); err != nil || objects[index] == nil {
			return nil, fmt.Errorf("%s[%d] 必须是对象", field, index)
		}
	}
	return objects, nil
}

func requiredString(object map[string]json.RawMessage, key string) (string, bool, error) {
	return optionalString(object, key)
}

func optionalString(object map[string]json.RawMessage, key string) (string, bool, error) {
	raw, present := object[key]
	if !present {
		return "", false, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", true, fmt.Errorf("%s 必须是字符串", key)
	}
	return value, true, nil
}

func firstOptionalString(object map[string]json.RawMessage, keys ...string) (string, error) {
	value := ""
	set := false
	for _, key := range keys {
		candidate, present, err := optionalString(object, key)
		if err != nil {
			return "", err
		}
		if present && !set {
			value, set = candidate, true
		}
	}
	return value, nil
}

func optionalBool(object map[string]json.RawMessage, key string) (bool, bool, error) {
	raw, present := object[key]
	if !present {
		return false, false, nil
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, true, fmt.Errorf("%s 必须是布尔值", key)
	}
	return value, true, nil
}

func optionalBoolPointer(object map[string]json.RawMessage, key string) (*bool, error) {
	value, present, err := optionalBool(object, key)
	if err != nil || !present {
		return nil, err
	}
	return &value, nil
}

func firstOptionalPositiveInteger(object map[string]json.RawMessage, keys ...string) (int, error) {
	value := 0
	set := false
	for _, key := range keys {
		raw, present := object[key]
		if !present {
			continue
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		var decoded any
		if err := decoder.Decode(&decoded); err != nil {
			return 0, fmt.Errorf("%s 必须是正整数", key)
		}
		number, ok := decoded.(json.Number)
		if !ok {
			return 0, fmt.Errorf("%s 必须是正整数", key)
		}
		integer, err := strconv.ParseInt(number.String(), 10, 0)
		if err != nil || integer <= 0 {
			return 0, fmt.Errorf("%s 必须是正整数", key)
		}
		if !set {
			value, set = int(integer), true
		}
	}
	return value, nil
}

func optionalStringArray(object map[string]json.RawMessage, key string) ([]string, bool, error) {
	raw, present := object[key]
	if !present {
		return nil, false, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return nil, true, fmt.Errorf("%s 必须是字符串数组", key)
	}
	return values, true, nil
}
