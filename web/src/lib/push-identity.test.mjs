import assert from "node:assert/strict";
import test from "node:test";
import { pushIdentity } from "./push-identity.ts";

test("push identities match API targeting and require a shared secret", () => {
  assert.equal(pushIdentity(" User@Example.com ", "a".repeat(32)), "sifto_v2_4386dd05bbea24cdc3e89636ee221cbdfc6fe51d29b4a31348a291f6bab440cf");
  assert.throws(() => pushIdentity("user@example.com", "short"));
  assert.throws(() => pushIdentity(" ", "a".repeat(32)));
  assert.notEqual(pushIdentity("other@example.com", "a".repeat(32)), pushIdentity("user@example.com", "a".repeat(32)));
});
