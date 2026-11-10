import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

import { buildOptionsForPurpose } from "./llm-model-options.ts";
import { enDict } from "../../../i18n/dictionaries/en.ts";
import { jaDict } from "../../../i18n/dictionaries/ja.ts";

const catalog = JSON.parse(readFileSync(new URL("../../../../../shared/llm_catalog.json", import.meta.url), "utf8"));

for (const [locale, dictionary] of [["en", enDict], ["ja", jaDict]]) {
  test(`Fable 5.2 is selectable with standard pricing and localized guidance in ${locale}`, () => {
    const t = (key, fallback) => dictionary[key] ?? fallback ?? key;
    for (const purpose of ["facts", "summary", "digest_cluster_draft", "digest", "ask", "source_suggestion"]) {
      const option = buildOptionsForPurpose(catalog, purpose, undefined, t).find((item) => item.value === "claude-fable-5-2");
      assert.ok(option);
      assert.equal(option.provider, "Anthropic");
      assert.equal(option.disabled, false);
      assert.equal(option.note, "cached in $0.2 / in $8 / out $40 / 1M tok");
    }
    assert.match(dictionary["settings.modelGuide.comments.claude-fable-5-2"], /Fable 5\.2/);
  });
}
