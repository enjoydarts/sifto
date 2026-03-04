import { type HTMLAttributes } from "react";

type SkeletonProps = HTMLAttributes<HTMLDivElement> & {
  className?: string;
};

export function Skeleton({ className = "", ...props }: SkeletonProps) {
  return (
    <div
      className={`animate-pulse rounded bg-zinc-200/70 ${className}`}
      {...props}
    />
  );
}

export function SkeletonLine({ width = "full" }: { width?: "full" | "3/4" | "1/2" | "2/3" }) {
  const widthClass =
    width === "full" ? "w-full" :
    width === "3/4"  ? "w-3/4"  :
    width === "2/3"  ? "w-2/3"  :
                       "w-1/2";
  return <Skeleton className={`h-4 ${widthClass}`} />;
}

export function SkeletonCard() {
  return (
    <div className="rounded-xl border border-zinc-200 bg-white p-4 space-y-3">
      <Skeleton className="h-3 w-16" />
      <div className="space-y-2">
        <SkeletonLine width="full" />
        <SkeletonLine width="3/4" />
        <SkeletonLine width="1/2" />
      </div>
      <div className="flex items-center justify-between">
        <Skeleton className="h-3 w-24" />
        <Skeleton className="h-3 w-16" />
      </div>
    </div>
  );
}

export function SkeletonItemRow() {
  return (
    <div className="flex items-stretch gap-3 rounded-xl border border-zinc-200 bg-white px-4 py-3.5">
      <Skeleton className="hidden h-[72px] w-[72px] shrink-0 rounded-lg sm:block" />
      <div className="flex min-w-0 flex-1 flex-col justify-between gap-1.5 py-0.5">
        <div className="space-y-1.5">
          <SkeletonLine width="full" />
          <SkeletonLine width="3/4" />
          <div className="flex gap-2 pt-0.5">
            <Skeleton className="h-4 w-12 rounded-full" />
            <Skeleton className="h-4 w-20" />
            <Skeleton className="h-4 w-10 rounded" />
          </div>
        </div>
        <Skeleton className="h-3 w-1/2" />
      </div>
      <div className="flex shrink-0 flex-col items-end gap-2">
        <Skeleton className="h-8 w-[108px] rounded" />
        <Skeleton className="h-8 w-[108px] rounded" />
      </div>
    </div>
  );
}

export function SkeletonKpi() {
  return (
    <div className="rounded-xl border border-zinc-200 bg-zinc-50/60 p-3 space-y-2">
      <div className="flex items-center gap-2">
        <Skeleton className="h-4 w-4 rounded" />
        <Skeleton className="h-3 w-24" />
      </div>
      <Skeleton className="h-8 w-16" />
    </div>
  );
}
