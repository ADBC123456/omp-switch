import { currentProvider } from "./provider-actions.js";
import { buildRoleUpdates } from "../domain/model-roles.js";
import { roleRows } from "../components/model-roles.js";

function adopt(state, backend, ui = {}) { return { ...state, ...backend, presets: state.presets, modal: null, modelMenuOpen: false, ...ui }; }

export function createAppActions({ root, api, store, feedback, applyTheme }) {
  async function selectProvider(id) {
    try { const backend = await api.setSelectedProvider(id); store.setState((state) => adopt(state, backend)); }
    catch (error) { feedback.showError("选择 Provider 失败", error); }
  }
  async function selectModel(modelId) {
    const provider = currentProvider(store.getState()); if (!provider) return;
    try { const backend = await api.setSelectedModel(provider.id, modelId); store.setState((state) => adopt(state, backend)); }
    catch (error) { feedback.showError("选择模型失败", error); }
  }
  async function directLaunch() {
    const state = store.getState();
    const provider = currentProvider(state);
    if (!provider?.selectedModelId || state.launchPending) return;
    store.setState((current) => ({ ...current, launchPending: true, modelMenuOpen: false }));
    try { await api.executeLaunchOMP(provider.id, provider.selectedModelId); }
    catch (error) { feedback.showError("启动 OMP 失败", error); }
    finally { store.setState((current) => ({ ...current, launchPending: false })); }
  }
  async function restartOMP() {
    if (store.getState().launchPending) return;
    store.setState((current) => ({ ...current, launchPending: true, modelMenuOpen: false }));
    try { await api.restartOMP(); }
    catch (error) { feedback.showError("重新启动 OMP 失败", error); }
    finally { store.setState((current) => ({ ...current, launchPending: false })); }
  }
  async function openSessions() {
    store.setState((state) => ({ ...state, modal: { kind: "operation-loading", payload: { title: "加载会话", message: "正在读取 OMP 会话…" } } }));
    try {
      const sessions = await api.listSessions();
      store.setState((state) => ({ ...state, modal: { kind: "session-manager", payload: { sessions } } }));
    } catch (error) { feedback.showError("读取 OMP 会话失败", error); }
  }
  async function continueSession(id) {
    if (!id || store.getState().launchPending) return;
    store.setState((state) => ({ ...state, launchPending: true, modal: { kind: "session-manager", payload: { ...state.modal?.payload, pendingSessionId: id } } }));
    try {
      await api.continueSession(id);
      store.setState((state) => ({ ...state, launchPending: false, modal: null }));
    } catch (error) {
      store.setState((state) => ({ ...state, launchPending: false }));
      feedback.showError("继续 OMP 会话失败", error);
    }
  }
  function requestDeleteSession(session) {
    if (!session || store.getState().launchPending) return;
    store.setState((state) => ({ ...state, modal: { kind: "confirm-delete-session", payload: { session, sessions: state.modal?.payload?.sessions ?? [] } } }));
  }
  async function confirmDeleteSession() {
    const session = store.getState().modal?.payload?.session;
    if (!session?.id) return;
    try {
      const sessions = await api.deleteSession(session.id);
      store.setState((state) => ({ ...state, modal: { kind: "session-manager", payload: { sessions } } }));
    } catch (error) { feedback.showError("删除 OMP 会话失败", error); }
  }
  async function saveSettings() {
    const next = { ompCommand: root.querySelector('[name="ompCommand"]')?.value.trim(), workingDir: root.querySelector('[name="workingDir"]')?.value.trim(), theme: root.querySelector('[name="theme"]')?.value ?? "system" };
    try { const backend = await api.updateSettings(next); applyTheme(backend.settings); store.setState((state) => adopt(state, backend)); }
    catch (error) { feedback.showError("保存应用设置失败", error); }
  }
  function openRoles() { store.setState((state) => ({ ...state, modal: { kind: "model-roles", payload: { rows: roleRows(state) } } })); }
  function updateRole(role, patch) {
    store.setState((state) => {
      if (state.modal?.kind !== "model-roles") return state;
      return { ...state, modal: { ...state.modal, payload: { rows: state.modal.payload.rows.map((row) => row.role === role ? { ...row, ...patch, dirty: true, kind: "model" } : row) } } };
    });
  }
  async function saveRoles() {
    const rows = store.getState().modal?.payload?.rows ?? [];
    try { const backend = await api.updateModelRoles(buildRoleUpdates(rows)); store.setState((state) => adopt(state, backend)); }
    catch (error) { feedback.showError("保存模型角色失败", error); }
  }
  async function checkUpdate() {
    const status = root.querySelector("[data-update-status]"); if (status) status.textContent = "正在检查…";
    try { const result = await api.checkForUpdate(); if (result.hasUpdate) store.setState((state) => ({ ...state, modal: { kind: "update-available", payload: result } })); else if (status) status.textContent = "已是最新版本"; }
    catch (error) { feedback.showError("检查更新失败", error); }
  }
  async function skipUpdate() { try { await api.markUpdateChecked(); } catch (error) { feedback.showError("记录跳过失败", error); } store.setState((state) => ({ ...state, modal: null })); }
  async function installUpdate() { try { await api.installUpdate(); api.closeWindow(); } catch (error) { feedback.showError("更新失败", error); } }
  return { selectProvider, selectModel, directLaunch, restartOMP, openSessions, continueSession, requestDeleteSession, confirmDeleteSession, saveSettings, openRoles, updateRole, saveRoles, checkUpdate, skipUpdate, installUpdate };
}
