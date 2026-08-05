package provider

func Presets() []Config {
	reasoningTrue := true
	reasoningFalse := false
	return []Config{
		{
			ID:              "deepseek",
			Name:            "DeepSeek",
			BaseURL:         "https://api.deepseek.com/v1",
			APIKey:          "DEEPSEEK_API_KEY",
			API:             "openai-completions",
			HeaderMode:      "none",
			SelectedModelID: "Deepseek-v4-pro",
			Models: []ModelInfo{
				{ID: "Deepseek-v4-pro", Name: "Deepseek-v4-pro"},
			},
		},
		{
			ID:              "anthropic",
			Name:            "Anthropic",
			BaseURL:         "https://api.anthropic.com",
			APIKey:          "ANTHROPIC_API_KEY",
			API:             "anthropic-messages",
			HeaderMode:      "none",
			SelectedModelID: "claude-opus-4-6",
			Models: []ModelInfo{
				{ID: "claude-opus-4-6", Name: "Claude Opus 4.6", Reasoning: &reasoningTrue, ContextWindow: 1000000, MaxTokens: 128000},
				{ID: "claude-sonnet-4-5", Name: "Claude Sonnet 4.5", Reasoning: &reasoningTrue, ContextWindow: 1000000, MaxTokens: 128000},
				{ID: "claude-haiku-4-5", Name: "Claude Haiku 4.5", Reasoning: &reasoningFalse, ContextWindow: 200000, MaxTokens: 64000},
			},
		},
		{
			ID:              "openai",
			Name:            "OpenAI",
			BaseURL:         "https://api.openai.com/v1",
			APIKey:          "OPENAI_API_KEY",
			API:             "openai-completions",
			HeaderMode:      "none",
			SelectedModelID: "GPT-5.5",
			Models: []ModelInfo{
				{ID: "GPT-5.5", Name: "GPT-5.5"},
			},
		},
	}
}
