import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { buildOptionsForPurpose } from "./llm-model-options.ts";
import { enDict } from "../../../i18n/dictionaries/en.ts";
import { jaDict } from "../../../i18n/dictionaries/ja.ts";

const catalog = JSON.parse(readFileSync(new URL("../../../../../shared/llm_catalog.json", import.meta.url), "utf8"));
for (const [locale, dictionary] of [["en", enDict], ["ja", jaDict]]) {
  test(`Gemini 4 models are selectable and localized in ${locale}`, () => {
    const t = (key, fallback) => dictionary[key] ?? fallback ?? key;
    for (const purpose of ["facts", "summary", "digest_cluster_draft", "digest", "ask", "source_suggestion"]) {
      const options = buildOptionsForPurpose(catalog, purpose, undefined, t);
      for (const model of ["gemini-4-carbon", "gemini-4-argon", "gemini-4-hydrogen"]) {
        const option = options.find((item) => item.value === model);
        assert.ok(option, model);
        assert.equal(option.disabled, false);
        assert.match(dictionary[`settings.modelGuide.comments.${model}`], /Gemini 4/);
        assert.match(option.note, model.endsWith("hydrogen") ? /in \$1\.5 \/ out \$7\.5/ : /in \$2 \/ out \$10/);
      }
    }
  });
}
