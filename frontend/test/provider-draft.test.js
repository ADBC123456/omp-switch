import test from "node:test";
import assert from "node:assert/strict";
import { createDraftFromProvider, createDraftFromPreset, normalizeModelInput, readProviderInput, validateModelInput, validateProviderInput } from "../src/domain/provider-draft.js";

test("saved provider draft redacts secret and preserves key capability", () => {
  const draft = createDraftFromProvider({ id: "p", apiKey: "secret", hasApiKey: true, customHeaders: { X: "1" } });
  assert.equal(draft.apiKey, "");
  assert.equal(draft.hasApiKey, true);
  draft.customHeaders.X = "2";
});

test("provider input uses ID as its only identity", () => {
  const form = { querySelector: (selector) => ({ value: selector === '[name="id"]' ? "renamed" : selector === '[name="baseUrl"]' ? "https://example.com/v1" : selector === '[name="api"]' ? "openai-responses" : "secret" }) };
  const input = readProviderInput(form, { headerMode: "none", customHeaders: {} });
  assert.equal(input.id, "renamed");
  assert.equal("name" in input, false);
});

test("preset stays an unsaved creating draft", () => {
  const draft = createDraftFromPreset({ id: "openai", label: "OpenAI", baseUrl: "https://api.openai.com/v1", api: "openai-responses" });
  assert.equal(draft.creating, true);
  assert.equal(validateProviderInput({ ...draft, apiKey: "" }, { creating: true, hasApiKey: false }), "API Key 不能为空");
});

test("model normalization preserves unknown true false reasoning", () => {
  assert.equal(normalizeModelInput({ id: "m", reasoning: "" }).reasoning, null);
  assert.equal(normalizeModelInput({ id: "m", reasoning: "true" }).reasoning, true);
  assert.equal(normalizeModelInput({ id: "m", reasoning: "false" }).reasoning, false);
});

test("model validation rejects duplicate and negative limits", () => {
  assert.match(validateModelInput(normalizeModelInput({ id: "same", contextWindow: "0", maxTokens: "0" }), [{ id: "same" }]), /冲突/);
  assert.match(validateModelInput(normalizeModelInput({ id: "new", contextWindow: "-1", maxTokens: "0" }), []), /上下文/);
  assert.match(validateModelInput(normalizeModelInput({ id: "new", contextWindow: "0", maxTokens: "-1" }), []), /最大输出/);
});
