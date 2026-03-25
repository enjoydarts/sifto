"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { RefreshCw, Search, X } from "lucide-react";
import { AivisModelSnapshot, AivisModelSpeaker, AivisModelSpeakerStyle } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";

type AivisVoiceOption = {
  key: string;
  model: AivisModelSnapshot;
  speaker: AivisModelSpeaker;
  style: AivisModelSpeakerStyle;
};

function flattenVoiceOptions(models: AivisModelSnapshot[]): AivisVoiceOption[] {
  return models.flatMap((model) =>
    (model.speakers_json ?? []).flatMap((speaker) =>
      (speaker.styles ?? []).map((style) => ({
        key: `${model.aivm_model_uuid}|${speaker.aivm_speaker_uuid}:${style.local_id}`,
        model,
        speaker,
        style,
      }))
    )
  );
}

export default function AivisVoicePickerModal({
  open,
  loading,
  syncing,
  error,
  models,
  currentVoiceModel,
  currentVoiceStyle,
  onClose,
  onSync,
  onSelect,
}: {
  open: boolean;
  loading: boolean;
  syncing: boolean;
  error: string | null;
  models: AivisModelSnapshot[];
  currentVoiceModel: string;
  currentVoiceStyle: string;
  onClose: () => void;
  onSync: () => Promise<void> | void;
  onSelect: (selection: { voice_model: string; voice_style: string }) => void;
}) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const [selectedKey, setSelectedKey] = useState<string | null>(null);

  const options = useMemo(() => flattenVoiceOptions(models), [models]);

  const selected = useMemo(() => {
    const activeKey = selectedKey ?? `${currentVoiceModel}|${currentVoiceStyle}`;
    return options.find((option) => option.key === activeKey) ?? null;
  }, [currentVoiceModel, currentVoiceStyle, options, selectedKey]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return options;
    return options.filter((option) =>
      [
        option.model.name,
        option.model.description,
        option.model.category,
        option.model.voice_timbre,
        option.speaker.name,
        option.style.name,
      ]
        .join(" ")
        .toLowerCase()
        .includes(q)
    );
  }, [options, query]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6" onClick={onClose}>
      <div
        className="flex max-h-[92vh] w-full max-w-6xl flex-col overflow-hidden rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] shadow-[0_30px_80px_rgba(35,24,12,0.24)]"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex flex-wrap items-start justify-between gap-4 border-b border-[var(--color-editorial-line)] px-5 py-4">
          <div>
            <h2 className="text-base font-semibold text-[var(--color-editorial-ink)]">{t("aivisModels.picker.title")}</h2>
            <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("aivisModels.picker.subtitle")}</p>
          </div>
          <div className="flex items-center gap-2">
            <Link
              href="/aivis-models"
              className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
            >
              {t("aivisModels.picker.openAdmin")}
            </Link>
            <button
              type="button"
              onClick={() => void onSync()}
              disabled={syncing}
              className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:opacity-60"
            >
              <RefreshCw className={`size-4 ${syncing ? "animate-spin" : ""}`} />
              {syncing ? t("aivisModels.syncing") : t("aivisModels.sync")}
            </button>
            <button
              type="button"
              onClick={onClose}
              className="inline-flex size-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-white text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
              aria-label={t("common.close")}
            >
              <X className="size-4" />
            </button>
          </div>
        </div>

        <div className="border-b border-[var(--color-editorial-line)] px-5 py-4">
          <div className="flex items-center gap-3 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-3">
            <Search className="size-4 text-[var(--color-editorial-ink-soft)]" />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t("aivisModels.search")}
              className="w-full bg-transparent text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
            />
          </div>
          {error ? <p className="mt-3 text-sm text-red-600">{error}</p> : null}
        </div>

        <div className="grid min-h-0 flex-1 gap-0 lg:grid-cols-[minmax(0,1.15fr)_minmax(320px,0.85fr)]">
          <div className="min-h-0 overflow-auto border-b border-[var(--color-editorial-line)] lg:border-b-0 lg:border-r">
            <div className="overflow-x-auto">
              <table className="min-w-[840px] divide-y divide-[var(--color-editorial-line)] text-sm">
                <thead className="bg-[var(--color-editorial-panel-strong)]">
                  <tr className="text-left text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">
                    <th className="px-4 py-3">{t("aivisModels.table.model")}</th>
                    <th className="px-4 py-3">{t("aivisModels.table.speaker")}</th>
                    <th className="px-4 py-3">{t("aivisModels.table.style")}</th>
                    <th className="px-4 py-3">{t("aivisModels.table.timbre")}</th>
                    <th className="px-4 py-3 text-right">{t("aivisModels.table.downloads")}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--color-editorial-line)] bg-white">
                  {loading ? (
                    <tr>
                      <td colSpan={5} className="px-4 py-8 text-center text-sm text-[var(--color-editorial-ink-soft)]">
                        {t("common.loading")}
                      </td>
                    </tr>
                  ) : filtered.length === 0 ? (
                    <tr>
                      <td colSpan={5} className="px-4 py-8 text-center text-sm text-[var(--color-editorial-ink-soft)]">
                        {t("aivisModels.picker.noResults")}
                      </td>
                    </tr>
                  ) : (
                    filtered.map((option) => (
                      <tr
                        key={option.key}
                        className={`cursor-pointer transition hover:bg-[var(--color-editorial-panel)] ${selected?.key === option.key ? "bg-[var(--color-editorial-panel)]" : ""}`}
                        onClick={() => setSelectedKey(option.key)}
                      >
                        <td className="px-4 py-3">
                          <div className="font-medium text-[var(--color-editorial-ink)]">{option.model.name}</div>
                          <div className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{option.model.category}</div>
                        </td>
                        <td className="px-4 py-3 text-[var(--color-editorial-ink)]">{option.speaker.name}</td>
                        <td className="px-4 py-3 text-[var(--color-editorial-ink)]">{option.style.name}</td>
                        <td className="px-4 py-3 text-[var(--color-editorial-ink-soft)]">{option.model.voice_timbre}</td>
                        <td className="px-4 py-3 text-right text-[var(--color-editorial-ink-soft)]">
                          {new Intl.NumberFormat().format(option.model.total_download_count)}
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>

          <div className="min-h-0 overflow-auto px-5 py-5">
            {selected ? (
              <div className="space-y-5">
                <div>
                  <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("aivisModels.table.model")}</div>
                  <h3 className="mt-2 text-lg font-semibold text-[var(--color-editorial-ink)]">{selected.model.name}</h3>
                  <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{selected.model.description}</p>
                </div>

                <div className="grid gap-3 sm:grid-cols-2">
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("aivisModels.table.speaker")}</div>
                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">{selected.speaker.name}</div>
                  </div>
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("aivisModels.table.style")}</div>
                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">{selected.style.name}</div>
                  </div>
                </div>

                {(selected.style.voice_samples ?? []).length > 0 ? (
                  <div>
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("aivisModels.modal.voiceSamples")}</div>
                    <div className="mt-3 space-y-3">
                      {selected.style.voice_samples.slice(0, 2).map((sample, index) => (
                        <div key={`${selected.key}-sample-${index}`} className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                          <audio controls preload="none" className="w-full" src={sample.audio_url} />
                          <p className="mt-3 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{sample.transcript}</p>
                        </div>
                      ))}
                    </div>
                  </div>
                ) : null}

                <div className="flex justify-end">
                  <button
                    type="button"
                    onClick={() => {
                      onSelect({
                        voice_model: selected.model.aivm_model_uuid,
                        voice_style: `${selected.speaker.aivm_speaker_uuid}:${selected.style.local_id}`,
                      });
                      onClose();
                    }}
                    className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90"
                  >
                    {t("aivisModels.picker.select")}
                  </button>
                </div>
              </div>
            ) : (
              <div className="rounded-[22px] border border-dashed border-[var(--color-editorial-line)] bg-white/70 px-5 py-8 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                {t("aivisModels.picker.emptySelection")}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
