package omp

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"ompswitch/internal/provider"
)

func DecodeModels(data []byte) ([]provider.Config, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("解析 models YAML: %w", err)
	}
	root, err := documentMapping(&document)
	if err != nil {
		return nil, err
	}
	providersNode, found, err := uniqueMappingValue(root, "providers")
	if err != nil {
		return nil, err
	}
	if !found || providersNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("providers 必须是 mapping")
	}

	providers := make([]provider.Config, 0, len(providersNode.Content)/2)
	seenProviders := make(map[string]struct{}, len(providersNode.Content)/2)
	for index := 0; index < len(providersNode.Content); index += 2 {
		idNode := providersNode.Content[index]
		valueNode := providersNode.Content[index+1]
		if idNode.Kind != yaml.ScalarNode || idNode.Tag != "!!str" || strings.TrimSpace(idNode.Value) == "" {
			return nil, fmt.Errorf("Provider ID 必须是非空字符串")
		}
		if _, exists := seenProviders[idNode.Value]; exists {
			return nil, fmt.Errorf("Provider ID 重复：%s", idNode.Value)
		}
		seenProviders[idNode.Value] = struct{}{}

		decoded, err := decodeProvider(idNode.Value, valueNode)
		if err != nil {
			return nil, fmt.Errorf("Provider %s: %w", idNode.Value, err)
		}
		providers = append(providers, decoded)
	}
	return providers, nil
}

func decodeProvider(id string, node *yaml.Node) (provider.Config, error) {
	if node.Kind != yaml.MappingNode {
		return provider.Config{}, fmt.Errorf("配置必须是 mapping")
	}
	baseURL, err := requiredString(node, "baseUrl")
	if err != nil {
		return provider.Config{}, err
	}
	apiKey, err := requiredString(node, "apiKey")
	if err != nil {
		return provider.Config{}, err
	}
	api, err := requiredString(node, "api")
	if err != nil {
		return provider.Config{}, err
	}

	headers := make(map[string]string)
	headersNode, found, err := uniqueMappingValue(node, "headers")
	if err != nil {
		return provider.Config{}, err
	}
	if found {
		if err := decodeStringMap(headersNode, headers); err != nil {
			return provider.Config{}, fmt.Errorf("headers: %w", err)
		}
	}

	authHeaderNode, found, err := uniqueMappingValue(node, "authHeader")
	if err != nil {
		return provider.Config{}, err
	}
	if found {
		var ignored bool
		if err := decodeScalar(authHeaderNode, "!!bool", &ignored); err != nil {
			return provider.Config{}, fmt.Errorf("authHeader: %w", err)
		}
	}

	models := make([]provider.ModelInfo, 0)
	modelsNode, found, err := uniqueMappingValue(node, "models")
	if err != nil {
		return provider.Config{}, err
	}
	if found {
		if modelsNode.Kind != yaml.SequenceNode {
			return provider.Config{}, fmt.Errorf("models 必须是 array")
		}
		models = make([]provider.ModelInfo, 0, len(modelsNode.Content))
		seenModels := make(map[string]struct{}, len(modelsNode.Content))
		for _, modelNode := range modelsNode.Content {
			model, err := decodeModel(modelNode)
			if err != nil {
				return provider.Config{}, err
			}
			if _, exists := seenModels[model.ID]; exists {
				return provider.Config{}, fmt.Errorf("模型 ID 重复：%s", model.ID)
			}
			seenModels[model.ID] = struct{}{}
			models = append(models, model)
		}
	}

	selectedModelID := ""
	if len(models) > 0 {
		selectedModelID = models[0].ID
	}
	return provider.Config{
		ID:              id,
		Name:            id,
		BaseURL:         baseURL,
		APIKey:          apiKey,
		API:             api,
		HeaderMode:      "custom",
		Headers:         cloneStrings(headers),
		CustomHeaders:   cloneStrings(headers),
		Models:          models,
		SelectedModelID: selectedModelID,
	}, nil
}

func decodeModel(node *yaml.Node) (provider.ModelInfo, error) {
	if node.Kind != yaml.MappingNode {
		return provider.ModelInfo{}, fmt.Errorf("模型必须是 mapping")
	}
	id, err := requiredString(node, "id")
	if err != nil || strings.TrimSpace(id) == "" {
		return provider.ModelInfo{}, fmt.Errorf("模型 id 必须是非空字符串")
	}

	model := provider.ModelInfo{ID: id}
	if value, found, err := uniqueMappingValue(node, "name"); err != nil {
		return provider.ModelInfo{}, fmt.Errorf("模型 %s name: %w", id, err)
	} else if found {
		if err := decodeScalar(value, "!!str", &model.Name); err != nil {
			return provider.ModelInfo{}, fmt.Errorf("模型 %s name: %w", id, err)
		}
	}
	if value, found, err := uniqueMappingValue(node, "api"); err != nil {
		return provider.ModelInfo{}, fmt.Errorf("模型 %s api: %w", id, err)
	} else if found {
		if err := decodeScalar(value, "!!str", &model.API); err != nil {
			return provider.ModelInfo{}, fmt.Errorf("模型 %s api: %w", id, err)
		}
	}
	if value, found, err := uniqueMappingValue(node, "reasoning"); err != nil {
		return provider.ModelInfo{}, fmt.Errorf("模型 %s reasoning: %w", id, err)
	} else if found {
		var decoded bool
		if err := decodeScalar(value, "!!bool", &decoded); err != nil {
			return provider.ModelInfo{}, fmt.Errorf("模型 %s reasoning: %w", id, err)
		}
		model.Reasoning = &decoded
	}
	if value, found, err := uniqueMappingValue(node, "contextWindow"); err != nil {
		return provider.ModelInfo{}, fmt.Errorf("模型 %s contextWindow: %w", id, err)
	} else if found {
		if err := decodeNonNegativeInt(value, &model.ContextWindow); err != nil {
			return provider.ModelInfo{}, fmt.Errorf("模型 %s contextWindow: %w", id, err)
		}
	}
	if value, found, err := uniqueMappingValue(node, "maxTokens"); err != nil {
		return provider.ModelInfo{}, fmt.Errorf("模型 %s maxTokens: %w", id, err)
	} else if found {
		if err := decodeNonNegativeInt(value, &model.MaxTokens); err != nil {
			return provider.ModelInfo{}, fmt.Errorf("模型 %s maxTokens: %w", id, err)
		}
	}
	return model, nil
}

func EncodeModels(providers []provider.Config) ([]byte, error) {
	root := mappingNode()
	providersNode := mappingNode()
	appendMapping(root, scalarNode("providers"), providersNode)

	seenProviders := make(map[string]struct{}, len(providers))
	for _, cfg := range providers {
		if strings.TrimSpace(cfg.ID) == "" {
			return nil, fmt.Errorf("Provider ID 不能为空")
		}
		if _, exists := seenProviders[cfg.ID]; exists {
			return nil, fmt.Errorf("Provider ID 重复：%s", cfg.ID)
		}
		seenProviders[cfg.ID] = struct{}{}

		node, err := encodeProvider(cfg)
		if err != nil {
			return nil, fmt.Errorf("Provider %s: %w", cfg.ID, err)
		}
		appendMapping(providersNode, scalarNode(cfg.ID), node)
	}

	document := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}
	return yaml.Marshal(document)
}

func encodeProvider(cfg provider.Config) (*yaml.Node, error) {
	if cfg.BaseURL == "" || cfg.APIKey == "" || cfg.API == "" {
		return nil, fmt.Errorf("baseUrl、apiKey 和 api 必填")
	}

	node := mappingNode()
	appendString(node, "baseUrl", cfg.BaseURL)
	appendString(node, "apiKey", cfg.APIKey)
	appendString(node, "api", cfg.API)
	if cfg.API == "openai-completions" || cfg.API == "openai-responses" {
		appendBool(node, "authHeader", true)
	}
	if len(cfg.Headers) > 0 {
		headersNode := mappingNode()
		keys := make([]string, 0, len(cfg.Headers))
		for key := range cfg.Headers {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			appendString(headersNode, key, cfg.Headers[key])
		}
		appendMapping(node, scalarNode("headers"), headersNode)
	}

	modelsNode := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	seenModels := make(map[string]struct{}, len(cfg.Models))
	for _, model := range cfg.Models {
		if strings.TrimSpace(model.ID) == "" {
			return nil, fmt.Errorf("模型 ID 不能为空")
		}
		if _, exists := seenModels[model.ID]; exists {
			return nil, fmt.Errorf("模型 ID 重复：%s", model.ID)
		}
		if model.ContextWindow < 0 {
			return nil, fmt.Errorf("模型 %s contextWindow 不能为负数", model.ID)
		}
		if model.MaxTokens < 0 {
			return nil, fmt.Errorf("模型 %s maxTokens 不能为负数", model.ID)
		}
		seenModels[model.ID] = struct{}{}

		modelNode := mappingNode()
		appendString(modelNode, "id", model.ID)
		if model.Name != "" {
			appendString(modelNode, "name", model.Name)
		}
		if model.API != "" {
			appendString(modelNode, "api", model.API)
		}
		if model.Reasoning != nil {
			appendBool(modelNode, "reasoning", *model.Reasoning)
		}
		if model.ContextWindow != 0 {
			appendInt(modelNode, "contextWindow", model.ContextWindow)
		}
		if model.MaxTokens != 0 {
			appendInt(modelNode, "maxTokens", model.MaxTokens)
		}
		modelsNode.Content = append(modelsNode.Content, modelNode)
	}
	appendMapping(node, scalarNode("models"), modelsNode)
	return node, nil
}

func documentMapping(document *yaml.Node) (*yaml.Node, error) {
	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("YAML root 必须是 mapping")
	}
	return document.Content[0], nil
}

func uniqueMappingValue(node *yaml.Node, key string) (*yaml.Node, bool, error) {
	var result *yaml.Node
	for index := 0; index < len(node.Content); index += 2 {
		keyNode := node.Content[index]
		if keyNode.Kind != yaml.ScalarNode || keyNode.Tag != "!!str" || keyNode.Value != key {
			continue
		}
		if result != nil {
			return nil, false, fmt.Errorf("字段重复：%s", key)
		}
		result = node.Content[index+1]
	}
	return result, result != nil, nil
}

func requiredString(node *yaml.Node, key string) (string, error) {
	value, found, err := uniqueMappingValue(node, key)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("%s 必填", key)
	}
	var decoded string
	if err := decodeScalar(value, "!!str", &decoded); err != nil {
		return "", fmt.Errorf("%s: %w", key, err)
	}
	return decoded, nil
}

func decodeScalar(node *yaml.Node, tag string, target any) error {
	if node.Kind != yaml.ScalarNode || node.Tag != tag {
		return fmt.Errorf("类型错误")
	}
	if err := node.Decode(target); err != nil {
		return fmt.Errorf("值无效: %w", err)
	}
	return nil
}

func decodeNonNegativeInt(node *yaml.Node, target *int) error {
	if err := decodeScalar(node, "!!int", target); err != nil {
		return err
	}
	if *target < 0 {
		return fmt.Errorf("不能为负数")
	}
	return nil
}

func decodeStringMap(node *yaml.Node, target map[string]string) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("必须是 mapping")
	}
	for index := 0; index < len(node.Content); index += 2 {
		keyNode := node.Content[index]
		valueNode := node.Content[index+1]
		if keyNode.Kind != yaml.ScalarNode || keyNode.Tag != "!!str" || valueNode.Kind != yaml.ScalarNode || valueNode.Tag != "!!str" {
			return fmt.Errorf("键和值必须是字符串")
		}
		if _, exists := target[keyNode.Value]; exists {
			return fmt.Errorf("键重复：%s", keyNode.Value)
		}
		target[keyNode.Value] = valueNode.Value
	}
	return nil
}

func mappingNode() *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
}

func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func appendMapping(mapping, key, value *yaml.Node) {
	mapping.Content = append(mapping.Content, key, value)
}

func appendString(mapping *yaml.Node, key, value string) {
	appendMapping(mapping, scalarNode(key), scalarNode(value))
}

func appendBool(mapping *yaml.Node, key string, value bool) {
	appendMapping(mapping, scalarNode(key), &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!bool",
		Value: strconv.FormatBool(value),
	})
}

func appendInt(mapping *yaml.Node, key string, value int) {
	appendMapping(mapping, scalarNode(key), &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!int",
		Value: strconv.Itoa(value),
	})
}

func cloneStrings(input map[string]string) map[string]string {
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}
