import test from "node:test";
import assert from "node:assert/strict";
import { createAppActions } from "../src/actions/app-actions.js";
import { renderModal } from "../src/components/modals.js";

function harness(apiOverrides = {}) {
  const state = { modal: null, launchPending: false, presets: [] };
  const store = {
    getState: () => state,
    setState(update) { Object.assign(state, update(state)); }
  };
  const errors = [];
  const api = {
    listSessions: async () => [],
    continueSession: async () => {},
    deleteSession: async () => [],
    ...apiOverrides
  };
  const feedback = { showError(title, error) { errors.push([title, error]); } };
  const actions = createAppActions({ root: {}, api, store, feedback, applyTheme() {} });
  return { state, actions, errors };
}

const session = { id: "session-id", title: "Fix <session>", workingDir: "C:\\work", model: "provider/model", updatedAt: "2026-08-05T04:00:19Z", sizeBytes: 2048 };

test("session manager loads sessions and renders escaped actions", async () => {
  const run = harness({ listSessions: async () => [session] });
  await run.actions.openSessions();
  assert.equal(run.state.modal.kind, "session-manager");
  const markup = renderModal({ modal: run.state.modal });
  assert.match(markup, /data-continue-session="session-id"/);
  assert.match(markup, /data-delete-session="session-id"/);
  assert.match(markup, /Fix &lt;session&gt;/);
  assert.doesNotMatch(markup, /Fix <session>/);
});

test("continuing a session locks launch state then closes modal", async () => {
  let release;
  const pending = new Promise((resolve) => { release = resolve; });
  const run = harness({ continueSession: async () => pending });
  run.state.modal = { kind: "session-manager", payload: { sessions: [session] } };
  const continuing = run.actions.continueSession(session.id);
  assert.equal(run.state.launchPending, true);
  assert.equal(run.state.modal.payload.pendingSessionId, session.id);
  release();
  await continuing;
  assert.equal(run.state.launchPending, false);
  assert.equal(run.state.modal, null);
});

test("deleting a session requires confirmation and adopts refreshed list", async () => {
  const remaining = [{ ...session, id: "other" }];
  const run = harness({ deleteSession: async (id) => { assert.equal(id, session.id); return remaining; } });
  run.state.modal = { kind: "session-manager", payload: { sessions: [session, ...remaining] } };
  run.actions.requestDeleteSession(session);
  assert.equal(run.state.modal.kind, "confirm-delete-session");
  await run.actions.confirmDeleteSession();
  assert.equal(run.state.modal.kind, "session-manager");
  assert.deepEqual(run.state.modal.payload.sessions, remaining);
});
