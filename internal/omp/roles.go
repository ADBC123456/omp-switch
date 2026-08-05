package omp

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"ompswitch/internal/provider"
)

var ManagedRoles = []string{
	"default",
	"smol",
	"slow",
	"plan",
	"commit",
	"vision",
	"designer",
	"task",
	"advisor",
	"tiny",
}

var ThinkingLevels = []string{"off", "minimal", "low", "medium", "high", "xhigh", "max", "auto"}

// ParseManagedSelector resolves only selectors that exactly name a configured
// provider/model. A thinking suffix is considered only after trying the full raw
// value, so model IDs containing slashes or ending in suffix-looking text remain
// unambiguous.
func ParseManagedSelector(raw string, providers []provider.Config) (providerID, modelID, thinking string, ok bool) {
	if providerID, modelID, ok = matchSelector(raw, providers); ok {
		return providerID, modelID, "", true
	}
	separator := strings.LastIndexByte(raw, ':')
	if separator < 0 || !isThinkingLevel(raw[separator+1:]) {
		return "", "", "", false
	}
	providerID, modelID, ok = matchSelector(raw[:separator], providers)
	if !ok {
		return "", "", "", false
	}
	return providerID, modelID, raw[separator+1:], true
}

func RewriteManagedSelectors(roles map[string]string, providers []provider.Config, oldProvider, oldModel, newProvider, newModel string) []string {
	changed := make([]string, 0)
	for _, role := range ManagedRoles {
		raw, exists := roles[role]
		if !exists {
			continue
		}
		providerID, modelID, thinking, ok := ParseManagedSelector(raw, providers)
		if !ok || providerID != oldProvider || (oldModel != "" && modelID != oldModel) {
			continue
		}
		if newProvider == "" {
			delete(roles, role)
			changed = append(changed, role)
			continue
		}
		modelReplacement := newModel
		if modelReplacement == "" {
			modelReplacement = modelID
		}
		selector := newProvider + "/" + modelReplacement
		if thinking != "" {
			selector += ":" + thinking
		}
		roles[role] = selector
		changed = append(changed, role)
	}
	return changed
}

func ManagedRoleImpact(roles map[string]string, providers []provider.Config, providerID, modelID string) []string {
	impacted := make([]string, 0)
	for _, role := range ManagedRoles {
		matchedProvider, matchedModel, _, ok := ParseManagedSelector(roles[role], providers)
		if ok && matchedProvider == providerID && (modelID == "" || matchedModel == modelID) {
			impacted = append(impacted, role)
		}
	}
	return impacted
}

func IsManagedRole(role string) bool {
	_, ok := managedRoleSet()[role]
	return ok
}

func matchSelector(raw string, providers []provider.Config) (string, string, bool) {
	for _, item := range providers {
		prefix := item.ID + "/"
		if !strings.HasPrefix(raw, prefix) {
			continue
		}
		modelID := raw[len(prefix):]
		for _, model := range item.Models {
			if model.ID == modelID {
				return item.ID, model.ID, true
			}
		}
	}
	return "", "", false
}

func isThinkingLevel(value string) bool {
	for _, level := range ThinkingLevels {
		if value == level {
			return true
		}
	}
	return false
}

func DecodeManagedRoles(data []byte) (map[string]string, error) {
	roles := make(map[string]string)
	if len(bytes.TrimSpace(data)) == 0 {
		return roles, nil
	}

	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("解析 config YAML: %w", err)
	}
	if len(document.Content) == 0 {
		return roles, nil
	}
	root, err := documentMapping(&document)
	if err != nil {
		return nil, err
	}
	modelRoles, found, err := uniqueMappingValue(root, "modelRoles")
	if err != nil {
		return nil, err
	}
	if !found {
		return roles, nil
	}
	if modelRoles.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("modelRoles 必须是 mapping")
	}

	managed := managedRoleSet()
	for index := 0; index < len(modelRoles.Content); index += 2 {
		keyNode := modelRoles.Content[index]
		valueNode := modelRoles.Content[index+1]
		if keyNode.Kind != yaml.ScalarNode || keyNode.Tag != "!!str" {
			continue
		}
		if _, isManaged := managed[keyNode.Value]; !isManaged {
			continue
		}
		if _, duplicate := roles[keyNode.Value]; duplicate {
			return nil, fmt.Errorf("modelRoles 字段重复：%s", keyNode.Value)
		}
		if valueNode.Kind != yaml.ScalarNode || valueNode.Tag != "!!str" {
			return nil, fmt.Errorf("modelRoles.%s 必须是字符串", keyNode.Value)
		}
		roles[keyNode.Value] = valueNode.Value
	}
	return roles, nil
}

func MergeManagedRoles(data []byte, roles map[string]string) ([]byte, error) {
	document, root, err := decodeConfigDocument(data)
	if err != nil {
		return nil, err
	}
	modelRoles, found, err := uniqueMappingValue(root, "modelRoles")
	if err != nil {
		return nil, err
	}
	if !found {
		modelRoles = mappingNode()
		appendMapping(root, scalarNode("modelRoles"), modelRoles)
	} else if modelRoles.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("modelRoles 必须是 mapping")
	}

	managed := managedRoleSet()
	seenManaged := make(map[string]struct{}, len(ManagedRoles))
	content := make([]*yaml.Node, 0, len(modelRoles.Content))
	for index := 0; index < len(modelRoles.Content); index += 2 {
		keyNode := modelRoles.Content[index]
		valueNode := modelRoles.Content[index+1]
		if keyNode.Kind != yaml.ScalarNode || keyNode.Tag != "!!str" {
			content = append(content, keyNode, valueNode)
			continue
		}
		role := keyNode.Value
		if _, isManaged := managed[role]; !isManaged {
			content = append(content, keyNode, valueNode)
			continue
		}
		if _, duplicate := seenManaged[role]; duplicate {
			return nil, fmt.Errorf("modelRoles 字段重复：%s", role)
		}
		seenManaged[role] = struct{}{}
		selector := roles[role]
		if selector == "" {
			continue
		}
		valueNode.Kind = yaml.ScalarNode
		valueNode.Tag = "!!str"
		valueNode.Value = selector
		valueNode.Content = nil
		valueNode.Alias = nil
		content = append(content, keyNode, valueNode)
	}
	modelRoles.Content = content

	for _, role := range ManagedRoles {
		if _, exists := seenManaged[role]; exists {
			continue
		}
		if selector := roles[role]; selector != "" {
			appendString(modelRoles, role, selector)
		}
	}
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(document); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func decodeConfigDocument(data []byte) (*yaml.Node, *yaml.Node, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		root := mappingNode()
		document := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}
		return document, root, nil
	}

	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, nil, fmt.Errorf("解析 config YAML: %w", err)
	}
	if len(document.Content) == 0 {
		root := mappingNode()
		document = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}
		return &document, root, nil
	}
	root, err := documentMapping(&document)
	if err != nil {
		return nil, nil, err
	}
	return &document, root, nil
}

func managedRoleSet() map[string]struct{} {
	roles := make(map[string]struct{}, len(ManagedRoles))
	for _, role := range ManagedRoles {
		roles[role] = struct{}{}
	}
	return roles
}
