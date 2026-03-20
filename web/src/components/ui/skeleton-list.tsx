import type { ReactNode } from "react";

type SkeletonListProps = {
  rows?: number;
  variant?: "article" | "card" | "compact";
  className?: string;
  label?: ReactNode;
};

function joinClassNames(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

function SkeletonBlock({ className = "" }: { className?: string }) {
  return <div className={joinClassNames("animate-pulse rounded bg-[var(--border-subtle)]/70", className)} />;
}

function ArticleRowSkeleton() {
  return (
    <div className="rounded-[18px] border border-editorial bg-[var(--panel-strong)] px-4 py-4">
      <div className="flex flex-col gap-4 sm:flex-row">
        <SkeletonBlock className="h-[96px] w-full rounded-[14px] sm:w-[104px]" />
        <div className="min-w-0 flex-1 space-y-3">
          <div className="flex flex-wrap gap-2">
            <SkeletonBlock className="h-5 w-16 rounded-full" />
            <SkeletonBlock className="h-5 w-14 rounded-full" />
            <SkeletonBlock className="h-5 w-20 rounded-full" />
          </div>
          <SkeletonBlock className="h-6 w-11/12" />
          <SkeletonBlock className="h-6 w-5/6" />
          <SkeletonBlock className="h-4 w-3/4" />
          <div className="flex flex-wrap gap-2 pt-1">
            <SkeletonBlock className="h-4 w-20 rounded-full" />
            <SkeletonBlock className="h-4 w-24 rounded-full" />
            <SkeletonBlock className="h-4 w-28 rounded-full" />
          </div>
        </div>
        <div className="flex shrink-0 flex-col gap-3 sm:items-end">
          <SkeletonBlock className="h-[84px] w-[140px] rounded-[16px]" />
          <div className="flex gap-2">
            <SkeletonBlock className="h-9 w-20 rounded-full" />
            <SkeletonBlock className="h-9 w-20 rounded-full" />
          </div>
        </div>
      </div>
    </div>
  );
}

function CardSkeleton() {
  return (
    <div className="rounded-[18px] border border-editorial bg-[var(--panel-strong)] p-4">
      <SkeletonBlock className="h-3 w-20 rounded-full" />
      <SkeletonBlock className="mt-4 h-9 w-28" />
      <SkeletonBlock className="mt-3 h-4 w-3/4" />
      <SkeletonBlock className="mt-2 h-4 w-1/2" />
    </div>
  );
}

function CompactSkeleton() {
  return (
    <div className="rounded-[18px] border border-editorial bg-[var(--panel-strong)] px-4 py-3">
      <SkeletonBlock className="h-3 w-16 rounded-full" />
      <SkeletonBlock className="mt-3 h-7 w-24" />
      <SkeletonBlock className="mt-2 h-3 w-2/3" />
    </div>
  );
}

export function SkeletonList({
  rows = 4,
  variant = "article",
  className = "",
  label,
}: SkeletonListProps) {
  const blocks = Array.from({ length: rows });

  return (
    <div className={joinClassNames("space-y-3", className)} aria-busy="true" aria-live="polite">
      {label && <div className="px-1 text-[11px] font-semibold uppercase tracking-[0.18em] text-editorial-muted">{label}</div>}
      {blocks.map((_, index) => (
        <div key={index}>
          {variant === "card" ? <CardSkeleton /> : variant === "compact" ? <CompactSkeleton /> : <ArticleRowSkeleton />}
        </div>
      ))}
    </div>
  );
}
