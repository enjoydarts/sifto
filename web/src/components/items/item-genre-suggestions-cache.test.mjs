import test from "node:test";
import assert from "node:assert/strict";

import { patchGenreSuggestionsResponse } from "./item-genre-suggestions-cache.js";

function makeResponse() {
  return {
    genre_counts: [
      { genre: "Security", label: "Security", count: 4 },
      { genre: "AI", label: "AI", count: 2 },
    ],
    items: [],
    page: 1,
    page_size: 1,
    total: 0,
    has_next: false,
    sort: "newest",
  };
}

test("patchGenreSuggestionsResponse keeps counts stable when effective genre does not change", () => {
  const patched = patchGenreSuggestionsResponse(makeResponse(), {
    beforeEffectiveGenre: "Security",
    afterEffectiveGenre: "Security",
  });

  assert.deepEqual(
    patched.genre_counts,
    [
      { genre: "Security", label: "Security", count: 4 },
      { genre: "AI", label: "AI", count: 2 },
    ]
  );
});

test("patchGenreSuggestionsResponse updates counts only when effective genre changes", () => {
  const patched = patchGenreSuggestionsResponse(makeResponse(), {
    beforeEffectiveGenre: "Security",
    afterEffectiveGenre: "AI",
  });

  assert.deepEqual(
    patched.genre_counts,
    [
      { genre: "AI", label: "AI", count: 3 },
      { genre: "Security", label: "Security", count: 3 },
    ]
  );
});

test("patchGenreSuggestionsResponse adds the resulting effective genre when missing", () => {
  const patched = patchGenreSuggestionsResponse(makeResponse(), {
    beforeEffectiveGenre: "",
    afterEffectiveGenre: "Robotics",
  });

  assert.deepEqual(
    patched.genre_counts,
    [
      { genre: "Security", label: "Security", count: 4 },
      { genre: "AI", label: "AI", count: 2 },
      { genre: "Robotics", label: "Robotics", count: 1 },
    ]
  );
});
