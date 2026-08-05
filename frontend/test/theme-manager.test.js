import test from "node:test";
import assert from "node:assert/strict";
import { ThemeManager, normalizeThemeMode, resolveTheme } from "../src/theme/theme-manager.js";
import { normalizeThemeState, renderThemeSwitcher } from "../src/components/theme-switcher.js";

test("theme modes normalize and system resolves against OS preference", () => {
  assert.equal(normalizeThemeMode("light"), "light");
  assert.equal(normalizeThemeMode("dark"), "dark");
  assert.equal(normalizeThemeMode("system"), "system");
  assert.equal(normalizeThemeMode("unknown"), "system");
  assert.equal(resolveTheme("system", true), "dark");
  assert.equal(resolveTheme("system", false), "light");
});


test("theme switcher exposes one three-state control", () => {
  assert.equal(normalizeThemeState("unknown"), "system");
  const markup = renderThemeSwitcher("dark");
  assert.match(markup, /data-theme-state="dark"/);
  assert.equal((markup.match(/data-set-theme=/g) ?? []).length, 3);
  assert.equal((markup.match(/aria-pressed="true"/g) ?? []).length, 1);
  assert.doesNotMatch(markup, /theme-switcher__liquid|glass-orb/);
});

function themeHarness(updateSettings) {
  const attributes = new Map();
  const classes = new Set();
  const root = { dataset: {}, style: {} };
  globalThis.window = {};
  globalThis.document = { documentElement: root, body: { dataset: {} } };
  const state = { settings: { theme: "light", ompCommand: "omp" }, presets: [], modal: null };
  const store = { getState: () => state, setState(update) { Object.assign(state, update(state)); } };
  const api = { applyWindowTheme() {}, updateSettings };
  const button = {
    getBoundingClientRect: () => ({ left: 740, top: 10, width: 34, height: 34 }),
    setAttribute: (name, value) => attributes.set(name, value),
    removeAttribute: (name) => attributes.delete(name),
    classList: { add: (name) => classes.add(name), remove: (name) => classes.delete(name) }
  };
  const manager = new ThemeManager({ api, store, media: { matches: false, addEventListener() {} } });
  manager.initialise(state.settings);
  return { manager, state, button, attributes, classes };
}

test("theme toggle persists once and blocks re-entry while saving", async () => {
  let release;
  const pending = new Promise((resolve) => { release = resolve; });
  let saves = 0;
  const harness = themeHarness(async (settings) => { saves += 1; await pending; return { settings }; });
  const first = harness.manager.toggle(harness.button);
  assert.equal(await harness.manager.toggle(harness.button), false);
  assert.equal(harness.manager.resolved, "dark");
  assert.equal(harness.attributes.has("aria-disabled"), false);
  release();
  assert.equal(await first, true);
  assert.equal(saves, 1);
  assert.equal(harness.state.settings.theme, "dark");
  assert.equal(harness.classes.size, 0);
});

test("failed theme persistence restores mode and visual theme", async () => {
  const harness = themeHarness(async () => { throw new Error("save failed"); });
  await assert.rejects(harness.manager.toggle(harness.button), /save failed/);
  assert.equal(harness.manager.mode, "light");
  assert.equal(harness.manager.resolved, "light");
  assert.equal(document.documentElement.dataset.theme, "light");
  assert.equal(harness.manager.animating, false);
});
