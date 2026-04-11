"use client";

import type { FormEvent } from "react";
import { SectionCard } from "@/components/ui/section-card";

type Translate = (key: string, fallback?: string) => string;

export default function BudgetSettingsSection({
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
    budgetUSD: string;
    alertEnabled: boolean;
    thresholdPct: number;
    budgetRemainingTone: string;
    monthJst: string;
  };
  actions: {
    onChangeBudgetUSD: (value: string) => void;
    onChangeAlertEnabled: (value: boolean) => void;
    onChangeThresholdPct: (value: number) => void;
  };
}) {
  const { onSubmit, saving } = form;
  const { budgetUSD, alertEnabled, thresholdPct, budgetRemainingTone, monthJst } = state;
  const { onChangeBudgetUSD, onChangeAlertEnabled, onChangeThresholdPct } = actions;

  return (
    <SectionCard>
      <form onSubmit={onSubmit} className="space-y-5">
        <div className="grid gap-4 lg:grid-cols-[minmax(0,240px)_minmax(0,1fr)] lg:gap-6">
          <div>
            <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.monthlyBudgetUsd")}</div>
            <p className="mt-1 text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.controlRoom.monthlyBudgetHelp")}</p>
          </div>
          <input
            type="number"
            min={0}
            step="0.01"
            value={budgetUSD}
            onChange={(e) => onChangeBudgetUSD(e.target.value)}
            placeholder={t("settings.budgetPlaceholder")}
            className="w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
          />
        </div>
        <div className="grid gap-4 border-t border-[var(--color-editorial-line)] pt-5 lg:grid-cols-[minmax(0,240px)_minmax(0,1fr)] lg:gap-6">
          <div>
            <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.budgetAlertEmail")}</div>
            <p className="mt-1 text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.budgetAlertHint")}</p>
          </div>
          <label className="flex items-center justify-between gap-3 rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink-soft)]">
            <span>{alertEnabled ? t("settings.on") : t("settings.off")}</span>
            <input
              type="checkbox"
              checked={alertEnabled}
              onChange={(e) => onChangeAlertEnabled(e.target.checked)}
              className="size-4 rounded border-[var(--color-editorial-line-strong)]"
            />
          </label>
        </div>
        <div className="grid gap-4 border-t border-[var(--color-editorial-line)] pt-5 lg:grid-cols-[minmax(0,240px)_minmax(0,1fr)] lg:gap-6">
          <div>
            <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.alertThreshold")}</div>
            <p className="mt-1 text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.controlRoom.thresholdHelp")}</p>
          </div>
          <div className="rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3">
            <div className="flex items-center gap-3">
              <input
                type="range"
                min={1}
                max={99}
                value={thresholdPct}
                onChange={(e) => onChangeThresholdPct(Number(e.target.value))}
                className="w-full accent-[var(--color-editorial-ink)]"
              />
              <span className={`w-12 text-right text-sm font-medium ${budgetRemainingTone}`}>{thresholdPct}%</span>
            </div>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <button
            type="submit"
            disabled={saving}
            className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:opacity-60"
          >
            {saving ? t("common.saving") : t("settings.saveBudget")}
          </button>
          <span className="text-xs text-[var(--color-editorial-ink-faint)]">
            {`${t("settings.currentMonth")}: ${monthJst}`}
          </span>
        </div>
      </form>
    </SectionCard>
  );
}
