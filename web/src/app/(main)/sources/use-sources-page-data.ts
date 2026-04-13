"use client";

import { useState, useEffect, useCallback, useMemo, useRef } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, Source, SourceDailyStats, SourceHealth, SourceItemStats, SourceOptimizationItem, SourceSuggestion, SourcesDailyOverview } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { useConfirm } from "@/components/confirm-provider";
import { queryKeys } from "@/lib/query-keys";

function navigatorThemeTokens(persona: string, avatarStyle?: string) {
  const key = avatarStyle || persona;
  switch (key) {
    case "hype":
      return {
        shell: "border-[#f0b677] bg-[linear-gradient(180deg,#fff6e9_0%,#fffdf8_100%)]",
        header: "",
        avatar: "bg-[#d96c28] text-white",
        bubble: "border-[#f0b677] bg-[#fff0da]",
        badge: "bg-[#d96c28] text-white",
      };
    case "analyst":
      return {
        shell: "border-[#9db5d5] bg-[linear-gradient(180deg,#eef4fb_0%,#fbfdff_100%)]",
        header: "",
        avatar: "bg-[#365f93] text-white",
        bubble: "border-[#c8d8ec] bg-[#f3f8fd]",
        badge: "bg-[#365f93] text-white",
      };
    case "concierge":
      return {
        shell: "border-[#d9c7b2] bg-[linear-gradient(180deg,#fbf5ef_0%,#fffdfb_100%)]",
        header: "",
        avatar: "bg-[#8c6a52] text-white",
        bubble: "border-[#e7d8c8] bg-[#fff8f1]",
        badge: "bg-[#8c6a52] text-white",
      };
    case "snark":
      return {
        shell: "border-[#caa8a8] bg-[linear-gradient(180deg,#f9eeee_0%,#fffdfd_100%)]",
        header: "",
        avatar: "bg-[#7d3f3f] text-white",
        bubble: "border-[#dfc2c2] bg-[#fff5f5]",
        badge: "bg-[#7d3f3f] text-white",
      };
    case "native":
      return {
        shell: "border-[#efb2c6] bg-[linear-gradient(180deg,#fff0f6_0%,#fffdfd_100%)]",
        header: "",
        avatar: "bg-[#d24f7a] text-white",
        bubble: "border-[#f3c8d7] bg-[#fff5f8]",
        badge: "bg-[#d24f7a] text-white",
      };
    case "junior":
      return {
        shell: "border-[#edb0aa] bg-[linear-gradient(180deg,#fff3f1_0%,#fffdfd_100%)]",
        header: "",
        avatar: "bg-[#d85a5a] text-white",
        bubble: "border-[#f1c9c4] bg-[#fff8f7]",
        badge: "bg-[#d85a5a] text-white",
      };
    case "urban":
      return {
        shell: "border-[#b8dcf0] bg-[linear-gradient(180deg,#f1fbff_0%,#fffdfd_100%)]",
        header: "",
        avatar: "bg-[#57a9d8] text-white",
        bubble: "border-[#cae9f7] bg-[#f6fcff]",
        badge: "bg-[#57a9d8] text-white",
      };
    default:
      return {
        shell: "border-[#c7b79c] bg-[linear-gradient(180deg,#f8f3e7_0%,#fffdf8_100%)]",
        header: "",
        avatar: "bg-[#8f5a24] text-white",
        bubble: "border-[#ddcfb7] bg-[#fff8ea]",
        badge: "bg-[#8f5a24] text-white",
      };
  }
}

export function useSourcesPageData() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const { confirm } = useConfirm();
  const [activeSection, setActiveSection] = useState<"overview" | "sources" | "optimization" | "add">("overview");
  const [sources, setSources] = useState<Source[]>([]);
  const [sourceHealthByID, setSourceHealthByID] = useState<Record<string, SourceHealth>>({});
  const [sourceItemStatsByID, setSourceItemStatsByID] = useState<Record<string, SourceItemStats>>({});
  const [sourceDailyStatsByID, setSourceDailyStatsByID] = useState<Record<string, SourceDailyStats>>({});
  const [sourceOptimization, setSourceOptimization] = useState<SourceOptimizationItem[]>([]);
  const [sourcesDailyOverview, setSourcesDailyOverview] = useState<SourcesDailyOverview | null>(null);
  const [loadingDailyStats, setLoadingDailyStats] = useState(false);
  const [dailyStatsError, setDailyStatsError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [url, setUrl] = useState("");
  const [title, setTitle] = useState("");
  const [type, setType] = useState<"rss" | "manual">("rss");
  const [adding, setAdding] = useState(false);
  const [editingSource, setEditingSource] = useState<Source | null>(null);
  const [editTitle, setEditTitle] = useState("");
  const [savingEdit, setSavingEdit] = useState(false);
  const [recommendations, setRecommendations] = useState<SourceSuggestion[]>([]);
  const [loadingSuggestions, setLoadingSuggestions] = useState(false);
  const [suggestionsError, setSuggestionsError] = useState<string | null>(null);
  const [suggestionsLLM, setSuggestionsLLM] = useState<{
    provider?: string;
    model?: string;
    estimated_cost_usd?: number;
    warning?: string;
    error?: string;
    stage?: string;
    items_count?: number;
  } | null>(null);
  const [addingSuggestedURL, setAddingSuggestedURL] = useState<string | null>(null);
  const [hasLoadedSuggestions, setHasLoadedSuggestions] = useState(false);
  const [candidates, setCandidates] = useState<
    { url: string; title: string | null }[]
  >([]);
  const [addError, setAddError] = useState<string | null>(null);
  const [exportingOPML, setExportingOPML] = useState(false);
  const [importingOPML, setImportingOPML] = useState(false);
  const [importingInoreader, setImportingInoreader] = useState(false);
  const [navigatorPersona, setNavigatorPersona] = useState("editor");
  const [sourceNavigator, setSourceNavigator] = useState<Awaited<ReturnType<typeof api.getSourceNavigator>>["navigator"] | null>(null);
  const [sourceNavigatorLoading, setSourceNavigatorLoading] = useState(false);
  const [sourceNavigatorError, setSourceNavigatorError] = useState<string | null>(null);
  const [sourceNavigatorOpen, setSourceNavigatorOpen] = useState(false);
  const opmlInputRef = useRef<HTMLInputElement | null>(null);
  const loadSequenceRef = useRef(0);
  const dailyStatsSequenceRef = useRef(0);
  const settingsQuery = useQuery({
    queryKey: queryKeys.settings.all(),
    queryFn: () => api.getSettings(),
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  });
  const pageSize = 10;
  const dateLocale = useMemo(() => (locale === "ja" ? "ja-JP" : "en-US"), [locale]);
  const normalizeSuggestionReason = useCallback((value: string | null | undefined) => {
    return (value ?? "").trim().replace(/\s+/g, " ");
  }, []);
  const normalizeSuggestionText = useCallback((value: unknown): string | null => {
    if (typeof value === "string") {
      const normalized = value.trim();
      return normalized === "" ? null : normalized;
    }
    if (value === null || value === undefined) return null;
    if (typeof value === "number" || typeof value === "boolean") {
      return String(value);
    }
    return null;
  }, []);
  const normalizeSuggestionStringList = useCallback((value: unknown): string[] => {
    if (!Array.isArray(value)) return [];
    const out: string[] = [];
    for (const raw of value) {
      const normalized = normalizeSuggestionText(raw);
      if (normalized !== null) {
        out.push(normalized);
      }
    }
    return out;
  }, [normalizeSuggestionText]);
  const suggestionLLMLabel = useMemo(() => {
    if (!suggestionsLLM) return null;
    if (suggestionsLLM.stage === "seed_generation") return t("sources.suggest.aiSeed");
    return t("sources.suggest.aiRanked");
  }, [suggestionsLLM, t]);
  const normalizeSuggestion = useCallback(
    (raw: unknown): SourceSuggestion | null => {
      if (!raw || typeof raw !== "object") return null;
      const source = raw as Record<string, unknown>;
      const url = normalizeSuggestionText(source.url);
      if (!url) return null;
      const title = normalizeSuggestionText(source.title);
      return {
        url,
        title,
        reasons: normalizeSuggestionStringList(source.reasons),
        matched_topics: normalizeSuggestionStringList(source.matched_topics),
        ai_reason: normalizeSuggestionText(source.ai_reason) ?? null,
        ai_confidence: typeof source.ai_confidence === "number" ? source.ai_confidence : null,
        seed_source_ids: normalizeSuggestionStringList(source.seed_source_ids),
      };
    },
    [normalizeSuggestionStringList, normalizeSuggestionText]
  );

  const load = useCallback(async () => {
    const seq = ++loadSequenceRef.current;
    try {
      setSuggestionsError(null);
      const [data, stats, health, optimization] = await Promise.all([
        api.getSources(),
        api.getSourceItemStats().catch(() => ({ items: [] as SourceItemStats[] })),
        api.getSourceHealth().catch(() => ({ items: [] as SourceHealth[] })),
        api.getSourceOptimization().catch(() => ({ items: [] as SourceOptimizationItem[] })),
      ]);
      if (seq !== loadSequenceRef.current) return;
      setSources(data ?? []);
      const statsMap: Record<string, SourceItemStats> = {};
      const healthMap: Record<string, SourceHealth> = {};
      for (const stat of stats.items ?? []) statsMap[stat.source_id] = stat;
      for (const h of health.items ?? []) healthMap[h.source_id] = h;
      setSourceItemStatsByID(statsMap);
      setSourceHealthByID(healthMap);
      setSourceOptimization(optimization.items ?? []);
      setError(null);
    } catch (e) {
      if (seq !== loadSequenceRef.current) return;
      setError(String(e));
    } finally {
      if (seq !== loadSequenceRef.current) return;
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    setNavigatorPersona(settingsQuery.data?.llm_models?.navigator_persona?.trim() || "editor");
  }, [settingsQuery.data]);

  const loadDailyStats = useCallback(async () => {
    const seq = ++dailyStatsSequenceRef.current;
    setLoadingDailyStats(true);
    setDailyStatsError(null);
    try {
      const data = await api.getSourceDailyStats(30);
      if (seq !== dailyStatsSequenceRef.current) return;
      const next: Record<string, SourceDailyStats> = {};
      for (const row of data.items ?? []) next[row.source_id] = row;
      setSourceDailyStatsByID(next);
      setSourcesDailyOverview(data.overview ?? null);
    } catch (e) {
      if (seq !== dailyStatsSequenceRef.current) return;
      const message = String(e);
      setDailyStatsError(message);
      showToast(`${t("common.error")}: ${message}`, "error");
    } finally {
      if (seq !== dailyStatsSequenceRef.current) return;
      setLoadingDailyStats(false);
    }
  }, [showToast, t]);

  useEffect(() => {
    if (activeSection === "overview" && !loadingDailyStats && Object.keys(sourceDailyStatsByID).length === 0) {
      void loadDailyStats();
    }
  }, [activeSection, loadDailyStats, loadingDailyStats, sourceDailyStatsByID]);

  const registerSource = async (feedUrl: string) => {
    if (adding) return;
    setAdding(true);
    try {
      await api.createSource({
        url: feedUrl,
        type,
        title: title.trim() || undefined,
      });
      setUrl("");
      setTitle("");
      setCandidates([]);
      await load();
      void loadDailyStats();
    } finally {
      setAdding(false);
    }
  };

  const registerSuggestedSource = async (s: SourceSuggestion) => {
    setAddingSuggestedURL(s.url);
    try {
      let targetURL = s.url;
      let foundFeed = false;
      try {
        const discovered = await api.discoverFeeds(s.url);
        if ((discovered.feeds ?? []).length > 0) {
          targetURL = discovered.feeds[0].url;
          foundFeed = true;
        }
      } catch {
        // Keep original URL and let createSource validate.
      }
      if (!foundFeed) {
        throw new Error(t("sources.suggest.noFeedFound"));
      }
      await api.createSource({
        url: targetURL,
        type: "rss",
        title: s.title ?? undefined,
      });
      setRecommendations((prev) => prev.filter((v) => v.url !== s.url));
      await load();
      void loadDailyStats();
      showToast(t("sources.toast.suggestedAdded"), "success");
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setAddingSuggestedURL(null);
    }
  };

  const loadSuggestions = useCallback(async () => {
    setHasLoadedSuggestions(false);
    setLoadingSuggestions(true);
    setSuggestionsError(null);
    setSuggestionsLLM(null);
    try {
      const res = await api.getSourceSuggestions({ limit: 24 });
      const next = Array.isArray(res.items) ? res.items.map(normalizeSuggestion).filter((v): v is SourceSuggestion => v !== null) : [];
      setRecommendations(next);
      setSuggestionsLLM(res.llm ?? null);
    } catch (e) {
      setSuggestionsError(String(e));
      setSuggestionsLLM(null);
    } finally {
      setLoadingSuggestions(false);
      setHasLoadedSuggestions(true);
    }
  }, [normalizeSuggestion]);

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!url.trim()) return;
    setAdding(true);
    setAddError(null);
    try {
      if (type === "rss") {
        const { feeds } = await api.discoverFeeds(url.trim());
        if (feeds.length === 1) {
          await registerSource(feeds[0].url);
        } else {
          setCandidates(feeds);
        }
      } else {
        await registerSource(url.trim());
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message.replace(/^\d+:\s*/, "") : String(e);
      setAddError(msg);
    } finally {
      setAdding(false);
    }
  };

  const handleToggle = async (id: string, enabled: boolean) => {
    try {
      await api.updateSource(id, { enabled: !enabled });
      setSources((prev) =>
        prev.map((s) => (s.id === id ? { ...s, enabled: !enabled } : s))
      );
      void loadDailyStats();
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    }
  };

  const handleDelete = async (id: string) => {
    const ok = await confirm({
      title: t("sources.delete"),
      message: t("sources.confirmDelete"),
      tone: "danger",
      confirmLabel: t("sources.delete"),
      cancelLabel: t("common.cancel"),
    });
    if (!ok) return;
    try {
      await api.deleteSource(id);
      setSources((prev) => prev.filter((s) => s.id !== id));
      void loadDailyStats();
      showToast(t("sources.toast.deleted"), "success");
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    }
  };

  const openEditDialog = (src: Source) => {
    setEditingSource(src);
    setEditTitle(src.title ?? "");
  };

  const closeEditDialog = () => {
    if (savingEdit) return;
    setEditingSource(null);
    setEditTitle("");
  };

  const handleSaveEdit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editingSource) return;
    setSavingEdit(true);
    try {
      const next = await api.updateSource(editingSource.id, { title: editTitle });
      setSources((prev) => prev.map((s) => (s.id === next.id ? next : s)));
      setEditingSource(null);
      setEditTitle("");
      showToast(t("sources.toast.updated"), "success");
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setSavingEdit(false);
    }
  };

  const handleExportOPML = async () => {
    setExportingOPML(true);
    try {
      const text = await api.exportSourcesOPML();
      const blob = new Blob([text], { type: "text/x-opml;charset=utf-8" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `sifto-sources-${new Date().toISOString().slice(0, 10)}.opml`;
      a.click();
      URL.revokeObjectURL(url);
      showToast(t("sources.toast.opmlExported"), "success");
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setExportingOPML(false);
    }
  };

  const handleImportOPMLFile = async (file: File) => {
    setImportingOPML(true);
    try {
      const text = await file.text();
      const res = await api.importSourcesOPML(text);
      await load();
      showToast(
        `${t("sources.toast.opmlImportedPrefix")}: ${t("sources.toast.opmlImportedAdded")} ${res.added} / ${t("sources.toast.opmlImportedSkipped")} ${res.skipped} / ${t("sources.toast.opmlImportedInvalid")} ${res.invalid}`,
        "success"
      );
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setImportingOPML(false);
      if (opmlInputRef.current) opmlInputRef.current.value = "";
    }
  };

  const handleImportInoreader = async () => {
    setImportingInoreader(true);
    try {
      const res = await api.importInoreaderSources();
      await load();
      showToast(
        `${t("sources.toast.inoreaderImportedPrefix")}: ${t("sources.toast.opmlImportedAdded")} ${res.added} / ${t("sources.toast.opmlImportedSkipped")} ${res.skipped} / ${t("sources.toast.opmlImportedInvalid")} ${res.invalid}`,
        "success"
      );
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setImportingInoreader(false);
    }
  };
  const formatShortDate = useCallback((value: string) => {
    const [y, m, d] = value.split("-").map(Number);
    const dt = new Date(Date.UTC(y, (m ?? 1) - 1, d ?? 1));
    return dt.toLocaleDateString(dateLocale, { month: "numeric", day: "numeric", timeZone: "UTC" });
  }, [dateLocale]);
  const overviewChartRows = useMemo(
    () =>
      (sourcesDailyOverview?.daily_counts ?? []).map((entry) => ({
        day: entry.day,
        label: formatShortDate(entry.day),
        count: entry.count,
      })),
    [formatShortDate, sourcesDailyOverview]
  );
  const healthSummary = useMemo(() => {
    const values = Object.values(sourceHealthByID);
    return {
      ok: values.filter((item) => item.status === "ok").length,
      stale: values.filter((item) => item.status === "stale").length,
      error: values.filter((item) => item.status === "error").length,
    };
  }, [sourceHealthByID]);
  const sectionItems = useMemo(
    () =>
      [
        {
          key: "overview" as const,
          title: t("sources.section.overviewTitle"),
          meta: t("sources.tabs.activityDesc"),
        },
        {
          key: "sources" as const,
          title: t("sources.section.sourcesTitle"),
          meta: t("sources.tabs.manageDesc"),
        },
        {
          key: "optimization" as const,
          title: t("sources.optimization.title"),
          meta: t("sources.tabs.improveDesc"),
        },
        {
          key: "add" as const,
          title: t("sources.tabs.addSource"),
          meta: t("sources.tabs.discoverDesc"),
        },
      ] satisfies Array<{ key: "overview" | "sources" | "optimization" | "add"; title: string; meta: string }>,
    [t]
  );
  const sourceNavigatorDisplayPersona = sourceNavigator?.avatar_style || sourceNavigator?.persona || navigatorPersona;
  const sourceNavigatorTheme = sourceNavigator ? navigatorThemeTokens(sourceNavigator.persona, sourceNavigator.avatar_style) : navigatorThemeTokens(navigatorPersona);

  const openSourceNavigator = useCallback(async () => {
    if (sourceNavigatorLoading) return;
    if (sourceNavigator) {
      setSourceNavigatorOpen(true);
      setSourceNavigatorError(null);
      return;
    }
    setSourceNavigatorLoading(true);
    setSourceNavigatorError(null);
    try {
      const resp = await api.getSourceNavigator();
      if (!resp?.navigator) {
        setSourceNavigatorError(t("sources.navigator.unavailable"));
        return;
      }
      setSourceNavigator(resp.navigator);
      setSourceNavigatorOpen(true);
    } catch (e) {
      setSourceNavigatorError(t("sources.navigator.error"));
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setSourceNavigatorLoading(false);
    }
  }, [showToast, sourceNavigator, sourceNavigatorLoading, t]);

  const pagedSources = sources.slice((page - 1) * pageSize, page * pageSize);

  return {
    t, locale, dateLocale,
    activeSection, setActiveSection,
    sources, setSources,
    sourceHealthByID,
    sourceItemStatsByID,
    sourceDailyStatsByID,
    sourceOptimization,
    sourcesDailyOverview,
    loadingDailyStats,
    dailyStatsError,
    page, setPage,
    loading, error,
    url, setUrl,
    title, setTitle,
    type, setType,
    adding,
    editingSource,
    editTitle, setEditTitle,
    savingEdit,
    recommendations,
    loadingSuggestions,
    suggestionsError,
    suggestionsLLM,
    suggestionLLMLabel,
    addingSuggestedURL,
    hasLoadedSuggestions,
    candidates,
    addError,
    exportingOPML,
    importingOPML,
    importingInoreader,
    navigatorPersona,
    sourceNavigator,
    sourceNavigatorLoading,
    sourceNavigatorError,
    sourceNavigatorOpen,
    setSourceNavigatorOpen,
    sourceNavigatorDisplayPersona,
    sourceNavigatorTheme,
    opmlInputRef,
    pageSize,
    normalizeSuggestionReason,
    formatShortDate,
    overviewChartRows,
    healthSummary,
    sectionItems,
    pagedSources,
    load,
    loadDailyStats,
    loadSuggestions,
    registerSource,
    registerSuggestedSource,
    handleAdd,
    handleToggle,
    handleDelete,
    openEditDialog,
    closeEditDialog,
    handleSaveEdit,
    handleExportOPML,
    handleImportOPMLFile,
    handleImportInoreader,
    openSourceNavigator,
  };
}
