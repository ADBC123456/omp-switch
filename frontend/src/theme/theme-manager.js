const MODES = new Set(["light", "dark", "system"]);
const WINDOW_BACKGROUNDS = {
  light: [243, 245, 247, 255],
  dark: [15, 17, 21, 255],
};

export function normalizeThemeMode(mode) {
  return MODES.has(mode) ? mode : "system";
}

export function resolveTheme(mode, systemDark) {
  const normalized = normalizeThemeMode(mode);
  return normalized === "system" ? (systemDark ? "dark" : "light") : normalized;
}


export class ThemeManager {
  constructor({ api, store, media = window.matchMedia("(prefers-color-scheme: dark)") }) {
    this.api = api;
    this.store = store;
    this.media = media;
    this.mode = "system";
    this.resolved = "light";
    this.animating = false;
    this.onSystemChange = () => {
      if (this.mode !== "system" || this.animating) return;
      this.applyResolved(resolveTheme(this.mode, this.media.matches));
    };
    this.media.addEventListener?.("change", this.onSystemChange);
  }

  initialise(settings) {
    this.mode = normalizeThemeMode(settings?.theme);
    this.applyResolved(resolveTheme(this.mode, this.media.matches));
    document.documentElement.dataset.themeReady = "true";
  }

  applyResolved(theme) {
    this.resolved = theme;
    document.documentElement.dataset.theme = theme;
    document.body.dataset.theme = theme;
    document.documentElement.style.colorScheme = theme;
    this.api.setWindowBackgroundColour?.(...WINDOW_BACKGROUNDS[theme]);
  }

  async toggle(button) {
    if (this.animating) return false;
    const mode = this.resolved === "dark" ? "light" : "dark";
    return this.transitionTo(mode, button);
  }
  async setMode(mode, button) {
    return this.transitionTo(mode, button);
  }

  async transitionTo(mode, button) {
    if (this.animating) return false;
    const nextMode = normalizeThemeMode(mode);
    const nextResolved = resolveTheme(nextMode, this.media.matches);
    if (nextMode === this.mode && nextResolved === this.resolved) return true;
    this.animating = true;
    const previousMode = this.mode;
    const previous = this.resolved;
    try {
      this.applyResolved(nextResolved);
      this.mode = nextMode;
      const settings = { ...this.store.getState().settings, theme: nextMode };
      delete settings.darkMode;
      const backend = await this.api.updateSettings(settings);
      this.store.setState((state) => ({ ...state, settings: backend.settings }));
      return true;
    } catch (error) {
      this.mode = previousMode;
      this.applyResolved(previous);
      throw error;
    } finally {
      button?.removeAttribute("aria-disabled");
      this.animating = false;
    }
  }
}
