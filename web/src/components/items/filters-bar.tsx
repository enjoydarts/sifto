import { ArrowDownAZ, Sparkles, Star, TrendingUp } from "lucide-react";
import { type SortMode } from "./feed-tabs";

export function FiltersBar({
  feedMode,
  sortMode,
  favoriteOnly,
  toolbarAction,
  bulkMarkingRead,
  onSortChange,
  onFavoriteChange,
  onToolbarActionChange,
  onToolbarRun,
  t,
}: {
  feedMode: string;
  sortMode: SortMode;
  favoriteOnly: boolean;
  toolbarAction: "" | "triage_all" | "bulk_filtered" | "bulk_older";
  bulkMarkingRead: boolean;
  onSortChange: (sort: SortMode) => void;
  onFavoriteChange: (v: boolean) => void;
  onToolbarActionChange: (v: "" | "triage_all" | "bulk_filtered" | "bulk_older") => void;
  onToolbarRun: () => void;
  t: (key: string) => string;
}) {
  const focusMode = feedMode === "recommended";
  const pendingMode = feedMode === "pending";
  const showPrimaryRow = !focusMode && !pendingMode;
  const sortIcons: Record<SortMode, typeof ArrowDownAZ> = {
    newest: ArrowDownAZ,
    score: TrendingUp,
    personal_score: Sparkles,
  };

  return (
    <div className="flex flex-col gap-2">
      {showPrimaryRow && (
        <div className="flex min-w-0 max-w-full flex-wrap gap-2 sm:pb-0">
          <div className="flex min-w-0 items-center gap-1 rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-1">
            {(["newest", "score", "personal_score"] as SortMode[]).map((s) => (
              (() => {
                const Icon = sortIcons[s];
                const active = sortMode === s;
                return (
                  <button
                    key={s}
                    type="button"
                    onClick={() => onSortChange(s)}
                    title={t(`items.sort.${s}`)}
                    aria-label={t(`items.sort.${s}`)}
                    aria-pressed={active}
                    className={`inline-flex h-9 w-9 items-center justify-center rounded-[12px] transition-colors press focus-ring ${
                      active
                        ? "bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                        : "text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] hover:text-[var(--color-editorial-ink)]"
                    }`}
                  >
                    <Icon className="size-4" aria-hidden="true" />
                  </button>
                );
              })()
            ))}
          </div>

          <label
            title={t("items.filter.favoriteOnly")}
            className={`inline-flex h-11 w-11 shrink-0 cursor-pointer items-center justify-center rounded-full border transition-colors ${
              favoriteOnly
                ? "border-[#d7b4a9] bg-[var(--color-editorial-accent-soft)] text-[var(--color-editorial-accent)]"
                : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
            }`}
          >
            <input
              type="checkbox"
              checked={favoriteOnly}
              onChange={(e) => onFavoriteChange(e.target.checked)}
              aria-label={t("items.filter.favoriteOnly")}
              className="sr-only"
            />
            <Star className={`size-4 ${favoriteOnly ? "fill-current" : "text-amber-600"}`} aria-hidden="true" />
          </label>

          <div className="flex min-w-0 flex-1 items-center gap-2 xl:hidden">
            <select
              value={toolbarAction}
              onChange={(e) => onToolbarActionChange(e.target.value as typeof toolbarAction)}
              className="min-h-11 min-w-0 flex-1 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2 text-sm text-[var(--color-editorial-ink-soft)] focus-ring"
              aria-label={t("items.toolbar.actions")}
            >
              <option value="">{t("items.actions.placeholder")}</option>
              <option value="bulk_filtered">{t("items.bulkRead.filtered")}</option>
              <option value="bulk_older">{t("items.bulkRead.olderThan7d")}</option>
            </select>
            <button
              type="button"
              disabled={!toolbarAction || bulkMarkingRead}
              onClick={onToolbarRun}
              className="inline-flex min-h-11 shrink-0 items-center justify-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:cursor-not-allowed disabled:opacity-50"
            >
              {bulkMarkingRead ? t("common.saving") : t("items.actions.run")}
            </button>
          </div>
        </div>
      )}

    </div>
  );
}
