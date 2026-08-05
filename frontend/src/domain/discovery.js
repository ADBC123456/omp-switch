function text(model) {
  return `${model.id ?? ""} ${model.name ?? ""}`.toLowerCase();
}

export function groupDiscovery(existing, fetched) {
  const existingById = new Map((existing ?? []).map((model) => [model.id, model]));
  const fetchedIds = new Set((fetched ?? []).map((model) => model.id));
  return {
    added: (fetched ?? []).filter((model) => !existingById.has(model.id)),
    existing: (fetched ?? []).filter((model) => existingById.has(model.id)),
    missing: (existing ?? []).filter((model) => !fetchedIds.has(model.id))
  };
}

export function filterNewModels(models, query) {
  const normalized = String(query ?? "").trim().toLowerCase();
  return normalized ? models.filter((model) => text(model).includes(normalized)) : [...models];
}

export function toggleFilteredSelection(models, query, selected) {
  const visible = new Set(filterNewModels(models, query).map((model) => model.id));
  const next = new Set(selected);
  const shouldSelect = [...visible].some((id) => !next.has(id));
  for (const id of visible) shouldSelect ? next.add(id) : next.delete(id);
  return next;
}

export function discoveryCounts(models, query, selected) {
  const filtered = filterNewModels(models, query);
  return { selected: filtered.filter((model) => selected.has(model.id)).length, filtered: filtered.length, total: models.length };
}
