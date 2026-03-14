"use client";

import type { LLMCatalogModel } from "@/lib/api";
import ModelGuideTable from "@/components/settings/model-guide-table";

export default function ModelGuideModal({
  open,
  onClose,
  entries,
  t,
}: {
  open: boolean;
  onClose: () => void;
  entries: LLMCatalogModel[];
  t: (key: string, fallback?: string) => string;
}) {
  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6">
      <div className="flex max-h-[90vh] w-full max-w-5xl flex-col overflow-hidden rounded-2xl border border-zinc-200 bg-white shadow-2xl">
        <div className="flex items-start justify-between gap-4 border-b border-zinc-200 px-5 py-4">
          <div>
            <h2 className="text-base font-semibold text-zinc-900">{t("settings.modelGuide.title")}</h2>
            <p className="mt-1 text-sm text-zinc-500">{t("settings.modelGuide.description")}</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-sm font-medium text-zinc-700 hover:border-zinc-400 hover:text-zinc-900"
          >
            {t("common.close")}
          </button>
        </div>
        <div className="overflow-auto px-5 py-4">
          <ModelGuideTable entries={entries} t={t} />
        </div>
      </div>
    </div>
  );
}
