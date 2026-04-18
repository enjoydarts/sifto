import test from "node:test";
import assert from "node:assert/strict";

import { formatModelDisplayName, providerLabel } from "./model-display.ts";

test("formatModelDisplayName formats Xiaomi MiMo models with branded casing", () => {
  assert.equal(formatModelDisplayName("mimo-v2-pro"), "MiMo-V2-Pro");
  assert.equal(formatModelDisplayName("mimo-v2-omni"), "MiMo-V2-Omni");
});

test("providerLabel formats Xiaomi MiMo TokenPlan provider label consistently", () => {
  assert.equal(providerLabel("xiaomi_mimo_token_plan"), "Xiaomi MiMo (TokenPlan)");
});
