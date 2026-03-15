export type FeedMode = "unread" | "later" | "read" | "pending";
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
    { value: "pending", labelKey: "items.feed.pending" },
  ];

  return (
    <div className="grid w-full grid-cols-4 gap-1 rounded-lg border border-zinc-200 bg-white p-0.5">
      {tabs.map(({ value, labelKey }) => (
        <button
          key={value}
          type="button"
          onClick={() => onSelect(value)}
          aria-pressed={feedMode === value}
          className={`inline-flex min-h-8 items-center justify-center whitespace-nowrap rounded-md px-1.5 py-1.5 text-[13px] font-medium transition-all duration-150 press focus-ring sm:px-2.5 sm:text-sm ${
            feedMode === value
              ? "bg-zinc-900 text-white shadow-sm"
              : "text-zinc-600 hover:bg-zinc-50 hover:text-zinc-900"
          }`}
        >
          {t(labelKey)}
        </button>
      ))}
    </div>
  );
}
