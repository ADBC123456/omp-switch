package config

import (
	"strings"
	"testing"

	"ompswitch/internal/provider"
)

func validProvider() provider.Config {
	return provider.Config{
		ID: "provider", Name: "Provider", BaseURL: "https://example.com/v1?tenant=a",
		APIKey: "secret", API: "openai-completions", HeaderMode: "custom",
		Headers: map[string]string{"X-Tenant": "a"},
		Models:  []provider.ModelInfo{{ID: "model"}},
	}
}

func TestValidateProviderAcceptsValidInput(t *testing.T) {
	if err := ValidateProvider(validProvider()); err != nil {
		t.Fatal(err)
	}
}

func TestValidateProviderRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name string
		edit func(*provider.Config)
		want string
	}{
		{"slash id", func(c *provider.Config) { c.ID = "bad/id" }, "Provider ID"},
		{"control id", func(c *provider.Config) { c.ID = "bad\n" }, "Provider ID"},
		{"relative url", func(c *provider.Config) { c.BaseURL = "/v1" }, "Base URL"},
		{"unsupported scheme", func(c *provider.Config) { c.BaseURL = "ftp://example.com" }, "Base URL"},
		{"fragment", func(c *provider.Config) { c.BaseURL = "https://example.com/#x" }, "Base URL"},
		{"unsupported api", func(c *provider.Config) { c.API = "anthropic" }, "API"},
		{"bad header", func(c *provider.Config) { c.Headers = map[string]string{"Bad Header": "value"} }, "Header"},
		{"duplicate model", func(c *provider.Config) { c.Models = append(c.Models, provider.ModelInfo{ID: "model"}) }, "模型 ID"},
		{"negative context", func(c *provider.Config) { c.Models[0].ContextWindow = -1 }, "contextWindow"},
		{"unsupported model api", func(c *provider.Config) { c.Models[0].API = "anthropic" }, "API"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validProvider()
			tt.edit(&cfg)
			err := ValidateProvider(cfg)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}
