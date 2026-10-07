import test from "node:test";
import assert from "node:assert/strict";
import { verifiedPrimaryEmail } from "./verified-primary-email.ts";

test("identity linking uses only the verified primary address", () => {
  const other = { id: "other", emailAddress: "victim@example.com", verification: { status: "verified" } };
  const primary = { id: "primary", emailAddress: "reader@example.com", verification: { status: "unverified" } };
  assert.equal(verifiedPrimaryEmail({ primaryEmailAddressId: "primary", emailAddresses: [other, primary] }), null);
  assert.equal(verifiedPrimaryEmail({ primaryEmailAddressId: "missing", emailAddresses: [other] }), null);
  assert.equal(verifiedPrimaryEmail({ primaryEmailAddressId: "primary", emailAddresses: [{ ...primary, verification: { status: "verified" } }] }), "reader@example.com");
});
