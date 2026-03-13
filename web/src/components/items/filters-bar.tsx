import { Star } from "lucide-react";
import { type SortMode } from "./feed-tabs";

export function FiltersBar({
  feedMode,
  sortMode,
  topic,
  unreadOnly,
  favoriteOnly,
  onSortChange,
  onTopicClear,
  onUnreadChange,
  onFavoriteChange,
  t,
}: {
  feedMode: string;
  sortMode: SortMode;
  topic: string;
  unreadOnly: boolean;
  favoriteOnly: boolean;
  onSortChange: (sort: SortMode) => void;
  onTopicClear: () => void;
  onUnreadChange: (v: boolean) => void;
  onFavoriteChange: (v: boolean) => void;
  t: (key: string) => string;
}) {
  const focusMode = feedMode === "recommended";

  return (
    <div className="flex flex-wrap items-center gap-2">
      {!focusMode && (
        <div className="flex items-center gap-1 rounded-lg border border-zinc-200 bg-white p-1">
          {(["newest", "score", "personal_score"] as SortMode[]).map((s) => (
            <button
              key={s}
              type="button"
              onClick={() => onSortChange(s)}
              className={`rounded px-3 py-1.5 text-xs font-medium transition-colors press focus-ring ${
                sortMode === s ? "bg-zinc-900 text-white" : "text-zinc-600 hover:bg-zinc-50"
              }`}
            >
              {t(`items.sort.${s}`)}
            </button>
          ))}
        </div>
      )}

      {!focusMode && topic && (
        <div className="inline-flex items-center gap-2 rounded border border-blue-200 bg-blue-50 px-3 py-1 text-sm text-blue-800">
          <span className="font-medium">{t("items.topic")}: {topic}</span>
          <button
            type="button"
            onClick={onTopicClear}
            className="rounded px-1.5 py-0.5 text-xs text-blue-700 hover:bg-blue-100 press"
          >
            {t("items.clear")}
          </button>
        </div>
      )}

      {!focusMode && (
        <label className="inline-flex cursor-pointer items-center gap-2 rounded border border-zinc-200 bg-white px-3 py-1 text-sm text-zinc-700 hover:bg-zinc-50 transition-colors">
          <input
            type="checkbox"
            checked={unreadOnly}
            onChange={(e) => onUnreadChange(e.target.checked)}
            className="size-4 rounded border-zinc-300"
          />
          {t("items.filter.unreadOnly")}
        </label>
      )}

      {!focusMode && (
        <label className="inline-flex cursor-pointer items-center gap-2 rounded border border-zinc-200 bg-white px-3 py-1 text-sm text-zinc-700 hover:bg-zinc-50 transition-colors">
          <input
            type="checkbox"
            checked={favoriteOnly}
            onChange={(e) => onFavoriteChange(e.target.checked)}
            className="size-4 rounded border-zinc-300"
          />
          <span className="inline-flex items-center gap-1">
            <Star className="size-3.5 text-amber-500" aria-hidden="true" />
            {t("items.filter.favoriteOnly")}
          </span>
        </label>
      )}
    </div>
  );
}
