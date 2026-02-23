"use client";

import { useState, useEffect, useCallback } from "react";
import { api, Source } from "@/lib/api";
import Pagination from "@/components/pagination";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { useConfirm } from "@/components/confirm-provider";

export default function SourcesPage() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const { confirm } = useConfirm();
  const [sources, setSources] = useState<Source[]>([]);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [url, setUrl] = useState("");
  const [title, setTitle] = useState("");
  const [type, setType] = useState<"rss" | "manual">("rss");
  const [adding, setAdding] = useState(false);
  const [candidates, setCandidates] = useState<
    { url: string; title: string | null }[]
  >([]);
  const [addError, setAddError] = useState<string | null>(null);
  const pageSize = 10;

  const load = useCallback(async () => {
    try {
      const data = await api.getSources();
      setSources(data ?? []);
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
      cancelLabel: locale === "ja" ? "キャンセル" : "Cancel",
    });
    if (!ok) return;
    try {
      await api.deleteSource(id);
      setSources((prev) => prev.filter((s) => s.id !== id));
      showToast(locale === "ja" ? "ソースを削除しました" : "Source deleted", "success");
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    }
  };

  const pagedSources = sources.slice((page - 1) * pageSize, page * pageSize);

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-bold">{t("sources.title")}</h1>
        <p className="mt-1 text-sm text-zinc-500">
          {sources.length.toLocaleString()} {t("common.rows")}
        </p>
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
                ? "https://example.com or https://example.com/feed.rss"
                : "https://example.com/article"
            }
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            required
            className="flex-1 rounded border border-zinc-300 px-3 py-2 text-sm outline-none focus:border-zinc-500"
          />
          <input
            type="text"
            placeholder="Name (optional)"
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
              Multiple feeds found. Select one to register:
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
                    Register
                  </button>
                </li>
              ))}
            </ul>
            <button
              type="button"
              onClick={() => setCandidates([])}
              className="mt-2 text-xs text-zinc-400 hover:text-zinc-700"
            >
              Cancel
            </button>
          </div>
        )}
      </form>

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
              aria-label={src.enabled ? "Disable" : "Enable"}
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
              {src.title && (
                <div className="truncate text-xs text-zinc-400">{src.url}</div>
              )}
              {src.last_fetched_at && (
                <div className="text-xs text-zinc-400">
                  {t("sources.lastFetched")}:{" "}
                  {new Date(src.last_fetched_at).toLocaleString(locale === "ja" ? "ja-JP" : "en-US")}
                </div>
              )}
            </div>

            {/* Delete */}
            <button
              onClick={() => handleDelete(src.id)}
              className="shrink-0 text-xs text-zinc-400 hover:text-red-500"
            >
              {t("sources.delete")}
            </button>
          </li>
        ))}
      </ul>
      <Pagination total={sources.length} page={page} pageSize={pageSize} onPageChange={setPage} />
    </div>
  );
}
