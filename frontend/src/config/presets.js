const PRESET_LIST = [
  { id: "anthropic", label: "Anthropic", name: "Anthropic", baseUrl: "https://api.anthropic.com", apiKey: "ANTHROPIC_API_KEY", api: "anthropic-messages", headerMode: "none", headers: {}, customHeaders: {} },
  { id: "deepseek", label: "DeepSeek", name: "DeepSeek", baseUrl: "https://api.deepseek.com/v1", apiKey: "DEEPSEEK_API_KEY", api: "openai-completions", headerMode: "none", headers: {}, customHeaders: {} },
  { id: "openai", label: "OpenAI", name: "OpenAI", baseUrl: "https://api.openai.com/v1", apiKey: "OPENAI_API_KEY", api: "openai-completions", headerMode: "none", headers: {}, customHeaders: {} },
  { id: "custom", label: "自定义", name: "自定义 Provider", baseUrl: "https://", apiKey: "", api: "openai-completions", headerMode: "none", headers: {}, customHeaders: {} }
];

export const PRESETS = PRESET_LIST;

export function providerPreset(id) {
  return PRESET_LIST.find((preset) => preset.id === id) ?? PRESET_LIST.at(-1);
}
