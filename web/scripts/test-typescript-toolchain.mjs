import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const ts = require("typescript");
assert.equal(typeof ts.createProgram, "function", "ESLint and Next.js need the compiler API");
assert.equal(ts.Extension.Cjs, ".cjs");

const nativeRoot = dirname(require.resolve("@typescript/native/package.json"));
const native = JSON.parse(readFileSync(join(nativeRoot, "package.json"), "utf8"));
assert.match(native.version, /^7\./);
const compiler = resolve(nativeRoot, native.bin.tsc);
const directory = mkdtempSync(join(tmpdir(), "sifto-typescript-"));
try {
  const input = join(directory, "example.ts");
  const run = (source) => {
    writeFileSync(input, source);
    const result = spawnSync(process.execPath, [compiler, "--noEmit", "--strict", input], {
      cwd: directory, encoding: "utf8", timeout: 30000,
    });
    assert.ifError(result.error);
    return result;
  };
  const valid = run("const value: string = 'Sifto';\n");
  assert.equal(valid.status, 0, valid.stdout + valid.stderr);
  const invalid = run("const value: string = 42;\n");
  assert.notEqual(invalid.status, 0, "The native compiler must reject type errors");
  assert.match(invalid.stdout + invalid.stderr, /TS2322/);
  console.log(`TypeScript API ${ts.version}; native compiler ${native.version}: passed`);
} finally {
  rmSync(directory, { recursive: true, force: true });
}
