import test from "node:test";
import assert from "node:assert/strict";

import { patchGenreSuggestionsResponse } from "./item-genre-suggestions-cache.js";

function makeResponse() {
  return {
    genre_counts: [
      { genre: "security", label: "security", count: 4 },
      { genre: "ai", label: "ai", count: 2 },
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
    beforeEffectiveGenre: "security",
    afterEffectiveGenre: "security",
  });

  assert.deepEqual(
    patched.genre_counts,
    [
      { genre: "security", label: "security", count: 4 },
      { genre: "ai", label: "ai", count: 2 },
    ]
  );
});

test("patchGenreSuggestionsResponse updates counts only when effective genre changes", () => {
  const patched = patchGenreSuggestionsResponse(makeResponse(), {
    beforeEffectiveGenre: "security",
    afterEffectiveGenre: "ai",
  });

  assert.deepEqual(
    patched.genre_counts,
    [
      { genre: "ai", label: "ai", count: 3 },
      { genre: "security", label: "security", count: 3 },
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
      { genre: "security", label: "security", count: 4 },
      { genre: "ai", label: "ai", count: 2 },
      { genre: "robotics", label: "robotics", count: 1 },
    ]
  );
});

test("patchGenreSuggestionsResponse resolves untagged fallback labels to uncategorized", () => {
  const patched = patchGenreSuggestionsResponse(
    {
      ...makeResponse(),
      genre_counts: [{ genre: "", label: "untagged", count: 2 }],
    },
    {
      beforeEffectiveGenre: "uncategorized",
      afterEffectiveGenre: "uncategorized",
    }
  );

  assert.deepEqual(patched.genre_counts, [{ genre: "", label: "untagged", count: 2 }]);
});
