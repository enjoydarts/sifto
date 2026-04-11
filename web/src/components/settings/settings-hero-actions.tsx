"use client";

import { ChevronDown } from "lucide-react";

export default function SettingsHeroActions({
  costPerformanceLabel,
  extrasLabel,
  extrasOpen,
  onApplyCostPerformancePreset,
  onToggleExtras,
}: {
  costPerformanceLabel: string;
  extrasLabel: string;
  extrasOpen: boolean;
  onApplyCostPerformancePreset: () => void;
  onToggleExtras: () => void;
}) {
  return (
    <div className="flex flex-wrap gap-2">
      <button
        type="button"
        onClick={onApplyCostPerformancePreset}
        className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90 press focus-ring"
      >
        {costPerformanceLabel}
      </button>
      <button
        type="button"
        onClick={onToggleExtras}
        className="inline-flex min-h-10 items-center gap-1 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] press focus-ring"
      >
        {extrasLabel}
        <ChevronDown className={`size-3 transition-transform ${extrasOpen ? "rotate-180" : ""}`} />
      </button>
    </div>
  );
}
