import test from "node:test";
import assert from "node:assert/strict";

import { swallowPlaybackSessionError } from "./playback-session-guard.ts";

test("swallowPlaybackSessionError returns result on success", async () => {
  const result = await swallowPlaybackSessionError(async () => "ok");

  assert.equal(result, "ok");
});

test("swallowPlaybackSessionError swallows playback tracking failures", async () => {
  const errors = [];

  const result = await swallowPlaybackSessionError(
    async () => {
      throw new Error("400: invalid request");
    },
    (error) => errors.push(error instanceof Error ? error.message : String(error)),
  );

  assert.equal(result, null);
  assert.deepEqual(errors, ["400: invalid request"]);
});
