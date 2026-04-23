import test from "node:test";
import assert from "node:assert/strict";

import { buildOptionsForChatModel, buildOptionsForPurpose } from "./llm-model-options.ts";

const t = (key, fallback) => fallback ?? key;

test("buildOptionsForPurpose keeps featherless unavailable and removed models visible but disabled", () => {
  const catalog = {
    chat_models: [
      {
        id: "featherless::available-model",
        provider: "featherless",
        available_purposes: ["summary"],
        pricing: null,
        availability: "available",
      },
      {
        id: "featherless::gated-model",
        provider: "featherless",
        available_purposes: ["summary"],
        pricing: null,
        availability: "unavailable",
        raw_availability: "not_on_plan",
        reason: "not_on_plan",
        available_on_current_plan: false,
      },
      {
        id: "featherless::removed-model",
        provider: "featherless",
        available_purposes: ["summary"],
        pricing: null,
        availability: "removed",
        raw_availability: "removed",
        reason: "removed",
      },
    ],
    embedding_models: [],
  };

  const options = buildOptionsForPurpose(catalog, "summary", undefined, t);
  assert.equal(options.length, 3);
  assert.deepEqual(
    options.map((option) => ({
      value: option.value,
      disabled: option.disabled ?? false,
      badge: option.badge ?? null,
    })),
    [
      { value: "featherless::available-model", disabled: false, badge: null },
      { value: "featherless::gated-model", disabled: true, badge: "Unavailable" },
      { value: "featherless::removed-model", disabled: true, badge: "Removed" },
    ],
  );
});

test("buildOptionsForChatModel marks featherless unavailable and removed entries", () => {
  const catalog = {
    chat_models: [
      {
        id: "featherless::gated-model",
        provider: "featherless",
        available_purposes: ["summary"],
        pricing: null,
        availability: "unavailable",
        raw_availability: "not_on_plan",
        reason: "not_on_plan",
      },
      {
        id: "featherless::removed-model",
        provider: "featherless",
        available_purposes: ["summary"],
        pricing: null,
        availability: "removed",
        raw_availability: "removed",
        reason: "removed",
      },
    ],
    embedding_models: [],
  };

  const options = buildOptionsForChatModel(catalog, undefined, t);
  assert.equal(options[0].disabled, true);
  assert.equal(options[0].badge, "Unavailable");
  assert.equal(options[1].disabled, true);
  assert.equal(options[1].badge, "Removed");
});

test("buildOptionsForPurpose formats DeepInfra entries with provider label and pricing", () => {
  const catalog = {
    chat_models: [
      {
        id: "deepinfra::meta-llama/Llama-4-Scout",
        provider: "deepinfra",
        available_purposes: ["summary"],
        pricing: {
          input_per_mtok_usd: 0.12,
          output_per_mtok_usd: 0.34,
          cache_read_per_mtok_usd: 0,
        },
      },
    ],
    embedding_models: [],
  };

  const [option] = buildOptionsForPurpose(catalog, "summary", undefined, t);
  assert.equal(option.provider, "DeepInfra");
  assert.equal(option.label, "meta-llama/Llama-4-Scout");
  assert.equal(option.selectedLabel, "DeepInfra / meta-llama/Llama-4-Scout");
  assert.equal(option.note, "in $0.12 / out $0.34 / 1M tok");
});

test("buildOptionsForPurpose keeps missing selected DeepInfra model visible with provider fallback", () => {
  const options = buildOptionsForPurpose(
    { chat_models: [], embedding_models: [] },
    "summary",
    "deepinfra::Qwen/Qwen3-32B",
    t,
  );

  assert.equal(options.length, 1);
  assert.equal(options[0].provider, "DeepInfra");
  assert.equal(options[0].selectedLabel, "DeepInfra / Qwen/Qwen3-32B");
});
