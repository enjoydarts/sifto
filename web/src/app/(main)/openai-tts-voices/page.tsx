"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { RefreshCw, Search, X } from "lucide-react";
import { api, OpenAITTSVoiceSnapshot, OpenAITTSVoicesResponse } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { PageTransition } from "@/components/page-transition";
import { PageHeader } from "@/components/ui/page-header";

function formatDateTime(value?: string | null) {
  if (!value) return "—";
  return new Date(value).toLocaleString();
}

function parseMetadataJSON(value?: string | null): Record<string, unknown> | null {
  if (!value) return null;
  try {
    const parsed = JSON.parse(value);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) return null;
    return parsed as Record<string, unknown>;
  } catch {
    return null;
  }
}

function getSupportedModels(voice: OpenAITTSVoiceSnapshot): string[] {
  const metadata = parseMetadataJSON(voice.metadata_json);
  const supportedModels = metadata?.supported_models;
  if (!Array.isArray(supportedModels)) return [];
  return supportedModels.filter((item): item is string => typeof item === "string" && item.trim().length > 0);
}

export default function OpenAITTSVoicesPage() {
  const { t } = useI18n();
  const { showToast } = useToast();
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [data, setData] = useState<OpenAITTSVoicesResponse | null>(null);
  const [selectedVoiceID, setSelectedVoiceID] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const next = await api.getOpenAITTSVoices();
      setData(next);
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

  useEffect(() => {
    if (data?.latest_run?.status !== "running" || data.latest_run.trigger_type !== "manual") return;
    const timer = window.setInterval(load, 3000);
    return () => window.clearInterval(timer);
  }, [data?.latest_run, load]);

  const handleSync = useCallback(async () => {
    setSyncing(true);
    try {
      const next = await api.syncOpenAITTSVoices();
      setData(next);
      setError(null);
      showToast(t("openaiTTS.syncCompleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSyncing(false);
    }
  }, [showToast, t]);

  const filteredVoices = useMemo(() => {
    const q = query.trim().toLowerCase();
    const voices = data?.voices ?? [];
    if (!q) return voices;
    return voices.filter((voice) =>
      [
        voice.voice_id,
        voice.name,
        voice.description,
        voice.language,
        getSupportedModels(voice).join(" "),
      ]
        .join(" ")
        .toLowerCase()
        .includes(q)
    );
  }, [data?.voices, query]);

  const selectedVoice = useMemo(() => {
    const voices = data?.voices ?? [];
    return selectedVoiceID ? voices.find((voice) => voice.voice_id === selectedVoiceID) ?? null : null;
  }, [data?.voices, selectedVoiceID]);

  const latestRunLabel = data?.latest_run?.finished_at ?? data?.latest_run?.started_at;
  const voiceCount = data?.voices?.length ?? 0;
  const addedCount = data?.latest_change_summary?.added.length ?? 0;
  const removedCount = data?.latest_change_summary?.removed.length ?? 0;

  return (
    <PageTransition>
      <div className="space-y-6">
        <PageHeader
          title={t("openaiTTS.title")}
          description={t("openaiTTS.subtitle")}
          actions={
            <button
              type="button"
              onClick={handleSync}
              disabled={syncing}
              className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90 disabled:opacity-60"
            >
              <RefreshCw className={`size-4 ${syncing ? "animate-spin" : ""}`} />
              {syncing ? t("openaiTTS.syncing") : t("openaiTTS.sync")}
            </button>
          }
        />

        <section className="grid gap-4 lg:grid-cols-[minmax(0,1.35fr)_minmax(320px,0.65fr)]">
          <div className="rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-5 shadow-[0_18px_48px_rgba(35,24,12,0.08)]">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("openaiTTS.searchTitle")}</div>
                <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("openaiTTS.searchDescription")}</p>
              </div>
              <div className="text-sm text-[var(--color-editorial-ink-soft)]">
                {`${t("openaiTTS.voiceCount")}: ${voiceCount}`}
              </div>
            </div>

            <div className="mt-5 flex items-center gap-3 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-3">
              <Search className="size-4 text-[var(--color-editorial-ink-soft)]" />
              <input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={t("openaiTTS.search")}
                className="w-full bg-transparent text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
              />
            </div>

            {error ? <p className="mt-4 text-sm text-red-600">{error}</p> : null}

            <div className="mt-5 overflow-hidden rounded-[24px] border border-[var(--color-editorial-line)] bg-white">
              <div className="overflow-x-auto">
                <table className="min-w-[920px] divide-y divide-[var(--color-editorial-line)] text-sm">
                  <thead className="bg-[var(--color-editorial-panel-strong)]">
                    <tr className="text-left text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">
                      <th className="px-4 py-3">{t("openaiTTS.table.voice")}</th>
                      <th className="px-4 py-3">{t("openaiTTS.table.language")}</th>
                      <th className="px-4 py-3">{t("openaiTTS.table.models")}</th>
                      <th className="px-4 py-3">{t("openaiTTS.table.description")}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[var(--color-editorial-line)] bg-white">
                    {loading ? (
                      <tr>
                        <td colSpan={4} className="px-4 py-8 text-center text-sm text-[var(--color-editorial-ink-soft)]">
                          {t("common.loading")}
                        </td>
                      </tr>
                    ) : filteredVoices.length === 0 ? (
                      <tr>
                        <td colSpan={4} className="px-4 py-8 text-center text-sm text-[var(--color-editorial-ink-soft)]">
                          {t("openaiTTS.noVoices")}
                        </td>
                      </tr>
                    ) : (
                      filteredVoices.map((voice) => {
                        const supportedModels = getSupportedModels(voice);
                        const selected = selectedVoice?.voice_id === voice.voice_id;
                        return (
                          <tr
                            key={voice.voice_id}
                            className={`cursor-pointer transition hover:bg-[var(--color-editorial-panel)] ${selected ? "bg-[var(--color-editorial-panel)]" : ""}`}
                            onClick={() => setSelectedVoiceID(voice.voice_id)}
                          >
                            <td className="px-4 py-3">
                              <div className="font-medium text-[var(--color-editorial-ink)]">{voice.name || voice.voice_id}</div>
                              <div className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{voice.voice_id}</div>
                            </td>
                            <td className="px-4 py-3 text-[var(--color-editorial-ink-soft)]">{voice.language || "—"}</td>
                            <td className="px-4 py-3 text-[var(--color-editorial-ink-soft)]">
                              {supportedModels.length > 0 ? supportedModels.join(", ") : "—"}
                            </td>
                            <td className="px-4 py-3 text-[var(--color-editorial-ink-soft)]">{voice.description || "—"}</td>
                          </tr>
                        );
                      })
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </div>

          <aside className="rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-5 shadow-[0_18px_48px_rgba(35,24,12,0.08)]">
            <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("openaiTTS.latestRun")}</div>
            <div className="mt-4 space-y-3 text-sm text-[var(--color-editorial-ink-soft)]">
              <div>{t("openaiTTS.fetched")} · {voiceCount}</div>
              <div>{t("openaiTTS.lastSynced")} · {formatDateTime(latestRunLabel)}</div>
              <div>{t("openaiTTS.syncStatus")} · {data?.latest_run?.status ?? "—"}</div>
            </div>

            {data?.latest_change_summary ? (
              <div className="mt-6 rounded-[22px] border border-[var(--color-editorial-line)] bg-white p-4">
                <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("openaiTTS.latestSummary.title")}</div>
                <div className="mt-3 space-y-2 text-sm text-[var(--color-editorial-ink-soft)]">
                  <div>{t("openaiTTS.latestSummary.added")} · {addedCount}</div>
                  <div>{t("openaiTTS.latestSummary.removed")} · {removedCount}</div>
                </div>
              </div>
            ) : null}

            <div className="mt-6 rounded-[22px] border border-[var(--color-editorial-line)] bg-white p-4">
              {selectedVoice ? (
                <div className="space-y-4">
                  <div>
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("openaiTTS.detail.title")}</div>
                    <h2 className="mt-2 text-lg font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.name || selectedVoice.voice_id}</h2>
                    <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{selectedVoice.description || t("openaiTTS.detail.noDescription")}</p>
                  </div>

                  <div className="grid gap-3 sm:grid-cols-2">
                    <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                      <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("openaiTTS.table.language")}</div>
                      <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.language || "—"}</div>
                    </div>
                    <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                      <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("openaiTTS.table.voice")}</div>
                      <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.voice_id}</div>
                    </div>
                  </div>

                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("openaiTTS.table.models")}</div>
                    <div className="mt-3 flex flex-wrap gap-2">
                      {getSupportedModels(selectedVoice).length > 0 ? (
                        getSupportedModels(selectedVoice).map((model) => (
                          <span
                            key={`${selectedVoice.voice_id}-${model}`}
                            className="rounded-full border border-[var(--color-editorial-line)] bg-white px-3 py-1 text-xs text-[var(--color-editorial-ink-soft)]"
                          >
                            {model}
                          </span>
                        ))
                      ) : (
                        <span className="text-sm text-[var(--color-editorial-ink-soft)]">—</span>
                      )}
                    </div>
                  </div>

                  {selectedVoice.preview_url ? (
                    <div>
                      <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("openaiTTS.preview")}</div>
                      <div className="mt-3 rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                        <audio controls preload="none" className="w-full" src={selectedVoice.preview_url} />
                      </div>
                    </div>
                  ) : null}
                </div>
              ) : (
                <div className="text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{t("openaiTTS.detail.emptySelection")}</div>
              )}
            </div>

            <div className="mt-4 flex justify-end">
              <button
                type="button"
                onClick={() => setSelectedVoiceID(null)}
                className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
              >
                <X className="size-4" />
                {t("common.clear")}
              </button>
            </div>
          </aside>
        </section>
      </div>
    </PageTransition>
  );
}
