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

test("providerLabel formats Featherless consistently", () => {
  assert.equal(providerLabel("featherless"), "Featherless.ai");
});

test("providerLabel formats DeepInfra consistently", () => {
  assert.equal(providerLabel("deepinfra"), "DeepInfra");
});

test("formatModelDisplayName strips Featherless alias prefix", () => {
  assert.equal(
    formatModelDisplayName("featherless::Qwen/Qwen3.5-9B"),
    "Qwen/Qwen3.5-9B",
  );
});

test("formatModelDisplayName strips DeepInfra alias prefix", () => {
  assert.equal(
    formatModelDisplayName("deepinfra::meta-llama/Llama-4-Scout"),
    "meta-llama/Llama-4-Scout",
  );
});
