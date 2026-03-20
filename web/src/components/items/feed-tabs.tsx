export type FeedMode = "unread" | "later" | "read" | "pending" | "deleted";
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
    { value: "deleted", labelKey: "items.filter.deleted" },
  ];

  return (
    <div className="flex w-full gap-1 overflow-x-auto rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-1 sm:grid sm:grid-cols-[0.95fr_1.2fr_0.9fr_1fr_0.95fr] sm:overflow-visible">
      {tabs.map(({ value, labelKey }) => (
        <button
          key={value}
          type="button"
          onClick={() => onSelect(value)}
          aria-pressed={feedMode === value}
          className={`inline-flex min-h-9 shrink-0 items-center justify-center whitespace-nowrap rounded-[12px] px-3 py-1.5 text-[12px] font-medium transition-all duration-150 press focus-ring sm:min-w-0 sm:px-2 sm:text-sm ${
            feedMode === value
              ? "bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)] shadow-sm"
              : "text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] hover:text-[var(--color-editorial-ink)]"
          }`}
        >
          {t(labelKey)}
        </button>
      ))}
    </div>
  );
}
