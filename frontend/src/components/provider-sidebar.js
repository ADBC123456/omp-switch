import { escapeHtml, extractHost } from "./view-utils.js";
import { icon } from "./icons.js";
import ompLogo from "../assets/omp-logo.svg";

const navItems = [
  ["overview", "概览", "data-nav-overview"],
  ["model", "模型管理", "data-open-current-provider"],
  ["provider", "Provider 管理", "data-open-provider-manager"],
  ["session", "会话管理", "data-open-sessions"],
  ["config", "配置文件", "data-open-config-folder"],
  ["settings", "设置", "data-open-settings"]
];

export function renderProviderSidebar(state, provider) {
  const host = extractHost(provider?.baseUrl) || "未配置地址";
  const providerName = provider?.name || "未选择 Provider";
  return `<aside class="sidebar dashboard-sidebar">
    <header class="aside-header" data-wails-drag><img class="brand-mark" src="${ompLogo}" alt=""><strong>OMP Switch</strong></header>
    <nav class="dashboard-nav" aria-label="主导航">${navItems.map(([glyph, label, action], index) => `<button class="dashboard-nav__item ${index === 0 ? "is-active" : ""}" ${action}>${icon(glyph)}<span>${label}</span></button>`).join("")}</nav>
    <section class="current-provider-card" aria-label="当前 Provider">
      <div class="current-provider-card__heading"><span>当前 Provider</span><span class="provider-ready"><i></i>${provider ? "配置就绪" : "未配置"}</span></div>
      <button class="provider-summary" data-open-current-provider ${provider ? "" : "disabled"}>
        <span class="provider-avatar">${escapeHtml((providerName[0] || "P").toUpperCase())}</span>
        <span><strong>${escapeHtml(providerName)}</strong><small>${escapeHtml(host)}</small></span>${icon("chevron")}
      </button>
      <button class="secondary-glass glow-button" data-toggle-provider-switcher ${state.providers.length > 1 ? "" : "disabled"}><span>切换 Provider</span>${icon("restart")}</button>
      ${state.providerMenuOpen ? `<div class="provider-switcher" role="menu">${state.providers.map((item) => `<button data-provider-id="${escapeHtml(item.id)}" role="menuitem" class="${item.id === state.selectedProviderId ? "is-selected" : ""}"><span>${escapeHtml(item.name)}</span><small>${escapeHtml(extractHost(item.baseUrl))}</small></button>`).join("")}</div>` : ""}
    </section>
    <footer class="sidebar-meta"><span><i></i>OMP 配置就绪</span><span>v${escapeHtml(state.version)}</span></footer>
  </aside>`;
}
