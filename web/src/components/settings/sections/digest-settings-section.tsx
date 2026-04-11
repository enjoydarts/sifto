"use client";

import type { FormEvent } from "react";
import { SectionCard } from "@/components/ui/section-card";

type Translate = (key: string, fallback?: string) => string;

export default function DigestSettingsSection({
  t,
  form,
  state,
  actions,
}: {
  t: Translate;
  form: {
    onSubmit: (event: FormEvent<HTMLFormElement>) => void;
    saving: boolean;
  };
  state: {
    enabled: boolean;
  };
  actions: {
    onChangeEnabled: (value: boolean) => void;
  };
}) {
  const { onSubmit, saving } = form;
  const { enabled } = state;
  const { onChangeEnabled } = actions;

  return (
    <SectionCard>
      <form onSubmit={onSubmit} className="space-y-5">
        <div className="flex items-center justify-between gap-3 rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3">
          <div className="min-w-0">
            <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.digestEmailSending")}</div>
            <div className="mt-1 text-[12px] leading-5 text-[var(--color-editorial-ink-soft)]">{t("settings.digestDisabledHint")}</div>
          </div>
          <label className="inline-flex shrink-0 items-center gap-2 text-sm text-[var(--color-editorial-ink-soft)]">
            <input
              type="checkbox"
              checked={enabled}
              onChange={(e) => onChangeEnabled(e.target.checked)}
              className="size-4 rounded border-[var(--color-editorial-line-strong)]"
            />
            {enabled ? t("settings.on") : t("settings.off")}
          </label>
        </div>
        <button
          type="submit"
          disabled={saving}
          className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:opacity-60"
        >
          {saving ? t("common.saving") : t("settings.saveDelivery")}
        </button>
      </form>
    </SectionCard>
  );
}
