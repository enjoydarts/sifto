import { Skeleton, SkeletonCard } from "@/components/skeleton";
import { SectionCard } from "@/components/ui/section-card";

export default function MainLoading() {
  return (
    <div className="space-y-6" aria-hidden="true">
      <SectionCard className="p-5 sm:p-6">
        <Skeleton className="h-3 w-36" />
        <Skeleton className="mt-4 h-11 w-full max-w-xl" />
        <Skeleton className="mt-3 h-5 w-full max-w-2xl" />
        <div className="mt-5 grid gap-3 md:grid-cols-3">
          {Array.from({ length: 3 }).map((_, index) => (
            <Skeleton key={index} className="h-28 rounded-[18px]" />
          ))}
        </div>
      </SectionCard>

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1.2fr)_340px] xl:items-start">
        <div className="grid gap-6">
          <SectionCard className="min-h-[420px]">
            <SkeletonCard />
          </SectionCard>
          <SectionCard>
            <div className="grid gap-3">
              {Array.from({ length: 3 }).map((_, index) => (
                <SkeletonCard key={index} />
              ))}
            </div>
          </SectionCard>
        </div>

        <SectionCard>
          <div className="grid gap-3">
            {Array.from({ length: 2 }).map((_, index) => (
              <SkeletonCard key={index} />
            ))}
          </div>
        </SectionCard>
      </div>
    </div>
  );
}
