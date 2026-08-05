export const MANAGED_ROLES = ["default", "smol", "slow", "plan", "commit", "vision", "designer", "task", "advisor", "tiny"];
export const THINKING_LEVELS = ["off", "minimal", "low", "medium", "high", "xhigh", "max", "auto"];

function configuredSelectors(providers) {
  const selectors = new Set();
  for (const provider of providers ?? []) {
    for (const model of provider.models ?? []) selectors.add(`${provider.id}/${model.id}`);
  }
  return selectors;
}

export function parseRoleValue(raw, providers) {
  if (!raw) return { kind: "unset", model: "", thinking: "" };
  const selectors = configuredSelectors(providers);
  if (selectors.has(raw)) return { kind: "model", model: raw, thinking: "" };
  const separator = raw.lastIndexOf(":");
  if (separator > 0 && THINKING_LEVELS.includes(raw.slice(separator + 1)) && selectors.has(raw.slice(0, separator))) {
    return { kind: "model", model: raw.slice(0, separator), thinking: raw.slice(separator + 1) };
  }
  return { kind: "custom", raw, model: "", thinking: "" };
}

export function buildRoleUpdates(rows) {
  return rows.filter((row) => row.dirty).map((row) => {
    if (!row.model) return { role: row.role, selector: "", clear: true };
    return { role: row.role, selector: row.model + (row.thinking ? `:${row.thinking}` : ""), clear: false };
  });
}
