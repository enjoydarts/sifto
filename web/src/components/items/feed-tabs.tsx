export type FeedMode = "unread" | "later" | "read";
export type SortMode = "newest" | "score" | "personal_score";

export function FeedTabs({
  feedMode,
  onSelect,
  t,
}: {
  feedMode: FeedMode;
  onSelect: (feed: FeedMode) => void;
  t: (key: string) => string;
}) {
  const tabs: { value: FeedMode; labelKey: string }[] = [
    { value: "unread", labelKey: "items.feed.unread" },
    { value: "later",  labelKey: "items.feed.later"  },
    { value: "read",   labelKey: "items.feed.read"   },
  ];

  return (
    <div className="flex items-center gap-1 rounded-lg border border-zinc-200 bg-white p-1">
      {tabs.map(({ value, labelKey }) => (
        <button
          key={value}
          type="button"
          onClick={() => onSelect(value)}
          className={`rounded px-3 py-1.5 text-xs font-medium transition-colors press focus-ring ${
            feedMode === value
              ? "bg-zinc-900 text-white"
              : "text-zinc-600 hover:bg-zinc-50"
          }`}
        >
          {t(labelKey)}
        </button>
      ))}
    </div>
  );
}
