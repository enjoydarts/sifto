"use client";

import { useState, useEffect, useCallback, useMemo, useRef } from "react";
import Link from "next/link";
import { Activity, Download, Lightbulb, Sparkles, Upload } from "lucide-react";
import { api, RecommendedSource, Source, SourceHealth, SourceSuggestion } from "@/lib/api";
import Pagination from "@/components/pagination";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { useConfirm } from "@/components/confirm-provider";

export default function SourcesPage() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const { confirm } = useConfirm();
  const [sources, setSources] = useState<Source[]>([]);
  const [sourceHealthByID, setSourceHealthByID] = useState<Record<string, SourceHealth>>({});
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
  const [suggestions, setSuggestions] = useState<SourceSuggestion[]>([]);
  const [recommendedFeeds, setRecommendedFeeds] = useState<RecommendedSource[]>([]);
  const [loadingSuggestions, setLoadingSuggestions] = useState(false);
  const [suggestionsError, setSuggestionsError] = useState<string | null>(null);
  const [suggestionsLLM, setSuggestionsLLM] = useState<{ provider?: string; model?: string; estimated_cost_usd?: number } | null>(null);
  const [addingSuggestedURL, setAddingSuggestedURL] = useState<string | null>(null);
  const [candidates, setCandidates] = useState<
    { url: string; title: string | null }[]
  >([]);
  const [addError, setAddError] = useState<string | null>(null);
  const [exportingOPML, setExportingOPML] = useState(false);
  const [importingOPML, setImportingOPML] = useState(false);
  const [importingInoreader, setImportingInoreader] = useState(false);
  const opmlInputRef = useRef<HTMLInputElement | null>(null);
  const pageSize = 10;
  const dateLocale = useMemo(() => (locale === "ja" ? "ja-JP" : "en-US"), [locale]);

  const load = useCallback(async () => {
    try {
      const [data, health, recommended] = await Promise.all([
        api.getSources(),
        api.getSourceHealth().catch(() => ({ items: [] as SourceHealth[] })),
        api.getRecommendedSources({ limit: 8 }).catch(() => ({ items: [] as RecommendedSource[] })),
      ]);
      setSources(data ?? []);
      setRecommendedFeeds(recommended.items ?? []);
      const healthMap: Record<string, SourceHealth> = {};
      for (const h of health.items ?? []) healthMap[h.source_id] = h;
      setSourceHealthByID(healthMap);
      setError(null);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const registerSource = async (feedUrl: string) => {
    await api.createSource({
      url: feedUrl,
      type,
      title: title.trim() || undefined,
    });
    setUrl("");
    setTitle("");
    setCandidates([]);
    await load();
  };

  const registerSuggestedSource = async (s: SourceSuggestion) => {
    setAddingSuggestedURL(s.url);
    try {
      await api.createSource({
        url: s.url,
        type: "rss",
        title: s.title ?? undefined,
      });
      setSuggestions((prev) => prev.filter((v) => v.url !== s.url));
      await load();
      showToast(t("sources.toast.suggestedAdded"), "success");
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setAddingSuggestedURL(null);
    }
  };

  const loadSuggestions = async () => {
    setLoadingSuggestions(true);
    setSuggestionsError(null);
    try {
      const res = await api.getSourceSuggestions({ limit: 12 });
      setSuggestions(res.items ?? []);
      setSuggestionsLLM(res.llm ?? null);
    } catch (e) {
      setSuggestionsError(String(e));
      setSuggestionsLLM(null);
    } finally {
      setLoadingSuggestions(false);
    }
  };

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

  const pagedSources = sources.slice((page - 1) * pageSize, page * pageSize);
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

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
        <h1 className="text-2xl font-bold">{t("sources.title")}</h1>
        <p className="mt-1 text-sm text-zinc-500">
          {sources.length.toLocaleString()} {t("common.rows")}
        </p>
        </div>
        <div className="flex items-center gap-2">
          <input
            ref={opmlInputRef}
            type="file"
            accept=".opml,.xml,text/xml,application/xml"
            className="hidden"
            onChange={(e) => {
              const f = e.target.files?.[0];
              if (f) void handleImportOPMLFile(f);
            }}
          />
          <button
            type="button"
            onClick={() => opmlInputRef.current?.click()}
            disabled={importingOPML}
            className="inline-flex items-center gap-1 rounded border border-zinc-200 bg-white px-3 py-1.5 text-xs font-medium text-zinc-700 hover:bg-zinc-50 disabled:opacity-50"
          >
            <Upload className="size-3.5" aria-hidden="true" />
            {importingOPML ? t("sources.opml.importing") : t("sources.opml.import")}
          </button>
          <button
            type="button"
            onClick={() => void handleExportOPML()}
            disabled={exportingOPML}
            className="inline-flex items-center gap-1 rounded border border-zinc-200 bg-white px-3 py-1.5 text-xs font-medium text-zinc-700 hover:bg-zinc-50 disabled:opacity-50"
          >
            <Download className="size-3.5" aria-hidden="true" />
            {exportingOPML ? t("sources.opml.exporting") : t("sources.opml.export")}
          </button>
        </div>
      </div>

      {/* Add form */}
      <form
        onSubmit={handleAdd}
        className="mb-8 rounded-lg border border-zinc-200 bg-white p-4"
      >
        <h2 className="mb-3 text-sm font-semibold text-zinc-700">
          {t("sources.addSource")}
        </h2>
        <div className="mb-2 flex gap-3 text-sm">
          {(["rss", "manual"] as const).map((kind) => (
            <label key={kind} className="flex cursor-pointer items-center gap-1.5">
              <input
                type="radio"
                name="type"
                value={kind}
                checked={type === kind}
                onChange={() => setType(kind)}
                className="accent-zinc-900"
              />
              {kind === "rss" ? t("sources.rss") : t("sources.manual")}
            </label>
          ))}
        </div>
        <div className="flex flex-col gap-2 sm:flex-row">
          <input
            type="url"
            placeholder={
              type === "rss"
                ? t("sources.placeholder.rss")
                : t("sources.placeholder.manual")
            }
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            required
            className="flex-1 rounded border border-zinc-300 px-3 py-2 text-sm outline-none focus:border-zinc-500"
          />
          <input
            type="text"
            placeholder={t("sources.placeholder.nameOptional")}
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            className="rounded border border-zinc-300 px-3 py-2 text-sm outline-none focus:border-zinc-500 sm:w-44"
          />
          <button
            type="submit"
            disabled={adding}
            className="rounded bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-700 disabled:opacity-50"
          >
            {adding ? t("sources.adding") : t("sources.add")}
          </button>
        </div>
        {addError && (
          <p className="mt-2 text-sm text-red-500">{addError}</p>
        )}
        {candidates.length > 1 && (
          <div className="mt-3">
            <p className="mb-2 text-xs font-medium text-zinc-600">
              {t("sources.discover.multiple")}
            </p>
            <ul className="space-y-1">
              {candidates.map((c) => (
                <li
                  key={c.url}
                  className="flex items-center justify-between gap-3 rounded border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm"
                >
                  <div className="min-w-0">
                    {c.title && (
                      <div className="truncate font-medium text-zinc-800">
                        {c.title}
                      </div>
                    )}
                    <div className="truncate text-xs text-zinc-500">{c.url}</div>
                  </div>
                  <button
                    type="button"
                    onClick={async () => {
                      setAdding(true);
                      setAddError(null);
                      try {
                        await registerSource(c.url);
                      } catch (e) {
                        const msg = e instanceof Error ? e.message.replace(/^\d+:\s*/, "") : String(e);
                        setAddError(msg);
                      } finally {
                        setAdding(false);
                      }
                    }}
                    disabled={adding}
                    className="shrink-0 rounded bg-zinc-900 px-3 py-1 text-xs font-medium text-white hover:bg-zinc-700 disabled:opacity-50"
                  >
                    {t("sources.discover.register")}
                  </button>
                </li>
              ))}
            </ul>
            <button
              type="button"
              onClick={() => setCandidates([])}
              className="mt-2 text-xs text-zinc-400 hover:text-zinc-700"
            >
              {t("common.cancel")}
            </button>
          </div>
        )}
      </form>

      <section className="rounded-lg border border-zinc-200 bg-white p-4">
        <div className="mb-3">
          <h2 className="text-sm font-semibold text-zinc-700">{t("sources.recommendedFeeds.title")}</h2>
          <p className="mt-1 text-xs text-zinc-500">{t("sources.recommendedFeeds.desc")}</p>
        </div>
        {recommendedFeeds.length === 0 ? (
          <p className="text-sm text-zinc-500">{t("sources.recommendedFeeds.empty")}</p>
        ) : (
          <ul className="space-y-2">
            {recommendedFeeds.map((s) => (
              <li key={s.source_id} className="flex items-center justify-between gap-2 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2">
                <div className="min-w-0">
                  <div className="truncate text-sm font-medium text-zinc-900">{s.title ?? s.url}</div>
                  {s.title && <div className="truncate text-xs text-zinc-500">{s.url}</div>}
                </div>
                <div className="shrink-0 text-right text-xs text-zinc-600">
                  <div className="font-semibold text-zinc-800">{s.affinity_score.toFixed(2)}</div>
                  <div>{t("sources.recommendedFeeds.reads")}: {s.read_count_30d}</div>
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="rounded-lg border border-zinc-200 bg-white p-4">
        <h2 className="mb-2 text-sm font-semibold text-zinc-700">{t("sources.inoreader.title")}</h2>
        <p className="mb-3 text-xs text-zinc-500">{t("sources.inoreader.desc")}</p>
        <button
          type="button"
          onClick={() => void handleImportInoreader()}
          disabled={importingInoreader}
          className="rounded bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-700 disabled:opacity-50"
        >
          {importingInoreader ? t("sources.inoreader.importing") : t("sources.inoreader.import")}
        </button>
      </section>

      <section className="rounded-lg border border-zinc-200 bg-white p-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="inline-flex items-center gap-2 text-sm font-semibold text-zinc-700">
              <Sparkles className="size-4 text-zinc-500" aria-hidden="true" />
              {t("sources.suggest.title")}
            </h2>
            <p className="mt-1 text-xs text-zinc-500">
              {t("sources.suggest.desc")}
            </p>
          </div>
          <button
            type="button"
            onClick={loadSuggestions}
            disabled={loadingSuggestions}
            className="inline-flex items-center gap-1.5 rounded-lg border border-zinc-200 bg-white px-3 py-2 text-xs font-medium text-zinc-700 hover:bg-zinc-50 disabled:opacity-50"
          >
            <Lightbulb className="size-3.5" aria-hidden="true" />
            {loadingSuggestions
              ? t("sources.suggest.finding")
              : t("sources.suggest.button")}
          </button>
        </div>
        {suggestionsLLM && (
          <p className="mt-2 text-xs text-zinc-500">
            {t("sources.suggest.aiRanked")}: {suggestionsLLM.provider ?? t("common.unknown")} /{" "}
            {suggestionsLLM.model ?? t("common.unknown")}
            {typeof suggestionsLLM.estimated_cost_usd === "number" && (
              <span className="ml-2 text-zinc-400">{`$${suggestionsLLM.estimated_cost_usd.toFixed(6)}`}</span>
            )}
          </p>
        )}
        {suggestionsError && (
          <p className="mt-3 text-sm text-red-500">{suggestionsError}</p>
        )}
        {!suggestionsError && !loadingSuggestions && suggestions.length === 0 && (
          <p className="mt-3 text-sm text-zinc-500">
            {t("sources.suggest.empty")}
          </p>
        )}
        {suggestions.length > 0 && (
          <ul className="mt-3 space-y-2">
            {suggestions.map((s) => (
              <li
                key={s.url}
                className="flex items-start justify-between gap-3 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2"
              >
                <div className="min-w-0">
                  <div className="truncate text-sm font-medium text-zinc-900">{s.title ?? s.url}</div>
                  {s.title && <div className="truncate text-xs text-zinc-500">{s.url}</div>}
                  {s.reasons.length > 0 && (
                    <div className="mt-1 flex flex-wrap gap-1.5">
                      {s.reasons.slice(0, 2).map((reason) => (
                        <span
                          key={`${s.url}-${reason}`}
                          className="rounded-full bg-white px-2 py-0.5 text-[11px] text-zinc-600 ring-1 ring-zinc-200"
                        >
                          {reason}
                        </span>
                      ))}
                    </div>
                  )}
                  {!!s.matched_topics?.length && (
                    <div className="mt-1 flex flex-wrap gap-1.5">
                      {s.matched_topics.slice(0, 3).map((topic) => (
                        <span
                          key={`${s.url}-topic-${topic}`}
                          className="rounded-full bg-blue-50 px-2 py-0.5 text-[11px] text-blue-700 ring-1 ring-blue-200"
                        >
                          {`${t("sources.suggest.topicPrefix")} ${topic}`}
                        </span>
                      ))}
                    </div>
                  )}
                  {s.ai_reason && (
                    <p className="mt-1 text-xs leading-5 text-zinc-600">
                      <span className="font-medium text-zinc-700">
                        {t("sources.suggest.aiReason")}:
                      </span>{" "}
                      {s.ai_reason}
                    </p>
                  )}
                </div>
                <button
                  type="button"
                  onClick={() => registerSuggestedSource(s)}
                  disabled={addingSuggestedURL === s.url}
                  className="shrink-0 rounded bg-zinc-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-zinc-700 disabled:opacity-50"
                >
                  {addingSuggestedURL === s.url
                    ? t("sources.adding")
                    : t("sources.add")}
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>

      {/* State */}
      {loading && <p className="text-sm text-zinc-500">{t("common.loading")}</p>}
      {error && <p className="text-sm text-red-500">{error}</p>}
      {!loading && sources.length === 0 && (
        <p className="text-sm text-zinc-400">
          {t("sources.empty")}
        </p>
      )}

      {/* List */}
      <ul className="space-y-2">
        {pagedSources.map((src) => (
          <li
            key={src.id}
            className="flex items-center gap-3 rounded-xl border border-zinc-200 bg-white px-4 py-3 shadow-sm"
          >
            {/* Toggle */}
            <button
              onClick={() => handleToggle(src.id, src.enabled)}
              aria-label={src.enabled ? t("sources.toggle.disable") : t("sources.toggle.enable")}
              className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors ${
                src.enabled ? "bg-blue-500" : "bg-zinc-300"
              }`}
            >
              <span
                className={`inline-block h-4 w-4 rounded-full bg-white shadow transition-transform ${
                  src.enabled ? "translate-x-4" : "translate-x-0"
                }`}
              />
            </button>

            {/* Info */}
            <div className="min-w-0 flex-1">
              <div className="truncate text-sm font-medium text-zinc-900">
                {src.title ?? src.url}
              </div>
              {sourceHealthByID[src.id] && (
                <div className="mt-0.5 flex flex-wrap items-center gap-1.5 text-xs">
                  <span className={`inline-flex items-center gap-1 rounded-full border px-2 py-0.5 ${
                    sourceHealthByID[src.id].status === "ok"
                      ? "border-green-200 bg-green-50 text-green-700"
                      : sourceHealthByID[src.id].status === "error"
                        ? "border-red-200 bg-red-50 text-red-700"
                        : sourceHealthByID[src.id].status === "stale"
                          ? "border-amber-200 bg-amber-50 text-amber-700"
                          : "border-zinc-200 bg-zinc-50 text-zinc-600"
                  }`}>
                    <Activity className="size-3" aria-hidden="true" />
                    {sourceHealthByID[src.id].status}
                  </span>
                  <span className="text-zinc-400">
                    {sourceHealthByID[src.id].failed_items}/{sourceHealthByID[src.id].total_items} {t("sources.health.failed")}
                  </span>
                </div>
              )}
              {src.title && (
                <div className="truncate text-xs text-zinc-400">{src.url}</div>
              )}
              {src.last_fetched_at && (
                <div className="text-xs text-zinc-400">
                  {t("sources.lastFetched")}:{" "}
                  {new Date(src.last_fetched_at).toLocaleString(dateLocale)}
                </div>
              )}
            </div>

            <div className="flex shrink-0 items-center gap-3">
              <Link
                href={`/items?feed=all&source_id=${encodeURIComponent(src.id)}`}
                className="text-xs text-blue-600 hover:text-blue-800"
              >
                {t("sources.openItems")}
              </Link>
              <button
                type="button"
                onClick={() => openEditDialog(src)}
                className="text-xs text-zinc-500 hover:text-zinc-900"
              >
                {t("sources.edit")}
              </button>
              <button
                type="button"
                onClick={() => handleDelete(src.id)}
                className="text-xs text-zinc-400 hover:text-red-500"
              >
                {t("sources.delete")}
              </button>
            </div>
          </li>
        ))}
      </ul>
      <Pagination total={sources.length} page={page} pageSize={pageSize} onPageChange={setPage} />

      {editingSource && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-900/40 px-4">
          <div className="w-full max-w-lg rounded-xl border border-zinc-200 bg-white p-5 shadow-xl">
            <div className="mb-4">
              <h2 className="text-base font-semibold text-zinc-900">
                {t("sources.editModal.title")}
              </h2>
              <p className="mt-1 break-all text-xs text-zinc-500">{editingSource.url}</p>
            </div>

            <form onSubmit={handleSaveEdit} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-zinc-700">
                  {t("sources.editModal.displayName")}
                </label>
                <input
                  type="text"
                  value={editTitle}
                  onChange={(e) => setEditTitle(e.target.value)}
                  placeholder={t("sources.editModal.placeholder")}
                  className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 outline-none placeholder:text-zinc-400 focus:border-zinc-400"
                  autoFocus
                />
              </div>

              <div className="flex items-center justify-end gap-2">
                <button
                  type="button"
                  onClick={closeEditDialog}
                  disabled={savingEdit}
                  className="rounded-lg border border-zinc-300 bg-white px-4 py-2 text-sm font-medium text-zinc-700 disabled:opacity-50"
                >
                  {t("common.cancel")}
                </button>
                <button
                  type="submit"
                  disabled={savingEdit}
                  className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
                >
                  {savingEdit ? t("common.saving") : t("common.save")}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
