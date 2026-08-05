import { escapeHtml } from "./view-utils.js";

const API_OPTIONS = [
  ["openai-completions", "OpenAI Chat Completions"],
  ["openai-responses", "OpenAI Responses"],
  ["anthropic-messages", "Anthropic Messages"],
  ["google-generative-ai", "Google Generative AI"]
];

function field({ label, name, value, type = "text", placeholder = "" }) {
  return `<label class="form-field"><span class="form-field__label">${label}</span><input type="${type}" name="${name}" value="${escapeHtml(value ?? "")}" placeholder="${escapeHtml(placeholder)}" autocomplete="off"></label>`;
}

export function modelManageList(provider) {
  const models = provider?.models ?? [];
  return `<section class="form-section model-manage">
    <div class="model-manage__heading"><span class="form-field__label">模型管理${models.length ? ` · ${models.length}` : ""}</span><button class="text-button text-button--accent" type="button" data-add-model>添加模型</button></div>
    ${models.length ? `<div class="model-manage__list">${models.map((model) => `<div class="model-manage__row" data-model-row="${escapeHtml(model.id)}"><button class="model-manage__edit" type="button" data-edit-model="${escapeHtml(model.id)}"><strong>${escapeHtml(model.name || model.id)}</strong><small>${escapeHtml(model.id)}${model.contextWindow ? ` · ${Math.round(model.contextWindow / 1000)}K` : ""}</small></button><button class="model-manage__remove" type="button" data-delete-model="${escapeHtml(model.id)}" aria-label="删除 ${escapeHtml(model.name || model.id)}">删除</button></div>`).join("")}</div>` : `<p class="model-manage__empty">尚未配置模型。可手工添加，或保存 Provider 后获取上游模型。</p>`}
  </section>`;
}


export function renderProviderDrawer(state, provider) {
  const drawer = state.drawer;
  if (drawer?.kind !== "provider" || !drawer.draft) return "";
  const draft = drawer.draft;
  const creating = !drawer.originalId;
  return `<section class="drawer open" aria-label="Provider 配置">
    <header class="drawer-header"><div class="drawer-heading"><span class="eyebrow">PROVIDER CONFIG</span><h2>${escapeHtml(creating ? "新建 Provider" : draft.id)}</h2></div><button class="text-button drawer-close" data-cancel-provider>关闭</button></header>
    <div class="drawer-body"><section class="form-section">
      ${field({ label: "API 基础地址", name: "baseUrl", value: draft.baseUrl })}
      ${field({ label: "API Key", name: "apiKey", value: "", type: "password", placeholder: !creating && draft.hasApiKey ? "已配置；留空保持不变" : "环境变量名或密钥" })}
      <label class="form-field"><span class="form-field__label">接口协议</span><select name="api">${API_OPTIONS.map(([value, label]) => `<option value="${value}" ${value === draft.api ? "selected" : ""}>${label}</option>`).join("")}</select></label>
      ${field({ label: "Provider ID", name: "id", value: draft.id, placeholder: "同时作为显示名称" })}
    </section>${creating ? "" : modelManageList(provider)}</div>
    <footer class="drawer-footer"><span class="drawer-status" aria-live="polite"></span><div class="drawer-footer__actions">${creating ? "" : `<button class="text-button text-button--danger" data-delete-provider>删除</button><button class="text-button" data-fetch-models>获取模型</button>`}<button class="text-button" data-cancel-provider>取消</button><button class="text-button text-button--accent" data-save-provider>保存</button></div></footer>
  </section>`;
}
