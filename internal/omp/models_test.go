package omp

import (
	"reflect"
	"strings"
	"testing"

	"ompswitch/internal/provider"
)

func TestEncodeModelsPreservesOrderAndFields(t *testing.T) {
	falseValue := false
	providers := []provider.Config{
		{
			ID:      "second",
			BaseURL: "https://second.example/v1",
			APIKey:  "SECOND_KEY",
			API:     "openai-responses",
			Headers: map[string]string{"X-Tenant": "b"},
			Models: []provider.ModelInfo{
				{ID: "model-b", Reasoning: &falseValue, ContextWindow: 128000},
				{ID: "model-a", Name: "Model A", API: "openai-completions", MaxTokens: 8192},
			},
		},
		{
			ID:      "first",
			BaseURL: "https://first.example",
			APIKey:  "FIRST_KEY",
			API:     "anthropic-messages",
		},
	}

	encoded, err := EncodeModels(providers)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	if strings.Index(text, "second:") > strings.Index(text, "first:") {
		t.Fatalf("provider order changed:\n%s", text)
	}
	if strings.Index(text, "id: model-b") > strings.Index(text, "id: model-a") {
		t.Fatalf("model order changed:\n%s", text)
	}
	if !strings.Contains(text, "reasoning: false") {
		t.Fatalf("explicit false missing:\n%s", text)
	}
	if !strings.Contains(text, "authHeader: true") {
		t.Fatalf("OpenAI authHeader missing:\n%s", text)
	}

	decoded, err := DecodeModels(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 2 || decoded[0].ID != "second" || decoded[1].ID != "first" {
		t.Fatalf("providers = %#v", decoded)
	}
	if len(decoded[0].Models) != 2 || decoded[0].Models[0].ID != "model-b" || decoded[0].Models[1].ID != "model-a" {
		t.Fatalf("models = %#v", decoded[0].Models)
	}
	if decoded[0].Models[0].Reasoning == nil || *decoded[0].Models[0].Reasoning {
		t.Fatalf("reasoning = %#v, want explicit false", decoded[0].Models[0].Reasoning)
	}
}

func TestEncodeModelsEmpty(t *testing.T) {
	encoded, err := EncodeModels(nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != "providers: {}\n" {
		t.Fatalf("encoded = %q", encoded)
	}
}

func TestDecodeModelsImportsAppFieldsAndIgnoresUnknownModelKeys(t *testing.T) {
	decoded, err := DecodeModels([]byte(`providers:
  gateway:
    baseUrl: https://example.com/v1
    apiKey: GATEWAY_KEY
    api: google-generative-ai
    headers:
      X-Tenant: alpha
    models:
      - id: gemini-pro
        name: Gemini Pro
        reasoning: false
        contextWindow: 1000000
        maxTokens: 8192
        vendorField:
          preservedByOMP: true
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 1 {
		t.Fatalf("providers = %#v", decoded)
	}
	cfg := decoded[0]
	if cfg.ID != "gateway" || cfg.Name != "gateway" || cfg.HeaderMode != "custom" || cfg.SelectedModelID != "gemini-pro" {
		t.Fatalf("imported provider = %#v", cfg)
	}
	wantHeaders := map[string]string{"X-Tenant": "alpha"}
	if !reflect.DeepEqual(cfg.Headers, wantHeaders) || !reflect.DeepEqual(cfg.CustomHeaders, wantHeaders) {
		t.Fatalf("headers = %#v, custom = %#v", cfg.Headers, cfg.CustomHeaders)
	}
	cfg.Headers["X-Tenant"] = "changed"
	if cfg.CustomHeaders["X-Tenant"] != "alpha" {
		t.Fatal("Headers and CustomHeaders share storage")
	}
	if cfg.Models[0].Reasoning == nil || *cfg.Models[0].Reasoning {
		t.Fatalf("reasoning = %#v, want explicit false", cfg.Models[0].Reasoning)
	}
}

func TestDecodeModelsRejectsWrongKnownTypesAndDuplicates(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{"root", "- providers\n"},
		{"providers", "providers: []\n"},
		{"baseUrl", "providers:\n  p:\n    baseUrl: 1\n    apiKey: key\n    api: openai-responses\n"},
		{"headers", "providers:\n  p:\n    baseUrl: https://example.com\n    apiKey: key\n    api: openai-responses\n    headers: []\n"},
		{"models", "providers:\n  p:\n    baseUrl: https://example.com\n    apiKey: key\n    api: openai-responses\n    models: {}\n"},
		{"reasoning", "providers:\n  p:\n    baseUrl: https://example.com\n    apiKey: key\n    api: openai-responses\n    models:\n      - id: m\n        reasoning: no\n"},
		{"contextWindow", "providers:\n  p:\n    baseUrl: https://example.com\n    apiKey: key\n    api: openai-responses\n    models:\n      - id: m\n        contextWindow: large\n"},
		{"duplicate model", "providers:\n  p:\n    baseUrl: https://example.com\n    apiKey: key\n    api: openai-responses\n    models:\n      - id: m\n      - id: m\n"},
		{"duplicate provider", "providers:\n  p: {baseUrl: https://one.example, apiKey: key, api: openai-responses}\n  p: {baseUrl: https://two.example, apiKey: key, api: openai-responses}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeModels([]byte(tt.yaml)); err == nil {
				t.Fatal("DecodeModels succeeded, want error")
			}
		})
	}
}
