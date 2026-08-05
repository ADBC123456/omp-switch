import { escapeHtml, extractHost } from "./view-utils.js";
import { icon } from "./icons.js";
import { renderThemeSwitcher } from "./theme-switcher.js";

function endpointFor(provider, model) {
  const api = model?.api || provider?.api || "";
  if (api === "openai-responses") return "/v1/responses";
  if (api === "anthropic-messages") return "/v1/messages";
  if (api === "google-generative-ai") return "/v1beta/models";
  return "/v1/chat/completions";
}


function configRows(provider, selected, defaultRole) {
  const rows = [
    ["provider", "Provider", provider ? `${provider.name} (${extractHost(provider.baseUrl)})` : "未配置"],
    ["model", "运行模型", selected?.name || selected?.id || "未配置"],
    ["role", "默认角色", defaultRole || "未配置"],
    ["api", "API 路径", provider ? endpointFor(provider, selected) : "未配置"],
    ["clock", "超时时间", "未配置"],
    ["restart", "请求重试", "未配置"]
  ];
  return rows.map(([glyph, label, value]) => `<div class="config-row">${icon(glyph)}<span>${label}</span><strong title="${escapeHtml(value)}">${escapeHtml(value)}</strong></div>`).join("");
}

function quickAction(glyph, title, description, attribute, disabled = false) {
  return `<button class="quick-action glow-button" ${attribute} ${disabled ? "disabled" : ""}>${icon(glyph)}<span><strong>${title}</strong><small>${description}</small></span></button>`;
}

export function renderMainPanel(state, provider) {
  const models = provider?.models ?? [];
  const selected = models.find((model) => model.id === provider?.selectedModelId) ?? models[0];
  const defaultRole = state.modelRoles?.default || "";
  const modelName = selected?.name || selected?.id || "请先添加模型";
  const canLaunch = Boolean(provider && selected && !state.launchPending);
  return `<main class="main-panel dashboard-main">
    <div class="desktop-titlebar" data-wails-drag><div class="window-actions"><button class="window-symbol" data-window-minimise aria-label="最小化">−</button><button class="window-symbol" data-window-toggle-maximise aria-label="最大化">□</button><button class="window-symbol window-symbol--danger" data-window-close aria-label="关闭">×</button></div></div>
    <div class="dashboard-scroll">
      <header class="dashboard-heading"><div><h1>欢迎使用 OMP Switch</h1><p>轻松切换与管理大模型服务</p></div>${renderThemeSwitcher(state.settings?.theme || "system")}</header>
      <section class="dashboard-card model-card" aria-labelledby="model-card-title">
        <div class="section-heading"><h2 id="model-card-title">模型选择</h2><p>选择要使用的运行模型</p></div>
        <div class="model-picker ${state.modelMenuOpen ? "is-open" : ""}">
          <button class="model-picker__trigger glow-button" data-toggle-model-menu aria-haspopup="listbox" aria-expanded="${state.modelMenuOpen}" ${models.length ? "" : "disabled"}><span class="model-picker__copy"><strong>${escapeHtml(modelName)}</strong>${selected ? "<small>默认</small>" : ""}</span>${icon("down")}</button>
          ${models.length && state.modelMenuOpen ? `<div class="model-picker__menu" role="listbox">${models.map((model) => `<button data-select-model="${escapeHtml(model.id)}" role="option" aria-selected="${model.id === selected?.id}" class="${model.id === selected?.id ? "is-selected" : ""}"><span>${escapeHtml(model.name || model.id)}</span><small>${escapeHtml(model.id)}</small></button>`).join("")}</div>` : ""}
        </div>
        <div class="model-card__actions"><button class="detail-button glow-button" data-open-model-details ${selected ? "" : "disabled"}>${icon("info")}<span>模型详情</span></button><button class="launch-button glow-button ${state.launchPending ? "is-loading" : ""}" data-launch-omp ${canLaunch ? "" : "disabled"}><span class="launch-button__liquid"></span><span class="launch-button__content">${state.launchPending ? '<i class="button-spinner"></i><strong>正在启动</strong>' : `${icon("play")}<strong>启动 OMP</strong>${icon("arrow")}`}</span></button></div>
      </section>
      <div class="dashboard-grid">
        <section class="dashboard-card config-card" aria-labelledby="config-title"><div class="section-heading"><h2 id="config-title">配置概览</h2></div><div class="config-list">${configRows(provider, selected, defaultRole)}</div><button class="detail-button glow-button" data-open-config-folder>${icon("config")}<span>查看完整配置</span>${icon("chevron")}</button></section>
        <section class="dashboard-card quick-card" aria-labelledby="quick-title"><div class="section-heading"><h2 id="quick-title">快捷操作</h2></div><div class="quick-grid">${quickAction("restart", "重新启动", "重新启动本应用启动的 OMP", "data-restart-omp", state.launchPending)}${quickAction("logs", "查看日志", "查看应用运行日志", "data-open-logs")}${quickAction("rocket", "OMP 官方文档", "打开 omp.sh", "data-open-api-doc")}${quickAction("plus", "添加 Provider", "配置新的 Provider", "data-open-add-provider")}</div></section>
      </div>
    </div>
  </main>`;
}
