"use client";

import type { FormEvent } from "react";
import { SectionCard } from "@/components/ui/section-card";

type Translate = (key: string, fallback?: string) => string;

export default function ReadingPlanSettingsSection({
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
    window: "24h" | "today_jst" | "7d";
    size: string;
    diversifyTopics: boolean;
  };
  actions: {
    onChangeWindow: (value: "24h" | "today_jst" | "7d") => void;
    onChangeSize: (value: string) => void;
    onChangeDiversifyTopics: (value: boolean) => void;
  };
}) {
  const { onSubmit, saving } = form;
  const { window, size, diversifyTopics } = state;
  const { onChangeWindow, onChangeSize, onChangeDiversifyTopics } = actions;

  return (
    <SectionCard>
      <form onSubmit={onSubmit} className="space-y-5">
        <div className="grid gap-4 lg:grid-cols-[minmax(0,240px)_minmax(0,1fr)] lg:gap-6">
          <div>
            <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.window")}</div>
            <p className="mt-1 text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.controlRoom.windowHelp")}</p>
          </div>
          <select
            value={window}
            onChange={(e) => onChangeWindow(e.target.value as "24h" | "today_jst" | "7d")}
            className="w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)]"
          >
            <option value="24h">{t("settings.window.24h")}</option>
            <option value="today_jst">{t("settings.window.today")}</option>
            <option value="7d">{t("settings.window.7d")}</option>
          </select>
        </div>
        <div className="grid gap-4 border-t border-[var(--color-editorial-line)] pt-5 lg:grid-cols-[minmax(0,240px)_minmax(0,1fr)] lg:gap-6">
          <div>
            <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.size")}</div>
            <p className="mt-1 text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.controlRoom.sizeHelp")}</p>
          </div>
          <select
            value={size}
            onChange={(e) => onChangeSize(e.target.value)}
            className="w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)]"
          >
            {[7, 15, 25].map((n) => (
              <option key={n} value={String(n)}>
                {n}
              </option>
            ))}
          </select>
        </div>
        <div className="grid gap-4 border-t border-[var(--color-editorial-line)] pt-5 lg:grid-cols-[minmax(0,240px)_minmax(0,1fr)] lg:gap-6">
          <div>
            <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.diversifyTopics")}</div>
            <p className="mt-1 text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.controlRoom.diversifyHelp")}</p>
          </div>
          <label className="flex items-center justify-between gap-3 rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink-soft)]">
            <span>{diversifyTopics ? t("settings.controlRoom.topicBalanceOn") : t("settings.controlRoom.topicBalanceOff")}</span>
            <input
              type="checkbox"
              checked={diversifyTopics}
              onChange={(e) => onChangeDiversifyTopics(e.target.checked)}
              className="size-4 rounded border-[var(--color-editorial-line-strong)]"
            />
          </label>
        </div>
        <button
          type="submit"
          disabled={saving}
          className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:opacity-60"
        >
          {saving ? t("common.saving") : t("settings.saveRecommended")}
        </button>
      </form>
    </SectionCard>
  );
}
