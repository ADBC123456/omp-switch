import { headersForMode } from "../config/header-presets.js";

const APIS = new Set(["anthropic-messages", "openai-completions", "openai-responses", "google-generative-ai"]);

function copyHeaders(input = {}) {
  return Object.fromEntries(Object.entries(input).map(([name, value]) => [name, String(value)]));
}

export function createDraftFromProvider(provider) {
  return {
    id: provider.id ?? "",
    baseUrl: provider.baseUrl ?? "",
    apiKey: "",
    hasApiKey: !!provider.hasApiKey,
    api: provider.api ?? "openai-completions",
    headerMode: provider.headerMode ?? "none",
    headers: copyHeaders(provider.headers),
    customHeaders: copyHeaders(provider.customHeaders),
    creating: false
  };
}

export function createDraftFromPreset(preset) {
  return {
    id: preset.id ?? "",
    baseUrl: preset.baseUrl ?? "",
    apiKey: preset.apiKey ?? "",
    hasApiKey: false,
    api: preset.api ?? "openai-completions",
    headerMode: preset.headerMode ?? "none",
    headers: copyHeaders(preset.headers),
    customHeaders: copyHeaders(preset.customHeaders),
    creating: true
  };
}

export function readProviderInput(form, draft) {
  const value = (name) => form.querySelector(`[name="${name}"]`)?.value?.trim?.() ?? "";
  const mode = draft.headerMode || "none";
  return {
    id: value("id"),
    baseUrl: value("baseUrl"),
    apiKey: value("apiKey"),
    api: value("api"),
    headerMode: mode,
    headers: headersForMode(mode, value("api"), draft.customHeaders),
    customHeaders: copyHeaders(draft.customHeaders)
  };
}

export function validateProviderInput(input, { creating, hasApiKey }) {
  if (!input.id) return "Provider ID 不能为空";
  if (input.id.includes("/") || /[\u0000-\u001f\u007f]/.test(input.id)) return "Provider ID 不能包含 / 或控制字符";
  try {
    const parsed = new URL(input.baseUrl);
    if (!/^https?:$/.test(parsed.protocol) || !parsed.host || parsed.hash) throw new Error();
  } catch {
    return "Base URL 必须是无 fragment 的绝对 HTTP(S) URL";
  }
  if (!APIS.has(input.api)) return "API 模式不受支持";
  if (creating && !input.apiKey) return "API Key 不能为空";
  if (!creating && !hasApiKey && !input.apiKey) return "API Key 不能为空";
  return "";
}

export function normalizeModelInput(raw) {
  const reasoning = raw.reasoning === "true" ? true : raw.reasoning === "false" ? false : null;
  return {
    id: String(raw.id ?? "").trim(),
    name: String(raw.name ?? "").trim(),
    api: String(raw.api ?? "").trim(),
    reasoning,
    contextWindow: raw.contextWindow === "" ? 0 : Number(raw.contextWindow),
    maxTokens: raw.maxTokens === "" ? 0 : Number(raw.maxTokens)
  };
}

export function validateModelInput(input, existingModels, originalId = "") {
  if (!input.id) return "模型 ID 不能为空";
  if (/[\u0000-\u001f\u007f]/.test(input.id)) return "模型 ID 不能包含控制字符";
  if (existingModels.some((model) => model.id === input.id && model.id !== originalId)) return `模型 ID 冲突：${input.id}`;
  if (input.api && !APIS.has(input.api)) return "模型 API 模式不受支持";
  if (!Number.isInteger(input.contextWindow) || input.contextWindow < 0) return "Context Window 必须是正整数";
  if (!Number.isInteger(input.maxTokens) || input.maxTokens < 0) return "Max Tokens 必须是正整数";
  return "";
}
