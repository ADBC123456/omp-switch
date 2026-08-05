import test from "node:test";
import assert from "node:assert/strict";
import { discoveryCounts, filterNewModels, groupDiscovery, toggleFilteredSelection } from "../src/domain/discovery.js";
import { buildRoleUpdates, parseRoleValue } from "../src/domain/model-roles.js";

test("discovery grouping and filtered toggle preserve upstream order", () => {
  const existing = [{ id: "old" }, { id: "missing" }];
  const fetched = [{ id: "new-a", name: "Alpha" }, { id: "old" }, { id: "new-b", name: "Beta" }];
  const groups = groupDiscovery(existing, fetched);
  assert.deepEqual(groups.added.map((m) => m.id), ["new-a", "new-b"]);
  assert.deepEqual(groups.existing.map((m) => m.id), ["old"]);
  assert.deepEqual(groups.missing.map((m) => m.id), ["missing"]);
  assert.deepEqual(filterNewModels(groups.added, "beta").map((m) => m.id), ["new-b"]);
  const selected = toggleFilteredSelection(groups.added, "beta", new Set(["new-a", "new-b"]));
  assert.deepEqual([...selected], ["new-a"]);
  assert.deepEqual(discoveryCounts(groups.added, "", selected), { selected: 1, filtered: 2, total: 2 });
});

test("role parser preserves custom values until changed", () => {
  const providers = [{ id: "gateway", models: [{ id: "team/model" }] }];
  assert.deepEqual(parseRoleValue("gateway/team/model:high", providers), { kind: "model", model: "gateway/team/model", thinking: "high" });
  assert.deepEqual(parseRoleValue("external/custom", providers), { kind: "custom", raw: "external/custom", model: "", thinking: "" });
  assert.deepEqual(buildRoleUpdates([{ role: "task", dirty: false }, { role: "default", model: "gateway/team/model", thinking: "", dirty: true }]), [{ role: "default", selector: "gateway/team/model", clear: false }]);
});
