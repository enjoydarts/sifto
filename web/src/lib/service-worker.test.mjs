import test from "node:test";
import assert from "node:assert/strict";
import vm from "node:vm";
import { readFileSync } from "node:fs";

function worker() {
  const handlers = {};
  const writes = [];
  const deletes = [];
  const cache = { match: async () => new Response("previous user's secret"), put: async (...args) => writes.push(args), keys: async () => [], addAll: async () => {} };
  vm.runInNewContext(readFileSync(new URL("../../public/sw.js", import.meta.url), "utf8"), {
    URL, Response,
    self: { location: { origin: "https://sifto.test" }, addEventListener: (name, handler) => handlers[name] = handler, registration: {}, clients: { claim: async () => {} } },
    caches: { open: async () => cache, match: async () => new Response("offline page"), keys: async () => ["sifto-api-v4", "sifto-pages-v4"], delete: async (key) => deletes.push(key) },
    fetch: async () => new Response("current user's response"),
  });
  return { handlers, writes, deletes };
}

test("service worker leaves authenticated API requests to the network", () => {
  const { handlers } = worker();
  let intercepted = false;
  handlers.fetch({ request: { method: "GET", url: "https://sifto.test/api/settings", mode: "cors" }, respondWith: () => intercepted = true });
  assert.equal(intercepted, false);
});

test("navigation responses containing private data are never cached", async () => {
  const { handlers, writes } = worker();
  let response;
  handlers.fetch({ request: { method: "GET", url: "https://sifto.test/items/private", mode: "navigate" }, respondWith: (promise) => response = promise });
  assert.equal(await (await response).text(), "current user's response");
  assert.equal(writes.length, 0);
});

test("activation deletes caches from earlier authenticated sessions", async () => {
  const { handlers, deletes } = worker();
  let done;
  handlers.activate({ waitUntil: (promise) => done = promise });
  await done;
  assert.deepEqual(deletes, ["sifto-api-v4", "sifto-pages-v4"]);
});
