import { Star } from "lucide-react";
import { type SortMode } from "./feed-tabs";

export function FiltersBar({
  feedMode,
  sortMode,
  topic,
  favoriteOnly,
  onSortChange,
  onTopicClear,
  onFavoriteChange,
  t,
}: {
  feedMode: string;
  sortMode: SortMode;
  topic: string;
  favoriteOnly: boolean;
  onSortChange: (sort: SortMode) => void;
  onTopicClear: () => void;
  onFavoriteChange: (v: boolean) => void;
  t: (key: string) => string;
}) {
  const focusMode = feedMode === "recommended";
  const pendingMode = feedMode === "pending";
  const showPrimaryRow = !focusMode && !pendingMode;

  return (
    <div className="flex flex-col gap-2">
      {showPrimaryRow && (
        <div className="flex min-w-0 items-center gap-2">
          <div className="flex min-w-0 flex-1 items-center gap-1 rounded-lg border border-zinc-200 bg-white p-0.5">
          {(["newest", "score", "personal_score"] as SortMode[]).map((s) => (
            <button
              key={s}
              type="button"
              onClick={() => onSortChange(s)}
              className={`min-w-0 flex-1 rounded-md px-2 py-1.5 text-[11px] font-medium transition-colors press focus-ring sm:px-2.5 sm:text-xs ${
                sortMode === s
                  ? "bg-zinc-900 text-white"
                  : "text-zinc-600 hover:bg-zinc-50 hover:text-zinc-900"
              }`}
            >
              {t(`items.sort.${s}`)}
            </button>
          ))}
          </div>

          <label className="inline-flex shrink-0 cursor-pointer items-center gap-2 whitespace-nowrap rounded-full border border-zinc-200 bg-white px-2.5 py-1 text-xs text-zinc-700 hover:bg-zinc-50 transition-colors">
            <input
              type="checkbox"
              checked={favoriteOnly}
              onChange={(e) => onFavoriteChange(e.target.checked)}
              className="size-4 rounded border-zinc-300"
            />
            <span className="inline-flex items-center gap-1">
              <Star className="size-3.5 text-amber-500" aria-hidden="true" />
              <span className="hidden whitespace-nowrap sm:inline">{t("items.filter.favoriteOnly")}</span>
            </span>
          </label>
        </div>
      )}

      {!focusMode && topic && (
        <div className="inline-flex max-w-full items-center gap-2 self-start rounded-full border border-blue-200 bg-blue-50 px-2.5 py-1 text-xs text-blue-800">
          <span className="truncate font-medium">{t("items.topic")}: {topic}</span>
          <button
            type="button"
            onClick={onTopicClear}
            className="rounded px-1.5 py-0.5 text-xs text-blue-700 hover:bg-blue-100 press"
          >
            {t("items.clear")}
          </button>
        </div>
      )}
    </div>
  );
}
