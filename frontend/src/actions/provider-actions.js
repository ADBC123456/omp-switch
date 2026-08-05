import { providerPreset } from "../config/presets.js";
import { discoveryCounts, filterNewModels, groupDiscovery, toggleFilteredSelection } from "../domain/discovery.js";
import { normalizeModelInput, validateModelInput } from "../domain/provider-draft.js";

export function currentProvider(state) {
  return state.providers.find((provider) => provider.id === state.selectedProviderId) ?? state.providers[0];
}

function adoptBackendState(state, backend, ui = {}) {
  return { ...state, ...backend, presets: state.presets, modal: null, modelMenuOpen: false, ...ui };
}


function readModel(root) {
  return normalizeModelInput({
    id: root.querySelector('[name="modelId"]')?.value,
    name: root.querySelector('[name="modelName"]')?.value,
    api: root.querySelector('[name="modelApi"]')?.value,
    reasoning: root.querySelector('[name="modelReasoning"]')?.value,
    contextWindow: root.querySelector('[name="modelContextWindow"]')?.value,
    maxTokens: root.querySelector('[name="modelMaxTokens"]')?.value
  });
}

export function createProviderActions({ root, api, store, providerForm, feedback }) {
  function createFromPreset(presetId) { providerForm.openNew(providerPreset(presetId)); }

  async function fetchModels() {
    const saved = await providerForm.save();
    if (!saved) return;
    const providerId = saved.finalProviderId;
    const provider = saved.state.providers.find((item) => item.id === providerId);
    const requestId = crypto.randomUUID();
    store.setState((state) => ({ ...state, modal: { kind: "operation-loading", payload: { title: "正在获取模型", message: `正在连接 ${provider?.name ?? providerId}`, requestId } } }));
    try {
      const result = await api.fetchModels(providerId, requestId);
      const latest = store.getState().providers.find((item) => item.id === providerId);
      const groups = groupDiscovery(latest?.models ?? [], result.models ?? []);
      store.setState((state) => ({ ...state, modal: { kind: "discovery-review", payload: { providerId, ...groups, warnings: result.warnings ?? [], query: "", selected: groups.added.map((model) => model.id) } } }));
    } catch (error) {
      feedback.showError("获取模型失败", error);
    }
  }

  async function cancelDiscovery() {
    const requestId = store.getState().modal?.payload?.requestId;
    if (!requestId) return;
    try { await api.cancelModelDiscovery(requestId); } catch (error) { feedback.showError("取消获取失败", error); }
  }

  function updateReviewQuery(query) {
    store.setState((state) => {
      if (state.modal?.kind !== "discovery-review" || state.modal.payload.query === query) return state;
      return { ...state, modal: { ...state.modal, payload: { ...state.modal.payload, query } } };
    });
  }
  function toggleReviewModel(id, checked) {
    store.setState((state) => {
      if (state.modal?.kind !== "discovery-review") return state;
      const selected = new Set(state.modal.payload.selected);
      checked ? selected.add(id) : selected.delete(id);
      return { ...state, modal: { ...state.modal, payload: { ...state.modal.payload, selected: [...selected] } } };
    });
  }
  function toggleFilteredReview() {
    store.setState((state) => {
      if (state.modal?.kind !== "discovery-review") return state;
      const payload = state.modal.payload;
      const selected = toggleFilteredSelection(payload.added, payload.query, new Set(payload.selected));
      return { ...state, modal: { ...state.modal, payload: { ...payload, selected: [...selected] } } };
    });
  }
  async function importModels() {
    const modal = store.getState().modal;
    if (modal?.kind !== "discovery-review") return;
    const selectedIds = new Set(modal.payload.selected);
    const selected = modal.payload.added.filter((model) => selectedIds.has(model.id));
    try {
      const backend = await api.importDiscoveredModels(modal.payload.providerId, selected);
      store.setState((state) => adoptBackendState(state, backend, { selectedProviderId: modal.payload.providerId }));
    } catch (error) { feedback.showError("导入模型失败", error); }
  }

  function openModelEditor(modelId = "") {
    const provider = currentProvider(store.getState());
    if (!provider) return;
    const model = provider.models?.find((item) => item.id === modelId) ?? { id: "", name: "", api: "", reasoning: null, contextWindow: 0, maxTokens: 0 };
    store.setState((state) => ({ ...state, modal: { kind: "model-editor", payload: { providerId: provider.id, originalId: modelId, model, error: "" } } }));
  }
  async function saveModel() {
    const modal = store.getState().modal;
    if (modal?.kind !== "model-editor") return;
    const provider = store.getState().providers.find((item) => item.id === modal.payload.providerId);
    const model = readModel(root);
    const message = validateModelInput(model, provider?.models ?? [], modal.payload.originalId);
    if (message) {
      store.setState((state) => ({ ...state, modal: { ...state.modal, payload: { ...state.modal.payload, error: message } } }));
      return;
    }
    try {
      const backend = await api.saveModel(provider.id, modal.payload.originalId, model);
      store.setState((state) => adoptBackendState(state, backend, { selectedProviderId: provider.id }));
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      if (message.includes("冲突")) {
        store.setState((state) => ({ ...state, modal: null }));
        queueMicrotask(() => {
          const row = root.querySelector(`[data-model-row="${CSS.escape(model.id)}"]`);
          row?.classList.add("is-conflict");
          row?.querySelector("button")?.focus();
          window.setTimeout(() => row?.classList.remove("is-conflict"), 1400);
        });
      } else feedback.showError("保存模型失败", error);
    }
  }

  async function requestDeleteModel(modelId) {
    const provider = currentProvider(store.getState());
    if (!provider) return;
    try {
      const roles = await api.getModelDeleteImpact(provider.id, modelId);
      store.setState((state) => ({ ...state, modal: { kind: "confirm-delete-model", payload: { providerId: provider.id, modelId, roles } } }));
    } catch (error) { feedback.showError("检查模型引用失败", error); }
  }
  async function confirmDeleteModel() {
    const payload = store.getState().modal?.payload;
    if (!payload?.providerId || !payload?.modelId) return;
    try {
      const backend = await api.deleteModel(payload.providerId, payload.modelId);
      store.setState((state) => adoptBackendState(state, backend, { selectedProviderId: payload.providerId }));
    } catch (error) { feedback.showError("删除模型失败", error); }
  }

  async function requestDeleteProvider(providerId = "") {
    const drawer = store.getState().drawer;
    const id = typeof providerId === "string" ? providerId : "";
    const provider = store.getState().providers.find((item) => item.id === (id || drawer?.originalId));
    if (!provider) return;
    try {
      const roles = await api.getProviderDeleteImpact(provider.id);
      store.setState((state) => ({ ...state, modal: { kind: "confirm-delete-provider", payload: { id: provider.id, name: provider.name, roles } } }));
    } catch (error) { feedback.showError("检查 Provider 引用失败", error); }
  }
  async function confirmDeleteProvider() {
    const id = store.getState().modal?.payload?.id;
    if (!id) return;
    try {
      const backend = await api.deleteProvider(id);
      store.setState((state) => adoptBackendState(state, backend, { drawer: null }));
    } catch (error) { feedback.showError("删除 Provider 失败", error); }
  }

  return { createFromPreset, fetchModels, cancelDiscovery, updateReviewQuery, toggleReviewModel, toggleFilteredReview, importModels, openModelEditor, saveModel, requestDeleteModel, confirmDeleteModel, requestDeleteProvider, confirmDeleteProvider, reviewCounts: (payload) => discoveryCounts(payload.added, payload.query, new Set(payload.selected)), visibleReviewModels: (payload) => filterNewModels(payload.added, payload.query) };
}
