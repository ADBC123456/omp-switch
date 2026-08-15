import { escapeHtml } from "./view-utils.js";

const CHECKBOX_SVG = '<svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="1.5" y="1.5" width="21" height="21" rx="6" pathLength="88"/><polyline points="6.5 12.5 10 17 18 7.5" pathLength="30"/></svg>';

export function renderCheckbox({ checked = false, dataset = {}, label = "", content = "", className = "" } = {}) {
  const attrs = Object.entries(dataset)
    .map(([key, value]) => `data-${key.replace(/[A-Z]/g, (letter) => `-${letter.toLowerCase()}`)}="${escapeHtml(String(value))}"`)
    .join(" ");
  const labelAttr = label ? ` aria-label="${escapeHtml(label)}"` : "";
  return `<label class="checkbox ${className}"${labelAttr}><input type="checkbox" ${attrs} ${checked ? "checked" : ""}><span class="checkmark">${CHECKBOX_SVG}${content}</span></label>`;
}
