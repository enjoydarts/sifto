"use client";

import { useState, useEffect, useCallback } from "react";
import { api, Source } from "@/lib/api";

export default function SourcesPage() {
  const [sources, setSources] = useState<Source[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [url, setUrl] = useState("");
  const [title, setTitle] = useState("");
  const [type, setType] = useState<"rss" | "manual">("rss");
  const [adding, setAdding] = useState(false);

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

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!url.trim()) return;
    setAdding(true);
    try {
      await api.createSource({
        url: url.trim(),
        type,
        title: title.trim() || undefined,
      });
      setUrl("");
      setTitle("");
      await load();
    } catch (e) {
      alert(`Error: ${e}`);
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
      alert(`Error: ${e}`);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm("このソースを削除しますか？")) return;
    try {
      await api.deleteSource(id);
      setSources((prev) => prev.filter((s) => s.id !== id));
    } catch (e) {
      alert(`Error: ${e}`);
    }
  };

  return (
    <div>
      <h1 className="mb-6 text-2xl font-bold">Sources</h1>

      {/* Add form */}
      <form
        onSubmit={handleAdd}
        className="mb-8 rounded-lg border border-zinc-200 bg-white p-4"
      >
        <h2 className="mb-3 text-sm font-semibold text-zinc-700">
          Add Source
        </h2>
        <div className="mb-2 flex gap-3 text-sm">
          {(["rss", "manual"] as const).map((t) => (
            <label key={t} className="flex cursor-pointer items-center gap-1.5">
              <input
                type="radio"
                name="type"
                value={t}
                checked={type === t}
                onChange={() => setType(t)}
                className="accent-zinc-900"
              />
              {t === "rss" ? "RSS Feed" : "Manual URL"}
            </label>
          ))}
        </div>
        <div className="flex flex-col gap-2 sm:flex-row">
          <input
            type="url"
            placeholder={
              type === "rss"
                ? "https://example.com/feed.rss"
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
            {adding ? "Adding…" : "Add"}
          </button>
        </div>
      </form>

      {/* State */}
      {loading && <p className="text-sm text-zinc-500">Loading…</p>}
      {error && <p className="text-sm text-red-500">{error}</p>}
      {!loading && sources.length === 0 && (
        <p className="text-sm text-zinc-400">
          No sources yet. Add an RSS feed above.
        </p>
      )}

      {/* List */}
      <ul className="space-y-2">
        {sources.map((src) => (
          <li
            key={src.id}
            className="flex items-center gap-3 rounded-lg border border-zinc-200 bg-white px-4 py-3"
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
                  Last fetched:{" "}
                  {new Date(src.last_fetched_at).toLocaleString("ja-JP")}
                </div>
              )}
            </div>

            {/* Delete */}
            <button
              onClick={() => handleDelete(src.id)}
              className="shrink-0 text-xs text-zinc-400 hover:text-red-500"
            >
              Delete
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
