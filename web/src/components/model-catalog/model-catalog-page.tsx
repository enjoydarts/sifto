"use client";

import { RefreshCw, Search } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useI18n } from "@/components/i18n-provider";
import { PageTransition } from "@/components/page-transition";
import { PageHeader } from "@/components/ui/page-header";

type SectionConfig = {
  key: string;
  label: string;
  meta: string;
};

type ModelCatalogPageProps = {
  title: string;
  titleIcon: LucideIcon;
  description: string;
  syncing: boolean;
  onSync: () => void;
  syncLabel: string;
  syncingLabel: string;
  sections: SectionConfig[];
  activeSection: string;
  onSectionChange: (key: string) => void;
  statusContent: React.ReactNode;
  children: React.ReactNode;
  loading?: boolean;
  error?: string | null;
};

export function ModelCatalogPage({
  title,
  titleIcon: TitleIcon,
  description,
  syncing,
  onSync,
  syncLabel,
  syncingLabel,
  sections,
  activeSection,
  onSectionChange,
  statusContent,
  children,
  loading,
  error,
}: ModelCatalogPageProps) {
  const { t } = useI18n();
  return (
    <PageTransition>
      <div className="space-y-6 overflow-x-hidden">
        <PageHeader
          title={title}
          titleIcon={TitleIcon}
          description={description}
          actions={
            <button
              type="button"
              onClick={onSync}
              disabled={syncing}
              className="inline-flex min-h-11 items-center justify-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] disabled:opacity-60"
            >
              <RefreshCw className={`size-4 ${syncing ? "animate-spin" : ""}`} aria-hidden="true" />
              {syncing ? syncingLabel : syncLabel}
            </button>
          }
        />

        <div className="grid gap-5 xl:grid-cols-[260px_minmax(0,1fr)]">
          <aside className="surface-editorial rounded-[24px] p-4">
            <div className="space-y-1">
              {sections.map((section) => (
                <button
                  key={section.key}
                  type="button"
                  onClick={() => onSectionChange(section.key)}
                  className={`relative block w-full rounded-[16px] px-4 py-[13px] text-left ${
                    activeSection === section.key
                      ? "bg-[linear-gradient(90deg,rgba(243,236,227,0.92),rgba(243,236,227,0.28)_78%,transparent)]"
                      : "bg-transparent"
                  }`}
                >
                  {activeSection === section.key ? (
                    <span className="absolute bottom-3 left-0 top-3 w-[3px] rounded-full bg-[var(--color-editorial-ink)]" />
                  ) : null}
                  <div className="text-[15px] font-semibold text-[var(--color-editorial-ink)]">{section.label}</div>
                  <div className="mt-1 text-[12px] leading-6 text-[var(--color-editorial-ink-faint)]">{section.meta}</div>
                </button>
              ))}
            </div>

            <div className="mt-5 border-t border-[var(--color-editorial-line)] pt-5">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                {t("modelCatalog.status")}
              </div>
              <div className="mt-3 space-y-2 text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">
                {statusContent}
              </div>
            </div>
          </aside>

          <section className="min-w-0 space-y-4">
            {loading ? (
              <div className="surface-editorial rounded-[28px] p-5 text-sm text-[var(--color-editorial-ink-faint)]">
                <div className="flex items-center gap-2">
                  <RefreshCw className="size-4 animate-spin" aria-hidden="true" />
                  <span>{t("common.loading")}</span>
                </div>
              </div>
            ) : error ? (
              <div className="surface-editorial rounded-[28px] border border-red-200 bg-red-50 p-5 text-sm text-red-800">
                {error}
              </div>
            ) : (
              children
            )}
          </section>
        </div>
      </div>
    </PageTransition>
  );
}

type ModelCatalogFiltersProps = {
  query: string;
  onQueryChange: (q: string) => void;
  searchLabel: string;
  searchPlaceholder: string;
  clearLabel: string;
  providerFilter?: string;
  onProviderFilterChange?: (v: string) => void;
  providerFilterLabel?: string;
  providerAllLabel?: string;
  providerOptions?: string[];
};

export function ModelCatalogFilters({
  query,
  onQueryChange,
  searchLabel,
  searchPlaceholder,
  clearLabel,
  providerFilter,
  onProviderFilterChange,
  providerFilterLabel,
  providerAllLabel,
  providerOptions,
}: ModelCatalogFiltersProps) {
  const hasProviderFilter = providerFilter !== undefined && onProviderFilterChange;
  return (
    <section className="surface-editorial rounded-[24px] p-4">
      <div className="flex flex-col gap-3 md:flex-row">
        {hasProviderFilter ? (
          <label className="flex shrink-0 items-center gap-2 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm text-[var(--color-editorial-ink-soft)]">
            <span className="whitespace-nowrap text-[11px] font-semibold uppercase tracking-[0.12em] text-[var(--color-editorial-ink-faint)]">
              {providerFilterLabel}
            </span>
            <select
              value={providerFilter}
              onChange={(e) => onProviderFilterChange(e.target.value)}
              className="min-w-0 bg-transparent text-sm text-[var(--color-editorial-ink)] outline-none"
            >
              <option value="">{providerAllLabel}</option>
              {providerOptions?.map((provider) => (
                <option key={provider} value={provider}>
                  {provider}
                </option>
              ))}
            </select>
          </label>
        ) : null}
        <label className="flex min-w-0 flex-1 items-center gap-2 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2">
          <Search className="size-4 text-[var(--color-editorial-ink-faint)]" aria-hidden="true" />
          <input
            value={query}
            onChange={(e) => onQueryChange(e.target.value)}
            aria-label={searchLabel}
            placeholder={searchPlaceholder}
            className="w-full bg-transparent text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
          />
          {query ? (
            <button
              type="button"
              onClick={() => onQueryChange("")}
              className="shrink-0 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-[11px] font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
            >
              {clearLabel}
            </button>
          ) : null}
        </label>
      </div>
    </section>
  );
}

type SectionHeadingProps = {
  badge?: string;
  title: string;
  count?: number;
  countLabel?: string;
};

export function SectionHeading({ badge, title, count, countLabel }: SectionHeadingProps) {
  return (
    <div className="flex items-start justify-between gap-3">
      <div>
        {badge ? (
          <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
            {badge}
          </div>
        ) : null}
        <h2 className="mt-2 font-serif text-[2rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">
          {title}
        </h2>
      </div>
      {count !== undefined && countLabel ? (
        <div className="text-xs text-[var(--color-editorial-ink-faint)]">
          {count} {countLabel}
        </div>
      ) : null}
    </div>
  );
}

export function EmptyState({ children }: { children: React.ReactNode }) {
  return (
    <div className="mt-4 rounded-[22px] border border-dashed border-[var(--color-editorial-line-strong)] bg-[var(--color-editorial-panel)] px-4 py-6 text-sm text-[var(--color-editorial-ink-faint)]">
      {children}
    </div>
  );
}
