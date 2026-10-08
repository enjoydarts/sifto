import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

import { buildOptionsForPurpose } from "./llm-model-options.ts";
import { enDict } from "../../../i18n/dictionaries/en.ts";
import { jaDict } from "../../../i18n/dictionaries/ja.ts";

const catalog = JSON.parse(readFileSync(new URL("../../../../../shared/llm_catalog.json", import.meta.url), "utf8"));

for (const [locale, dictionary] of [["en", enDict], ["ja", jaDict]]) {
  test(`Haiku 5.5 model option shows both prompt-length prices in ${locale}`, () => {
    const t = (key, fallback) => dictionary[key] ?? fallback ?? key;
    const options = buildOptionsForPurpose(catalog, "facts", undefined, t);
    const option = options.find((item) => item.value === "claude-haiku-5-5");
    assert.ok(option);
    assert.equal(option.provider, "Anthropic");
    assert.match(option.note, /\$0\.1/);
    assert.match(option.note, /\$0\.5/);
    assert.match(option.note, /100,000/);
    assert.match(option.note, /\$2\.5/);
    assert.match(option.note, /\$0\.05/);
    assert.doesNotMatch(option.note, /settings\./);
  });
}
