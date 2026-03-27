"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { Brain } from "lucide-react";
import { useI18n } from "@/components/i18n-provider";
import { PageTransition } from "@/components/page-transition";
import { PageHeader } from "@/components/ui/page-header";
import { SectionCard } from "@/components/ui/section-card";
import { Tag } from "@/components/ui/tag";
import { api } from "@/lib/api";

function formatDateTime(value: string | null | undefined, locale: string) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(locale === "ja" ? "ja-JP" : "en-US", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export default function AINavigatorBriefsPage() {
  const { t, locale } = useI18n();
  const briefsQuery = useQuery({
    queryKey: ["ai-navigator-briefs"],
    queryFn: () => api.getAINavigatorBriefs({ limit: 30 }),
  });

  return (
    <PageTransition>
      <div className="space-y-4">
        <PageHeader
          eyebrow={t("aiNavigatorBriefs.eyebrow")}
          title={t("aiNavigatorBriefs.title")}
          titleIcon={Brain}
          description={t("aiNavigatorBriefs.description")}
        />

        {briefsQuery.isLoading ? (
          <SectionCard>
            <p className="text-sm text-editorial-muted">{t("common.loading")}</p>
          </SectionCard>
        ) : briefsQuery.data?.items.length ? (
          <div className="space-y-3">
            {briefsQuery.data.items.map((brief) => (
              <SectionCard key={brief.id}>
                <div className="space-y-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <Tag tone="default">{t(`aiNavigatorBriefs.slot.${brief.slot}`)}</Tag>
                    <Tag tone="default">{t(`aiNavigatorBriefs.status.${brief.status}`)}</Tag>
                    <Tag tone="default">{formatDateTime(brief.generated_at ?? brief.created_at, locale)}</Tag>
                  </div>
                  <div className="space-y-1">
                    <h2 className="font-serif text-xl text-editorial-strong">{brief.title || "—"}</h2>
                    <p className="text-sm leading-6 text-editorial-muted">{brief.summary || brief.intro || "—"}</p>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <Link
                      href={`/ai-navigator-briefs/${brief.id}`}
                      className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90"
                    >
                      {t("aiNavigatorBriefs.openDetail")}
                    </Link>
                  </div>
                </div>
              </SectionCard>
            ))}
          </div>
        ) : (
          <SectionCard>
            <p className="text-sm text-editorial-muted">{t("aiNavigatorBriefs.empty")}</p>
          </SectionCard>
        )}
      </div>
    </PageTransition>
  );
}
