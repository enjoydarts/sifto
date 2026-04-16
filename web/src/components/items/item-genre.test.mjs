import test from "node:test";
import assert from "node:assert/strict";

import {
  displayGenreLabel,
  genreValueFromCountEntry,
  normalizeStoredGenreValue,
  orderGenreCountEntries,
} from "./item-genre.ts";

const LABELS = {
  "items.genre.label.ai": "AI",
  "items.genre.label.agent": "Agents",
  "items.genre.label.security": "Security",
  "items.genre.label.uncategorized": "Uncategorized",
  "items.genre.label.other": "Other",
};

function t(key) {
  return LABELS[key] ?? key;
}

test("normalizeStoredGenreValue canonicalizes fixed genre keys and uncategorized aliases", () => {
  assert.equal(normalizeStoredGenreValue(" Security "), "security");
  assert.equal(normalizeStoredGenreValue("agent"), "ai");
  assert.equal(normalizeStoredGenreValue("untagged"), "uncategorized");
  assert.equal(normalizeStoredGenreValue("uncategorized"), "uncategorized");
});

test("displayGenreLabel localizes fixed taxonomy keys", () => {
  assert.equal(displayGenreLabel("security", t), "Security");
  assert.equal(displayGenreLabel("uncategorized", t), "Uncategorized");
});

test("displayGenreLabel uses freeform other label only when requested", () => {
  assert.equal(displayGenreLabel("other", t), "Other");
  assert.equal(displayGenreLabel("other", t, { otherLabel: "Edge AI chips" }), "Edge AI chips");
});

test("genreValueFromCountEntry canonicalizes alias labels", () => {
  assert.equal(genreValueFromCountEntry({ genre: "", label: "untagged", count: 2 }), "uncategorized");
});

test("orderGenreCountEntries follows taxonomy order for known keys", () => {
  const ordered = orderGenreCountEntries([
    { genre: "other", label: "other", count: 1 },
    { genre: "security", label: "security", count: 4 },
    { genre: "ai", label: "ai", count: 3 },
    { genre: "uncategorized", label: "uncategorized", count: 2 },
  ]);

  assert.deepEqual(ordered.map((entry) => entry.genre), [
    "ai",
    "security",
    "uncategorized",
    "other",
  ]);
});
