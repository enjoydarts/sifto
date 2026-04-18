import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";

test("Xiaomi MiMo TokenPlan provider id has localized labels in both dictionaries", () => {
  const ja = fs.readFileSync(new URL("./ja.ts", import.meta.url), "utf8");
  const en = fs.readFileSync(new URL("./en.ts", import.meta.url), "utf8");

  assert.match(ja, /"settings\.modelGuide\.provider\.xiaomi_mimo_token_plan": "Xiaomi MiMo \(TokenPlan\)"/);
  assert.match(en, /"settings\.modelGuide\.provider\.xiaomi_mimo_token_plan": "Xiaomi MiMo \(TokenPlan\)"/);
});
