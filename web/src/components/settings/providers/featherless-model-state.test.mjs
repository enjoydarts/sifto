import test from "node:test";
import assert from "node:assert/strict";

import {
  getFeatherlessModelState,
  isFeatherlessModelSelectable,
  featherlessModelBadgeLabel,
} from "./featherless-model-state.ts";

test("getFeatherlessModelState treats not-on-plan models as unavailable and disabled", () => {
  const state = getFeatherlessModelState({
    provider: "featherless",
    availability: "unavailable",
    reason: "not_on_plan",
  });

  assert.equal(state.kind, "unavailable");
  assert.equal(state.selectable, false);
  assert.equal(featherlessModelBadgeLabel(state.kind), "Unavailable");
  assert.equal(isFeatherlessModelSelectable({ provider: "featherless", availability: "unavailable", reason: "not_on_plan" }), false);
});

test("getFeatherlessModelState keeps gated models selectable", () => {
  const state = getFeatherlessModelState({
    provider: "featherless",
    availability: "available",
    gated: true,
    available_on_current_plan: true,
  });

  assert.equal(state.kind, "gated");
  assert.equal(state.selectable, true);
  assert.equal(featherlessModelBadgeLabel(state.kind), "Gated");
});

test("getFeatherlessModelState treats removed models as removed", () => {
  const state = getFeatherlessModelState({
    provider: "featherless",
    availability: "removed",
    reason: "removed",
  });

  assert.equal(state.kind, "removed");
  assert.equal(state.selectable, false);
  assert.equal(featherlessModelBadgeLabel(state.kind), "Removed");
});

test("getFeatherlessModelState keeps available models selectable", () => {
  const state = getFeatherlessModelState({
    provider: "featherless",
    availability: "available",
  });

  assert.equal(state.kind, "available");
  assert.equal(state.selectable, true);
  assert.equal(featherlessModelBadgeLabel(state.kind), null);
});
