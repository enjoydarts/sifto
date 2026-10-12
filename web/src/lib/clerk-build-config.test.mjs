import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";
import ts from "typescript";

function loadModule(relativePath, env, serverCalls) {
  const source = readFileSync(new URL(relativePath, import.meta.url), "utf8");
  const { outputText } = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX },
  });
  const exports = {};
  const jsx = (type, props) => ({ type, props });
  const require = (name) => {
    if (name === "react/jsx-runtime") return { jsx, jsxs: jsx };
    if (name === "react") return { Suspense: "Suspense" };
    if (name === "@/components/providers") return { Providers: "Providers" };
    if (name === "@clerk/nextjs/server") return {
      auth: async () => { serverCalls.push("auth"); return { userId: null }; },
      currentUser: async () => { serverCalls.push("currentUser"); return null; },
    };
    if (name === "@/lib/verified-primary-email") return { verifiedPrimaryEmail: () => null };
    if (name === "next/script" || name === "./login-form") return { default: name };
    if (name === "./globals.css") return {};
    throw new Error(`Unexpected import: ${name}`);
  };
  vm.runInNewContext(outputText, { exports, require, process: { env } });
  return exports;
}

function findElement(element, type) {
  if (!element || typeof element !== "object") return null;
  if (element.type === type) return element;
  const children = element.props?.children;
  for (const child of Array.isArray(children) ? children : [children]) {
    const found = findElement(child, type);
    if (found) return found;
  }
  return null;
}

test("public Clerk config enables provider and sign-in UI without granting server authentication", async () => {
  for (const [publicKey, secretKey, enabled] of [
    ["public-config-fixture", undefined, true],
    ["public-config-fixture", "server-secret-fixture", true],
    [undefined, undefined, false],
    [undefined, "server-secret-fixture", false],
  ]) {
    const env = {};
    if (publicKey) env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY = publicKey;
    if (secretKey) env.CLERK_SECRET_KEY = secretKey;
    const serverCalls = [];
    const layout = loadModule("../app/layout.tsx", env, serverCalls).default({ children: "page" });
    const provider = findElement(layout, "Providers");
    assert.equal(provider.props.clerkEnabled, enabled);
    assert.equal(provider.props.clerkPublishableKey, publicKey);
    assert.equal(Object.hasOwn(provider.props, "clerkSecretKey"), false);
    const login = loadModule("../app/(auth)/login/page.tsx", env, serverCalls).default();
    assert.equal(findElement(login, "./login-form").props.showClerk, enabled);
    const server = loadModule("./server-auth.ts", env, serverCalls);
    assert.equal(await server.getServerAuthUser(), null);
    assert.deepEqual(serverCalls, publicKey && secretKey ? ["auth"] : []);
  }
});
