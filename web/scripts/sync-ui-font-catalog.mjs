import { copyFileSync, existsSync, mkdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const destination = resolve(__dirname, "../src/generated_ui_font_catalog.json");
const sourceCandidates = [
  resolve(__dirname, "../../shared/ui_font_catalog.json"),
  resolve(__dirname, "../shared/ui_font_catalog.json"),
  "/app/shared/ui_font_catalog.json",
  "/shared/ui_font_catalog.json",
];
const source = sourceCandidates.find((candidate) => existsSync(candidate));

if (!source) {
  if (!existsSync(destination)) {
    console.error("ui_font_catalog.json source not found");
    process.exit(1);
  }
  process.exit(0);
}

mkdirSync(dirname(destination), { recursive: true });
copyFileSync(source, destination);
