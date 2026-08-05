import { createAppActions } from "../actions/app-actions.js";
import { createOperationFeedback } from "../actions/operation-feedback.js";
import { createProviderActions } from "../actions/provider-actions.js";
import { renderApp, renderContentLayer, renderDrawerLayer } from "../components/app-shell.js";
import { renderModal } from "../components/modals.js";
import { ThemeSwitcher } from "../components/theme-switcher.js";
import { PRESETS } from "../config/presets.js";
import { createProviderFormController } from "../controllers/provider-form-controller.js";
import { WailsApi } from "../services/wails-api.js";
import { createStore } from "../state/store.js";
import { ThemeManager } from "../theme/theme-manager.js";
import { bindGlowButtons } from "../ui/glow-effect.js";

const root = document.querySelector("#app");
const api = new WailsApi();
const store = createStore({ version: "1.0", providers: [], selectedProviderId: "", modelRoles: {}, settings: { ompCommand: "omp", workingDir: "", theme: "system" }, paths: {}, logs: [], presets: PRESETS, modal: null, drawer: null, modelMenuOpen: false, providerMenuOpen: false, launchPending: false });
const feedback = createOperationFeedback(store);
const themeManager = new ThemeManager({ api, store });
const providerForm = createProviderFormController({ root, api, store, feedback });
const providerActions = createProviderActions({ root, api, store, providerForm, feedback });
const appActions = createAppActions({ root, api, store, feedback, applyTheme: (settings) => themeManager.initialise(settings) });

function adopt(data) {
  store.setState((state) => ({ ...state, ...data, presets: state.presets, drawer: null, modal: null, modelMenuOpen: false, providerMenuOpen: false, launchPending: state.launchPending }));
}
async function reloadConfig() {
  const data = await api.getAppState();
  themeManager.initialise(data.settings);
  adopt(data);
}
const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
let overlayExit = null;
function animateOverlayIn(selector, keyframes) {
  const element = root.querySelector(selector);
  if (!element || reduceMotion.matches) return;
  element.getAnimations().forEach((animation) => animation.cancel());
  element.animate(keyframes, { duration: 320, easing: "cubic-bezier(.16, 1, .3, 1)", fill: "both" });
}
async function closeOverlay(kind) {
  const selector = kind === "modal" ? ".modal-backdrop" : ".drawer";
  const element = root.querySelector(selector);
  if (!element || reduceMotion.matches) {
    store.setState((state) => ({ ...state, [kind]: null }));
    return;
  }
  overlayExit?.cancel();
  const style = getComputedStyle(element);
  overlayExit = element.animate([
    { opacity: style.opacity, transform: style.transform },
    { opacity: 0, transform: kind === "modal" ? "translateY(10px) scale(.985)" : "translateX(28px) scale(.992)" }
  ], { duration: 220, easing: "cubic-bezier(.4, 0, 1, 1)", fill: "forwards" });
  try { await overlayExit.finished; } catch { return; }
  store.setState((state) => ({ ...state, [kind]: null }));
}
function closeModal() { if (store.getState().modal?.kind !== "operation-loading") return closeOverlay("modal"); }
function openModal(kind, payload = {}) { overlayExit?.cancel(); store.setState((state) => ({ ...state, modal: { kind, payload }, modelMenuOpen: false, providerMenuOpen: false })); }
function toggleModelMenu() { store.setState((state) => ({ ...state, modelMenuOpen: !state.modelMenuOpen, providerMenuOpen: false })); }
function toggleProviderMenu() { store.setState((state) => ({ ...state, providerMenuOpen: !state.providerMenuOpen, modelMenuOpen: false })); }
function currentProvider() { const state = store.getState(); return state.providers.find((item) => item.id === state.selectedProviderId) ?? state.providers[0]; }
function currentModel() { const provider = currentProvider(); return provider?.models?.find((item) => item.id === provider.selectedModelId) ?? provider?.models?.[0]; }

const clickActions = {
  "data-nav-overview": () => store.setState((state) => ({ ...state, modelMenuOpen: false, providerMenuOpen: false })),
  "data-open-add-provider": () => openModal("add-provider"),
  "data-open-provider-manager": () => openModal("provider-manager"),
  "data-open-settings": () => openModal("settings"),
  "data-open-sessions": appActions.openSessions,
  "data-open-model-roles": appActions.openRoles,
  "data-open-current-provider": () => { const provider = currentProvider(); if (provider) providerForm.openExisting(provider.id); },
  "data-open-model-details": () => { const model = currentModel(); if (model) openModal("model-details", { model }); },
  "data-open-logs": () => openModal("logs", { logs: store.getState().logs ?? [] }),
  "data-open-config-folder": () => api.openConfigFolder().catch((error) => feedback.showError("打开配置目录失败", error)),
  "data-open-api-doc": () => api.openExternalURL("https://omp.sh"),
  "data-restart-omp": appActions.restartOMP,
  "data-toggle-provider-switcher": toggleProviderMenu,
  "data-close-modal": closeModal,
  "data-cancel-provider": () => closeOverlay("drawer"),
  "data-save-provider": providerForm.save,
  "data-fetch-models": providerActions.fetchModels,
  "data-cancel-discovery": providerActions.cancelDiscovery,
  "data-import-models": providerActions.importModels,
  "data-toggle-filtered-review": providerActions.toggleFilteredReview,
  "data-add-model": () => providerActions.openModelEditor(),
  "data-save-model": providerActions.saveModel,
  "data-delete-provider": providerActions.requestDeleteProvider,
  "data-confirm-delete-provider": providerActions.confirmDeleteProvider,
  "data-confirm-delete-model": providerActions.confirmDeleteModel,
  "data-confirm-delete-session": appActions.confirmDeleteSession,
  "data-return-sessions": () => store.setState((state) => ({ ...state, modal: { kind: "session-manager", payload: { sessions: state.modal?.payload?.sessions ?? [] } } })),
  "data-save-settings": appActions.saveSettings,
  "data-save-roles": appActions.saveRoles,
  "data-toggle-model-menu": toggleModelMenu,
  "data-launch-omp": appActions.directLaunch,
  "data-window-minimise": () => api.minimiseWindow(),
  "data-window-toggle-maximise": () => api.toggleMaximiseWindow(),
  "data-window-close": () => api.closeWindow(),
  "data-check-update": appActions.checkUpdate,
  "data-skip-update": appActions.skipUpdate,
  "data-install-update": appActions.installUpdate
};

root.addEventListener("click", async (event) => {
  const target = event.target.closest("button");
  if (!target) { if (event.target.classList.contains("modal-backdrop")) await closeModal(); return; }
  if (target.dataset.providerId) return appActions.selectProvider(target.dataset.providerId);
  if (target.dataset.selectModel) return appActions.selectModel(target.dataset.selectModel);
  if (target.dataset.setTheme) {
    if (themeManager.animating) return;
    const element = target.closest("[data-theme-switcher]");
    const switcher = element ? new ThemeSwitcher(element) : null;
    switcher?.setState(target.dataset.setTheme);
    try { await themeManager.setMode(target.dataset.setTheme, target); }
    catch (error) { switcher?.setState(themeManager.mode); feedback.showError("切换主题失败", error); }
    return;
  }
  if (target.dataset.openProviderSettings) return providerForm.openExisting(target.dataset.openProviderSettings);
  if (target.dataset.deleteProviderId) return providerActions.requestDeleteProvider(target.dataset.deleteProviderId);
  if (target.dataset.continueSession) return appActions.continueSession(target.dataset.continueSession);
  if (target.dataset.deleteSession) {
    const session = store.getState().modal?.payload?.sessions?.find((item) => item.id === target.dataset.deleteSession);
    return appActions.requestDeleteSession(session);
  }
  if (target.dataset.presetId) return providerActions.createFromPreset(target.dataset.presetId);
  if (target.dataset.editModel) return providerActions.openModelEditor(target.dataset.editModel);
  if (target.dataset.deleteModel) return providerActions.requestDeleteModel(target.dataset.deleteModel);
  if (target.hasAttribute("data-toggle-theme")) return themeManager.toggle(target).catch((error) => feedback.showError("切换主题失败", error));
  if (target.dataset.convertRole) return appActions.updateRole(target.dataset.convertRole, { model: "", thinking: "" });
  const attribute = Object.keys(clickActions).find((name) => target.hasAttribute(name));
  if (attribute) await clickActions[attribute](target);
});

root.addEventListener("input", (event) => {
  if (event.target.matches("[data-discovery-search]")) providerActions.updateReviewQuery(event.target.value);
});
root.addEventListener("change", (event) => {
  const target = event.target;
  if (target.dataset.reviewModel) providerActions.toggleReviewModel(target.dataset.reviewModel, target.checked);
  if (target.dataset.roleModel) appActions.updateRole(target.dataset.roleModel, { model: target.value, thinking: "" });
  if (target.dataset.roleThinking) appActions.updateRole(target.dataset.roleThinking, { thinking: target.value });
});

let renderedState = null;
function renderState(state) {
  if (!renderedState) root.innerHTML = renderApp(state);
  else {
    const content = root.querySelector("[data-content-layer]"); const drawer = root.querySelector("[data-drawer-layer]"); const modal = root.querySelector("[data-modal-layer]");
    if (!content || !drawer || !modal) root.innerHTML = renderApp(state);
    else {
      const contentChanged = renderedState.providers !== state.providers || renderedState.selectedProviderId !== state.selectedProviderId || renderedState.modelRoles !== state.modelRoles || renderedState.modelMenuOpen !== state.modelMenuOpen || renderedState.providerMenuOpen !== state.providerMenuOpen || renderedState.launchPending !== state.launchPending;
      if (contentChanged) {
        content.innerHTML = renderContentLayer(state);
        if (renderedState.selectedProviderId !== state.selectedProviderId || renderedState.providers !== state.providers) animateOverlayIn(".dashboard-main", [{ opacity: .82, transform: "translateX(7px)" }, { opacity: 1, transform: "translateX(0)" }]);
      }
      if (renderedState.drawer !== state.drawer || renderedState.providers !== state.providers) {
        drawer.innerHTML = renderDrawerLayer(state);
        if (!renderedState.drawer && state.drawer) animateOverlayIn(".drawer", [{ opacity: 0, transform: "translateX(28px) scale(.992)" }, { opacity: 1, transform: "translateX(0) scale(1)" }]);
      }
      if (renderedState.modal !== state.modal || (state.modal?.kind === "settings" && renderedState.settings !== state.settings)) {
        modal.innerHTML = renderModal(state);
        if (!renderedState.modal && state.modal) animateOverlayIn(".modal-backdrop", [{ opacity: 0, transform: "translateY(10px) scale(.985)" }, { opacity: 1, transform: "translateY(0) scale(1)" }]);
      }
    }
  }
  bindGlowButtons(root);
  renderedState = state; document.title = `OMP Switch v${state.version}`;
}
store.subscribe(renderState);
reloadConfig().catch((error) => { themeManager.initialise(store.getState().settings); feedback.showError("应用初始化失败", error); });
api.onConfigChanged(() => reloadConfig().catch((error) => feedback.showError("配置刷新失败", error)));
api.onUpdateAvailable((result) => store.setState((state) => ({ ...state, modal: { kind: "update-available", payload: result } })));
document.addEventListener("keydown", (event) => {
  if (event.key !== "Escape") return;
  const state = store.getState();
  if (state.modal) closeModal();
  else if (state.drawer) closeOverlay("drawer");
  else if (state.modelMenuOpen || state.providerMenuOpen) store.setState((current) => ({ ...current, modelMenuOpen: false, providerMenuOpen: false }));
});
