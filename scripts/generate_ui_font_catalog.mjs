#!/usr/bin/env node

/**
 * Regenerates shared/ui_font_catalog.json from the Google Webfonts API.
 *
 * Usage:
 *   GOOGLE_FONTS_API_KEY=... node scripts/generate_ui_font_catalog.mjs
 */

const apiKey = process.env.GOOGLE_FONTS_API_KEY;

const JAPANESE_LABELS = {
  "sawarabi-gothic": "さわらびゴシック",
  "noto-sans-jp": "Noto Sans JP",
  "biz-udgothic": "BIZ UDゴシック",
  "biz-udpgothic": "BIZ UDPゴシック",
  "zen-kaku-gothic-new": "Zen角ゴシック New",
  "zen-maru-gothic": "Zen丸ゴシック",
  "m-plus-1p": "M PLUS 1p",
  "m-plus-rounded-1c": "M PLUS Rounded 1c",
  "ibm-plex-sans-jp": "IBM Plex Sans JP",
  "kosugi": "小杉",
  "kosugi-maru": "小杉丸",
  "murecho": "ムレチョウ",
  "sawarabi-mincho": "さわらび明朝",
  "noto-serif-jp": "Noto Serif JP",
  "biz-udmincho": "BIZ UD明朝",
  "biz-udpmincho": "BIZ UDP明朝",
  "zen-old-mincho": "Zen Old明朝",
  "shippori-mincho": "しっぽり明朝",
  "shippori-mincho-b1": "しっぽり明朝 B1",
  "kaisei-decol": "解星デコール",
  "kaisei-opti": "解星オプティ",
  "kaisei-tokumin": "解星特ミン",
  "kaisei-harunoumi": "解星春の海",
  "yuji-syuku": "遊字 宿",
  "yuji-mai": "遊字 舞",
  "yuji-boku": "遊字 朴",
  "dotgothic16": "DotGothic16",
  "hachi-maru-pop": "はちまるポップ",
  "yusei-magic": "ユウセイ・マジック",
  "shippori-antique": "しっぽりアンチック",
  "shippori-antique-b1": "しっぽりアンチック B1",
  "stick": "スティック",
  "train-one": "トレイン One",
  "rocknroll-one": "ロックンロール One",
  "zen-antique": "Zenアンチック",
  "zen-kurenaido": "Zen紅道",
  "kiwi-maru": "キウイ丸",
  "yomogi": "よもぎ",
  "yuji-hentaigana-akari": "遊字 変体仮名 あかり",
};

if (!apiKey) {
  console.error("GOOGLE_FONTS_API_KEY is required");
  process.exit(1);
}

const response = await fetch(`https://www.googleapis.com/webfonts/v1/webfonts?key=${encodeURIComponent(apiKey)}&sort=alpha`);
if (!response.ok) {
  console.error(`google webfonts api failed: ${response.status}`);
  process.exit(1);
}

const payload = await response.json();
const items = Array.isArray(payload.items) ? payload.items : [];
const japaneseFamilies = items.filter((item) => Array.isArray(item.subsets) && item.subsets.includes("japanese"));

const fonts = japaneseFamilies.map((item) => {
  const family = String(item.family || "").trim();
  const key = family.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
  const category = item.category === "serif" ? "serif" : item.category === "sans-serif" ? "sans" : "display";
  return {
    key,
    label: JAPANESE_LABELS[key] ?? family,
    family,
    category,
    selectable_for_sans: category === "sans",
    selectable_for_serif: category === "serif",
    preview_ui: category === "serif"
      ? "静かな朝に読むためのブリーフィングです。"
      : "今日は重要な記事を3本ピックアップしました。",
    preview_body: category === "serif"
      ? "背景の情報も含めて、落ち着いた調子で本文を読める明朝体を選べます。"
      : "情報の見通しをよくするため、要点を短く整理して読みやすく表示します。",
  };
});

const catalog = {
  catalog_name: "Google Fonts Japanese UI Fonts",
  source: "google-webfonts",
  source_reference: "https://fonts.google.com/?lang=ja_Jpan",
  fonts,
};

process.stdout.write(`${JSON.stringify(catalog, null, 2)}\n`);
