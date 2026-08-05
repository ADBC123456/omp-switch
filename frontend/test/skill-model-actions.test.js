import test from "node:test";
import assert from "node:assert/strict";
import { createAppActions } from "../src/actions/app-actions.js";
import { renderModal } from "../src/components/modals.js";

function harness(apiOverrides = {}) {
  const state = {
    providers: [{ id: "gateway", selectedModelId: "model", models: [{ id: "model" }] }],
    selectedProviderId: "gateway",
    modal: null,
    presets: [],
    testPending: false,
    modelMenuOpen: false
  };
  const store = {
    getState: () => state,
    setState(update) { Object.assign(state, update(state)); }
  };
  const calls = [];
  const feedback = {
    showLoading(title, message) {
      calls.push(["loading", title, message]);
      state.modal = { kind: "operation-loading", payload: { title, message } };
    },
    showResult(result) {
      calls.push(["result", result]);
      state.modal = { kind: "operation-result", payload: result };
    },
    showError(title, error) { calls.push(["error", title, error]); }
  };
  const api = {
    testModel: async () => ({ title: "模型测试成功", lines: ["gateway / model", "上游响应 200 OK · 12 ms"] }),
    listGlobalSkills: async () => ({ root: "C:\\Users\\me\\.agents\\skills", skills: [] }),
    deleteGlobalSkill: async () => ({ root: "C:\\Users\\me\\.agents\\skills", skills: [] }),
    ...apiOverrides
  };
  const actions = createAppActions({ root: {}, api, store, feedback, applyTheme() {} });
  return { state, actions, calls };
}

test("model test sends the selected provider and model with pending feedback", async () => {
  let release;
  const pending = new Promise((resolve) => { release = resolve; });
  const run = harness({
    testModel: async (providerId, modelId) => {
      assert.equal(providerId, "gateway");
      assert.equal(modelId, "model");
      return pending;
    }
  });
  const testing = run.actions.testModel();
  assert.equal(run.state.testPending, true);
  assert.equal(run.state.modal.kind, "operation-loading");
  release({ title: "模型测试成功", lines: ["gateway / model", "上游响应 200 OK · 12 ms"] });
  await testing;
  assert.equal(run.state.testPending, false);
  assert.equal(run.state.modal.kind, "operation-result");
  assert.deepEqual(run.state.modal.payload.details, ["gateway / model", "上游响应 200 OK · 12 ms"]);
});

test("skill manager escapes content and deletes only after confirmation", async () => {
  const skill = { name: "unsafe<skill>", description: "Use <carefully>", path: "C:\\skills\\unsafe<skill>", locked: true };
  const remaining = { root: "C:\\skills", skills: [] };
  const run = harness({
    listGlobalSkills: async () => ({ root: "C:\\skills", skills: [skill] }),
    deleteGlobalSkill: async (name) => {
      assert.equal(name, skill.name);
      return remaining;
    }
  });
  await run.actions.openGlobalSkills();
  assert.equal(run.state.modal.kind, "skill-manager");
  const markup = renderModal({ modal: run.state.modal });
  assert.match(markup, /unsafe&lt;skill&gt;/);
  assert.match(markup, /Use &lt;carefully&gt;/);
  assert.doesNotMatch(markup, /unsafe<skill>/);
  run.actions.requestDeleteGlobalSkill(skill.name);
  assert.equal(run.state.modal.kind, "confirm-delete-skill");
  assert.match(renderModal({ modal: run.state.modal }), /永久删除，无法恢复/);
  await run.actions.confirmDeleteGlobalSkill();
  assert.equal(run.state.modal.kind, "skill-manager");
  assert.deepEqual(run.state.modal.payload, remaining);
});
