import { createDraftFromPreset, createDraftFromProvider, readProviderInput, validateProviderInput } from "../domain/provider-draft.js";

function setStatus(root, status, message) {
  const element = root.querySelector(".drawer-status");
  if (!element) return;
  element.dataset.status = status;
  element.textContent = message;
}

function mergeBackendState(state, backend, ui = {}) {
  return { ...state, ...backend, presets: state.presets, modal: null, modelMenuOpen: false, ...ui };
}

export function createProviderFormController({ root, api, store, feedback }) {
  function openExisting(providerId) {
    const provider = store.getState().providers.find((item) => item.id === providerId);
    if (!provider) return;
    store.setState((state) => ({ ...state, selectedProviderId: providerId, drawer: { kind: "provider", originalId: providerId, draft: createDraftFromProvider(provider) } }));
  }

  function openNew(preset) {
    store.setState((state) => ({ ...state, modal: null, drawer: { kind: "provider", originalId: "", draft: createDraftFromPreset(preset) } }));
  }

  function read() {
    const drawer = store.getState().drawer;
    const form = root.querySelector(".drawer");
    if (!drawer?.draft || !form) return null;
    return readProviderInput(form, drawer.draft);
  }

  async function save() {
    const drawer = store.getState().drawer;
    if (!drawer?.draft) return null;
    const input = read();
    if (!input) return null;
    const message = validateProviderInput(input, { creating: drawer.draft.creating, hasApiKey: drawer.draft.hasApiKey });
    if (message) {
      setStatus(root, "error", message);
      root.querySelector(`[name="${message.startsWith("Provider ID") ? "id" : message.startsWith("Base URL") ? "baseUrl" : message.startsWith("API Key") ? "apiKey" : "api"}"]`)?.focus();
      return null;
    }
    setStatus(root, "saving", "正在保存");
    try {
      const result = drawer.originalId ? await api.updateProvider(drawer.originalId, input) : await api.createProvider(input);
      const finalId = result.finalProviderId;
      const view = result.state.providers.find((item) => item.id === finalId);
      store.setState((state) => mergeBackendState(state, result.state, { selectedProviderId: finalId, drawer: { kind: "provider", originalId: finalId, draft: createDraftFromProvider(view) } }));
      queueMicrotask(() => setStatus(root, "saved", result.adjusted ? `Provider ID 已调整为 ${finalId}` : "已保存"));
      return result;
    } catch (error) {
      setStatus(root, "error", "保存失败");
      feedback.showError("配置保存失败", error);
      return null;
    }
  }

  function cancel() {
    store.setState((state) => ({ ...state, drawer: null }));
  }

  function patchDraft(patch) {
    store.setState((state) => ({ ...state, drawer: state.drawer?.draft ? { ...state.drawer, draft: { ...state.drawer.draft, ...patch } } : state.drawer }));
  }

  return { openExisting, openNew, read, save, cancel, patchDraft };
}
