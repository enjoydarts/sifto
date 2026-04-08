import uiFontCatalogData from "../generated_ui_font_catalog.json";

export type UIFontCategory = "sans" | "serif" | "display";

export type UIFontCatalogEntry = {
  key: string;
  label: string;
  family: string;
  category: UIFontCategory;
  selectable_for_sans: boolean;
  selectable_for_serif: boolean;
  preview_ui: string;
  preview_body: string;
};

export type UIFontCatalog = {
  catalog_name: string;
  source: string;
  source_reference: string;
  fonts: UIFontCatalogEntry[];
};

export type UIFontSelection = {
  sansKey: string;
  serifKey: string;
};

export const DEFAULT_UI_FONT_SANS_KEY = "sawarabi-gothic";
export const DEFAULT_UI_FONT_SERIF_KEY = "sawarabi-mincho";
const APP_FONT_LINK_ID = "sifto-ui-fonts-link";
const PREVIEW_FONT_LINK_ID = "sifto-ui-font-preview-link";

const UI_FONT_CATALOG: UIFontCatalog = uiFontCatalogData as UIFontCatalog;

function normalizeFontKey(key: string, fallback: string): string {
  const normalized = key.trim().toLowerCase();
  return normalized || fallback;
}

export function getUIFontCatalog(): UIFontCatalog {
  return UI_FONT_CATALOG;
}

export function getUIFontByKey(key: string): UIFontCatalogEntry | null {
  const normalized = key.trim().toLowerCase();
  return UI_FONT_CATALOG.fonts.find((item) => item.key === normalized) ?? null;
}

export function getSelectableSansFonts(): UIFontCatalogEntry[] {
  return UI_FONT_CATALOG.fonts.filter((item) => item.selectable_for_sans);
}

export function getSelectableSerifFonts(): UIFontCatalogEntry[] {
  return UI_FONT_CATALOG.fonts.filter((item) => item.selectable_for_serif);
}

function cssFamilyList(family: string, kind: "sans" | "serif"): string {
  if (kind === "sans") {
    return `"${family}", "Noto Sans JP", "Hiragino Kaku Gothic ProN", "Yu Gothic", "Meiryo", ui-sans-serif, system-ui, sans-serif`;
  }
  return `"${family}", "Noto Serif JP", "Hiragino Mincho ProN", "Yu Mincho", "MS PMincho", ui-serif, Georgia, serif`;
}

function buildGoogleFontsHref(families: string[]): string {
  const uniqueFamilies = Array.from(new Set(families.map((item) => item.trim()).filter(Boolean)));
  const params = uniqueFamilies.map((family) => `family=${encodeURIComponent(family).replace(/%20/g, "+")}`);
  return `https://fonts.googleapis.com/css2?${params.join("&")}&display=swap`;
}

function ensureFontStylesheet(id: string, families: string[]) {
  if (typeof document === "undefined") return;
  const href = buildGoogleFontsHref(families);
  let link = document.getElementById(id) as HTMLLinkElement | null;
  if (!link) {
    link = document.createElement("link");
    link.id = id;
    link.rel = "stylesheet";
    document.head.appendChild(link);
  }
  if (link.href !== href) {
    link.href = href;
  }
}

export function resolveUIFontSelection(selection?: Partial<UIFontSelection> | null): UIFontSelection {
  const sansEntry = getUIFontByKey(normalizeFontKey(selection?.sansKey ?? "", DEFAULT_UI_FONT_SANS_KEY));
  const serifEntry = getUIFontByKey(normalizeFontKey(selection?.serifKey ?? "", DEFAULT_UI_FONT_SERIF_KEY));
  return {
    sansKey: sansEntry?.selectable_for_sans ? sansEntry.key : DEFAULT_UI_FONT_SANS_KEY,
    serifKey: serifEntry?.selectable_for_serif ? serifEntry.key : DEFAULT_UI_FONT_SERIF_KEY,
  };
}

export function applyUIFontSelectionToDocument(selection?: Partial<UIFontSelection> | null) {
  if (typeof document === "undefined") return;
  const resolved = resolveUIFontSelection(selection);
  const sans = getUIFontByKey(resolved.sansKey);
  const serif = getUIFontByKey(resolved.serifKey);
  if (!sans || !serif) return;
  ensureFontStylesheet(APP_FONT_LINK_ID, [sans.family, serif.family]);
  document.documentElement.style.setProperty("--font-sans-jp", cssFamilyList(sans.family, "sans"));
  document.documentElement.style.setProperty("--font-serif-jp", cssFamilyList(serif.family, "serif"));
  document.documentElement.dataset.uiFontSans = sans.key;
  document.documentElement.dataset.uiFontSerif = serif.key;
}

export function ensureUIFontPreviewLoaded(key: string) {
  const entry = getUIFontByKey(key);
  if (!entry || typeof document === "undefined") return;
  ensureFontStylesheet(PREVIEW_FONT_LINK_ID, [entry.family]);
}

export function persistUIFontSelection(selection: UIFontSelection) {
  const resolved = resolveUIFontSelection(selection);
  applyUIFontSelectionToDocument(resolved);
}
