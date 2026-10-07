import assert from "node:assert/strict";
import test from "node:test";
import { createInstallationState, verifyInstallationState, authorizedInstallationRepositories } from "./github-install-flow.ts";

test("installation flow requires signed state, matching cookie and same user", () => {
  const secret = "a".repeat(32);
  const state = createInstallationState("user1", secret, 42);
  assert.equal(verifyInstallationState(state, state, "user1", secret)?.installationId, 42);
  for (const [s, cookie, user, key] of [[state, "", "user1", secret], [state, state, "user2", secret], [state + "x", state + "x", "user1", secret], [state, state, "user1", "b".repeat(32)]]) {
    assert.equal(verifyInstallationState(s, cookie, user, key), null);
  }
  const now = Date.now;
  Date.now = () => now() + 601_000;
  try { assert.equal(verifyInstallationState(state, state, "user1", secret), null); }
  finally { Date.now = now; }
});

test("user token must authorize selected installation and writable repos", async () => {
  const originalFetch = global.fetch;
  global.fetch = async (url) => url.includes("access_token")
    ? Response.json({ access_token: "user-token" })
    : Response.json({ total_count: 2, repositories: [{ full_name: "me/write", permissions: { push: true } }, { full_name: "other/readonly", permissions: { push: false } }] });
  try {
    assert.deepEqual(await authorizedInstallationRepositories("code", "https://app/callback", 42), ["me/write"]);
    global.fetch = async (url) => url.includes("access_token") ? Response.json({ access_token: "token" }) : new Response(null, { status: 403 });
    await assert.rejects(authorizedInstallationRepositories("code", "https://app/callback", 999));
  } finally { global.fetch = originalFetch; }
});
