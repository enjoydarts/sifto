"use client";

import {
  createContext,
  ReactNode,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";

export type Locale = "ja" | "en";

type Dict = Record<string, string>;

const dict: Record<Locale, Dict> = {
  ja: {
    "nav.items": "記事",
    "nav.sources": "ソース",
    "nav.digests": "ダイジェスト",
    "nav.llmUsage": "LLM利用",
    "nav.settings": "設定",
    "nav.debug": "デバッグ",
    "nav.signOut": "サインアウト",
    "nav.language": "言語",
    "common.loading": "読み込み中…",
    "common.error": "エラー",
    "common.noData": "データがありません。",
    "common.page": "ページ",
    "common.prev": "前へ",
    "common.next": "次へ",
    "common.rows": "件",
    "common.refresh": "更新",
    "dashboard.title": "ダッシュボード",
    "dashboard.subtitle": "配信状況・処理状況・コストの概要",
    "dashboard.card.sources": "ソース数",
    "dashboard.card.items": "記事数（取得分）",
    "dashboard.card.failedItems": "失敗記事",
    "dashboard.card.digests": "ダイジェスト数",
    "dashboard.card.llmCost": "LLMコスト（期間合計）",
    "dashboard.card.llmCalls": "LLM呼び出し",
    "dashboard.section.itemsStatus": "記事ステータス",
    "dashboard.section.latestDigests": "最新ダイジェスト",
    "dashboard.section.llmSummary": "LLM利用（JST日次）",
    "items.title": "記事一覧",
    "items.empty": "記事がありません。",
    "items.retry": "再試行",
    "items.retrying": "再試行中…",
    "items.filter.all": "すべて",
    "items.filter.summarized": "要約済み",
    "items.filter.new": "新規",
    "items.filter.fetched": "取得済み",
    "items.filter.facts": "事実抽出済み",
    "items.filter.facts_extracted": "事実抽出済み",
    "items.filter.failed": "失敗",
    "status.new": "新規",
    "status.fetched": "取得済み",
    "status.facts_extracted": "事実抽出",
    "status.summarized": "要約済み",
    "status.failed": "失敗",
    "digests.title": "ダイジェスト",
    "digests.empty": "ダイジェストはまだありません。",
    "digests.sent": "送信済み",
    "digests.pending": "未送信",
    "sources.title": "ソース",
    "sources.addSource": "ソース追加",
    "sources.empty": "ソースはまだありません。",
    "sources.add": "追加",
    "sources.adding": "追加中…",
    "sources.delete": "削除",
    "sources.lastFetched": "最終取得",
    "sources.rss": "RSSフィード",
    "sources.manual": "手動URL",
    "sources.confirmDelete": "このソースを削除しますか？",
    "llm.title": "LLM利用状況",
    "llm.subtitle": "コスト・トークン利用履歴（JST日次集計 + 最新実行ログ）",
    "llm.days": "集計日数",
    "llm.limit": "履歴件数",
    "llm.totalCost": "合計コスト",
    "llm.totalCalls": "呼び出し回数",
    "llm.input": "入力",
    "llm.output": "出力",
    "llm.dailySummary": "日次集計（JST）",
    "llm.recentLogs": "最新履歴",
    "llm.noSummary": "集計データがありません。",
    "llm.noLogs": "履歴がありません。",
    "settings.title": "設定",
    "settings.subtitle": "ユーザー別 APIキー（Anthropic / OpenAI）と月次LLM予算を管理",
  },
  en: {
    "nav.items": "Items",
    "nav.sources": "Sources",
    "nav.digests": "Digests",
    "nav.llmUsage": "LLM Usage",
    "nav.settings": "Settings",
    "nav.debug": "Debug",
    "nav.signOut": "Sign out",
    "nav.language": "Language",
    "common.loading": "Loading…",
    "common.error": "Error",
    "common.noData": "No data.",
    "common.page": "Page",
    "common.prev": "Prev",
    "common.next": "Next",
    "common.rows": "rows",
    "common.refresh": "Refresh",
    "dashboard.title": "Dashboard",
    "dashboard.subtitle": "Overview of delivery, processing, and cost",
    "dashboard.card.sources": "Sources",
    "dashboard.card.items": "Items (fetched)",
    "dashboard.card.failedItems": "Failed Items",
    "dashboard.card.digests": "Digests",
    "dashboard.card.llmCost": "LLM Cost (period)",
    "dashboard.card.llmCalls": "LLM Calls",
    "dashboard.section.itemsStatus": "Item Status",
    "dashboard.section.latestDigests": "Latest Digests",
    "dashboard.section.llmSummary": "LLM Usage (JST daily)",
    "items.title": "Items",
    "items.empty": "No items.",
    "items.retry": "Retry",
    "items.retrying": "Retrying…",
    "items.filter.all": "All",
    "items.filter.summarized": "Summarized",
    "items.filter.new": "New",
    "items.filter.fetched": "Fetched",
    "items.filter.facts": "Facts",
    "items.filter.facts_extracted": "Facts",
    "items.filter.failed": "Failed",
    "status.new": "New",
    "status.fetched": "Fetched",
    "status.facts_extracted": "Facts",
    "status.summarized": "Summarized",
    "status.failed": "Failed",
    "digests.title": "Digests",
    "digests.empty": "No digests yet.",
    "digests.sent": "Sent",
    "digests.pending": "Pending",
    "sources.title": "Sources",
    "sources.addSource": "Add Source",
    "sources.empty": "No sources yet.",
    "sources.add": "Add",
    "sources.adding": "Adding…",
    "sources.delete": "Delete",
    "sources.lastFetched": "Last fetched",
    "sources.rss": "RSS Feed",
    "sources.manual": "Manual URL",
    "sources.confirmDelete": "Delete this source?",
    "llm.title": "LLM Usage",
    "llm.subtitle": "Cost and token usage (JST daily summary + recent logs)",
    "llm.days": "Days",
    "llm.limit": "Rows",
    "llm.totalCost": "Total Cost",
    "llm.totalCalls": "Calls",
    "llm.input": "Input",
    "llm.output": "Output",
    "llm.dailySummary": "Daily Summary (JST)",
    "llm.recentLogs": "Recent Logs",
    "llm.noSummary": "No summary data.",
    "llm.noLogs": "No logs.",
    "settings.title": "Settings",
    "settings.subtitle": "Manage per-user API keys (Anthropic / OpenAI) and monthly LLM budget",
  },
};

type I18nValue = {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: string, fallback?: string) => string;
};

const I18nContext = createContext<I18nValue | null>(null);

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocale] = useState<Locale>("ja");

  useEffect(() => {
    const saved = globalThis.localStorage?.getItem("sifto.locale");
    if (saved === "ja" || saved === "en") setLocale(saved);
  }, []);

  useEffect(() => {
    globalThis.localStorage?.setItem("sifto.locale", locale);
    if (typeof document !== "undefined") {
      document.documentElement.lang = locale;
    }
  }, [locale]);

  const value = useMemo<I18nValue>(
    () => ({
      locale,
      setLocale,
      t: (key, fallback) => dict[locale][key] ?? fallback ?? key,
    }),
    [locale]
  );

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const ctx = useContext(I18nContext);
  if (!ctx) {
    throw new Error("useI18n must be used within I18nProvider");
  }
  return ctx;
}
