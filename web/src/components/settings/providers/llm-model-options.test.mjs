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
