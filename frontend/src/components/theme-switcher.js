import { icon } from "./icons.js";

const STATES = ["light", "system", "dark"];
const LABELS = { light: "浅色", system: "跟随系统", dark: "深色" };
const GLYPHS = { light: "sun", system: "system", dark: "moon" };

export function normalizeThemeState(state) {
  return STATES.includes(state) ? state : "system";
}

export function renderThemeSwitcher(state) {
  const current = normalizeThemeState(state);
  const options = STATES.map((value) => `<button class="theme-switcher__option" type="button" data-set-theme="${value}" aria-label="${LABELS[value]}" aria-pressed="${current === value}">${icon(GLYPHS[value])}</button>`).join("");
  return `<div class="theme-switcher" data-theme-switcher data-theme-state="${current}" role="group" aria-label="界面主题">${options}</div>`;
}

export class ThemeSwitcher {
  constructor(element) {
    this.element = element;
  }

  get state() { return normalizeThemeState(this.element.dataset.themeState); }

  setState(nextState) {
    const next = normalizeThemeState(nextState);
    if (next === this.state) return false;
    this.element.dataset.themeState = next;
    this.element.querySelectorAll("[data-set-theme]").forEach((button) => button.setAttribute("aria-pressed", String(button.dataset.setTheme === next)));
    return true;
  }
}
