import { existsSync, readdirSync, rmSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const nextDir = resolve(__dirname, "../.next");
const preservedEntries = new Set(["cache", "dev"]);

if (!existsSync(nextDir)) {
  process.exit(0);
}

for (const entry of readdirSync(nextDir)) {
  if (preservedEntries.has(entry)) {
    continue;
  }
  rmSync(resolve(nextDir, entry), { force: true, recursive: true });
}
