import { escapeHtml } from "./view-utils.js";
import { MANAGED_ROLES, THINKING_LEVELS, parseRoleValue } from "../domain/model-roles.js";

function modelOptions(providers, selected) {
  const options = ['<option value="">未设置</option>'];
  for (const provider of providers ?? []) {
    for (const model of provider.models ?? []) {
      const value = `${provider.id}/${model.id}`;
      options.push(`<option value="${escapeHtml(value)}" ${value === selected ? "selected" : ""}>${escapeHtml(value)}</option>`);
    }
  }
  return options.join("");
}

export function roleRows(state) {
  return MANAGED_ROLES.map((role) => {
    const parsed = parseRoleValue(state.modelRoles?.[role], state.providers);
    return { role, ...parsed, dirty: false };
  });
}

export function renderModelRoles(state, payload) {
  const rows = payload.rows ?? roleRows(state);
  return `<div class="role-editor">${rows.map((row) => `<div class="role-row" data-role-row="${row.role}"><div class="role-row__name"><strong>${row.role}</strong>${row.kind === "custom" ? `<small>自定义值：${escapeHtml(row.raw)}</small>` : ""}</div>${row.kind === "custom" ? `<button class="text-button" type="button" data-convert-role="${row.role}">改为模型</button>` : `<select data-role-model="${row.role}">${modelOptions(state.providers, row.model)}</select><select data-role-thinking="${row.role}" ${row.model ? "" : "disabled"}><option value="">继承</option>${THINKING_LEVELS.map((level) => `<option value="${level}" ${level === row.thinking ? "selected" : ""}>${level}</option>`).join("")}</select>`}</div>`).join("")}</div>`;
}
