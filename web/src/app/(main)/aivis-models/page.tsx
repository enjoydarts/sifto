"use client";

import { useMemo, useState } from "react";
import { RefreshCw, Search, X } from "lucide-react";
import { AivisModelSnapshot, AivisModelsResponse, api } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { PageTransition } from "@/components/page-transition";
import { PageHeader } from "@/components/ui/page-header";
import { useModelCatalog, formatDateTime, formatNumber } from "@/components/model-catalog/use-model-catalog";

type AivisSection = "models" | "removed";

function userName(model: AivisModelSnapshot) {
  const user = model.user_json ?? {};
  const displayName = typeof user.name === "string" && user.name.trim() ? user.name.trim() : null;
  const handle = typeof user.handle === "string" && user.handle.trim() ? user.handle.trim() : null;
  return displayName ?? handle ?? "—";
}

export default function AivisModelsPage() {
  const { t } = useI18n();
  const [activeSection, setActiveSection] = useState<AivisSection>("models");
  const [selectedModel, setSelectedModel] = useState<AivisModelSnapshot | null>(null);

  const { loading, syncing, error, data, query, setQuery, handleSync } = useModelCatalog<AivisModelsResponse>({
    fetchData: () => api.getAivisModels(),
    syncData: () => api.syncAivisModels(),
    syncSuccessKey: "aivisModels.syncCompleted",
  });

  const filteredModels = useMemo(() => {
    const q = query.trim().toLowerCase();
    const models = activeSection === "removed" ? data?.removed_models ?? [] : data?.models ?? [];
    if (!q) return models;
    return models.filter((model) =>
      [model.name, model.description, model.detailed_description, model.category, model.voice_timbre, userName(model), ...(model.tags_json ?? []).map((tag) => tag.name), ...(model.speakers_json ?? []).map((speaker) => speaker.name)].join(" ").toLowerCase().includes(q)
    );
  }, [activeSection, data?.models, data?.removed_models, query]);

  const latestRunLabel = data?.latest_run?.finished_at ?? data?.latest_run?.started_at;
  const modelCount = data?.models?.length ?? 0;
  const removedCount = data?.removed_models?.length ?? 0;

  return (
    <PageTransition>
      <div className="space-y-6">
        <PageHeader
          title={t("aivisModels.title")}
          description={t("aivisModels.subtitle")}
          actions={
            <button type="button" onClick={handleSync} disabled={syncing} className="inline-flex min-h-11 items-center justify-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] disabled:opacity-60">
              <RefreshCw className={`size-4 ${syncing ? "animate-spin" : ""}`} aria-hidden="true" />
              {syncing ? t("aivisModels.syncing") : t("aivisModels.sync")}
            </button>
          }
        />

        <section className="grid gap-4 lg:grid-cols-[minmax(0,1.35fr)_minmax(320px,0.65fr)]">
          <div className="rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-5 shadow-[0_18px_48px_rgba(35,24,12,0.08)]">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("aivisModels.searchTitle")}</div>
                <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("aivisModels.searchDescription")}</p>
              </div>
              <div className="flex rounded-full border border-[var(--color-editorial-line)] bg-white p-1">
                {(["models", "removed"] as AivisSection[]).map((section) => (
                  <button key={section} type="button" onClick={() => setActiveSection(section)} className={`rounded-full px-4 py-2 text-sm font-medium transition ${activeSection === section ? "bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]" : "text-[var(--color-editorial-ink-soft)]"}`}>
                    {section === "models" ? t("aivisModels.table.availableModels") : t("aivisModels.table.removedModels")}
                  </button>
                ))}
              </div>
            </div>

            <div className="mt-5 flex items-center gap-3 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-3">
              <Search className="size-4 text-[var(--color-editorial-ink-soft)]" />
              <input value={query} onChange={(e) => setQuery(e.target.value)} placeholder={t("aivisModels.search")} className="w-full bg-transparent text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]" />
            </div>

            {error ? <p className="mt-4 text-sm text-red-600">{error}</p> : null}

            <div className="mt-5 overflow-hidden rounded-[24px] border border-[var(--color-editorial-line)] bg-white">
              <div className="overflow-x-auto">
                <table className="min-w-[920px] divide-y divide-[var(--color-editorial-line)] text-sm">
                  <thead className="bg-[var(--color-editorial-panel-strong)]">
                    <tr className="text-left text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">
                      <th className="px-4 py-3">{t("aivisModels.table.model")}</th>
                      <th className="px-4 py-3">{t("aivisModels.table.author")}</th>
                      <th className="px-4 py-3">{t("aivisModels.table.timbre")}</th>
                      <th className="px-4 py-3 text-right">{t("aivisModels.table.speakers")}</th>
                      <th className="px-4 py-3 text-right">{t("aivisModels.table.styles")}</th>
                      <th className="px-4 py-3 text-right">{t("aivisModels.table.downloads")}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[var(--color-editorial-line)] bg-white">
                    {loading ? (
                      <tr><td colSpan={6} className="px-4 py-8 text-center text-sm text-[var(--color-editorial-ink-soft)]">{t("common.loading")}</td></tr>
                    ) : filteredModels.length === 0 ? (
                      <tr><td colSpan={6} className="px-4 py-8 text-center text-sm text-[var(--color-editorial-ink-soft)]">{activeSection === "removed" ? t("aivisModels.noRemovedModels") : t("aivisModels.noModels")}</td></tr>
                    ) : (
                      filteredModels.map((model) => (
                        <tr key={`${activeSection}-${model.aivm_model_uuid}`} className="cursor-pointer transition hover:bg-[var(--color-editorial-panel)]" onClick={() => setSelectedModel(model)}>
                          <td className="px-4 py-3">
                            <div className="font-medium text-[var(--color-editorial-ink)]">{model.name}</div>
                            <div className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{model.category}</div>
                          </td>
                          <td className="px-4 py-3 text-[var(--color-editorial-ink-soft)]">{userName(model)}</td>
                          <td className="px-4 py-3 text-[var(--color-editorial-ink-soft)]">{model.voice_timbre}</td>
                          <td className="px-4 py-3 text-right text-[var(--color-editorial-ink-soft)]">{formatNumber(model.speaker_count)}</td>
                          <td className="px-4 py-3 text-right text-[var(--color-editorial-ink-soft)]">{formatNumber(model.style_count)}</td>
                          <td className="px-4 py-3 text-right text-[var(--color-editorial-ink-soft)]">{formatNumber(model.total_download_count)}</td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </div>

          <aside className="rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-5 shadow-[0_18px_48px_rgba(35,24,12,0.08)]">
            <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("aivisModels.latestRun")}</div>
            <div className="mt-4 space-y-3 text-sm text-[var(--color-editorial-ink-soft)]">
              <div>{t("aivisModels.fetched")} · {modelCount}</div>
              <div>{t("aivisModels.removed")} · {removedCount}</div>
              <div>{t("aivisModels.lastSynced")} · {formatDateTime(latestRunLabel)}</div>
              <div>{t("aivisModels.syncStatus")} · {data?.latest_run?.status ?? "—"}</div>
            </div>
            {data?.latest_change_summary ? (
              <div className="mt-6 rounded-[22px] border border-[var(--color-editorial-line)] bg-white p-4">
                <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("aivisModels.latestSummary.title")}</div>
                <div className="mt-3 space-y-2 text-sm text-[var(--color-editorial-ink-soft)]">
                  <div>{t("aivisModels.latestSummary.added")} · {data.latest_change_summary.added.length}</div>
                  <div>{t("aivisModels.latestSummary.removed")} · {data.latest_change_summary.removed.length}</div>
                </div>
              </div>
            ) : null}
          </aside>
        </section>

        {selectedModel ? (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6" onClick={() => setSelectedModel(null)}>
            <div className="flex max-h-[92vh] w-full max-w-5xl flex-col overflow-hidden rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] shadow-[0_30px_80px_rgba(35,24,12,0.24)]" onClick={(event) => event.stopPropagation()}>
              <div className="flex items-start justify-between gap-4 border-b border-[var(--color-editorial-line)] px-5 py-4">
                <div>
                  <h2 className="text-lg font-semibold text-[var(--color-editorial-ink)]">{selectedModel.name}</h2>
                  <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{selectedModel.description}</p>
                </div>
                <button type="button" onClick={() => setSelectedModel(null)} className="inline-flex size-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-white text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]" aria-label={t("common.close")}>
                  <X className="size-4" />
                </button>
              </div>
              <div className="grid min-h-0 flex-1 gap-5 overflow-auto px-5 py-5 lg:grid-cols-[minmax(0,1fr)_minmax(300px,0.9fr)]">
                <div className="space-y-5">
                  <section className="rounded-[22px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("aivisModels.modal.detailedDescription")}</div>
                    <p className="mt-3 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{selectedModel.detailed_description || selectedModel.description}</p>
                  </section>
                  <section className="rounded-[22px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("aivisModels.modal.tags")}</div>
                    <div className="mt-3 flex flex-wrap gap-2">
                      {(selectedModel.tags_json ?? []).length > 0 ? (
                        selectedModel.tags_json.map((tag) => (<span key={`${selectedModel.aivm_model_uuid}-${tag.name}`} className="rounded-full border border-[var(--color-editorial-line)] px-3 py-1 text-xs text-[var(--color-editorial-ink-soft)]">{tag.name}</span>))
                      ) : (<span className="text-sm text-[var(--color-editorial-ink-soft)]">{t("aivisModels.modal.noTags")}</span>)}
                    </div>
                  </section>
                  <section className="rounded-[22px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("aivisModels.modal.speakers")}</div>
                    <div className="mt-4 space-y-4">
                      {(selectedModel.speakers_json ?? []).map((speaker) => (
                        <div key={speaker.aivm_speaker_uuid} className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                          <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{speaker.name}</div>
                          <div className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{speaker.supported_languages.join(", ") || "—"}</div>
                          <div className="mt-3 flex flex-wrap gap-2">
                            {speaker.styles.map((style) => (<span key={`${speaker.aivm_speaker_uuid}-${style.local_id}`} className="rounded-full border border-[var(--color-editorial-line)] bg-white px-3 py-1 text-xs text-[var(--color-editorial-ink-soft)]">{style.name} · #{style.local_id}</span>))}
                          </div>
                        </div>
                      ))}
                    </div>
                  </section>
                </div>
                <div className="space-y-5">
                  <section className="rounded-[22px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("aivisModels.modal.meta")}</div>
                    <div className="mt-4 space-y-3 text-sm text-[var(--color-editorial-ink-soft)]">
                      <div>{t("aivisModels.table.author")} · {userName(selectedModel)}</div>
                      <div>{t("aivisModels.table.timbre")} · {selectedModel.voice_timbre}</div>
                      <div>{t("aivisModels.table.downloads")} · {formatNumber(selectedModel.total_download_count)}</div>
                      <div>{t("aivisModels.modal.likes")} · {formatNumber(selectedModel.like_count)}</div>
                      <div>{t("aivisModels.modal.visibility")} · {selectedModel.visibility}</div>
                      <div>{t("aivisModels.modal.createdAt")} · {formatDateTime(selectedModel.created_at)}</div>
                      <div>{t("aivisModels.modal.updatedAt")} · {formatDateTime(selectedModel.updated_at)}</div>
                    </div>
                  </section>
                  <section className="rounded-[22px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("aivisModels.modal.modelFiles")}</div>
                    <div className="mt-4 space-y-3">
                      {(selectedModel.model_files_json ?? []).map((file) => (
                        <div key={`${selectedModel.aivm_model_uuid}-${file.version}-${file.checksum}`} className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4 text-sm text-[var(--color-editorial-ink-soft)]">
                          <div className="font-semibold text-[var(--color-editorial-ink)]">{file.name}</div>
                          <div className="mt-1">{file.model_architecture} / {file.model_format}</div>
                          <div className="mt-1">{t("aivisModels.modal.version")} · {file.version}</div>
                          <div className="mt-1">{t("aivisModels.modal.fileDownloads")} · {formatNumber(file.download_count)}</div>
                        </div>
                      ))}
                    </div>
                  </section>
                </div>
              </div>
            </div>
          </div>
        ) : null}
      </div>
    </PageTransition>
  );
}
