"use client";

import Link from "next/link";
import { Activity, Download, Lightbulb, Rss, Sparkles, Upload, X } from "lucide-react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Source, SourceHealth, SourceItemStats } from "@/lib/api";
import { AINavigatorAvatar } from "@/components/briefing/ai-navigator-avatar";
import Pagination from "@/components/pagination";
import { PageTransition } from "@/components/page-transition";
import { PageHeader } from "@/components/ui/page-header";
import { Tag } from "@/components/ui/tag";
import { useSourcesPageData } from "./use-sources-page-data";

type ChartTooltipValue = number | string | ReadonlyArray<number | string>;

export default function SourcesPage() {
  const {
    t, dateLocale,
    activeSection, setActiveSection,
    sources,
    sourceHealthByID,
    sourceItemStatsByID,
    sourceOptimization,
    sourcesDailyOverview,
    loadingDailyStats,
    dailyStatsError,
    page, setPage,
    loading, error,
    url, setUrl,
    title, setTitle,
    type, setType,
    adding,
    editingSource,
    editTitle, setEditTitle,
    savingEdit,
    recommendations,
    loadingSuggestions,
    suggestionsError,
    suggestionsLLM,
    suggestionLLMLabel,
    addingSuggestedURL,
    hasLoadedSuggestions,
    candidates,
    addError,
    exportingOPML,
    importingOPML,
    importingInoreader,
    sourceNavigator,
    sourceNavigatorLoading,
    sourceNavigatorError,
    sourceNavigatorOpen,
    setSourceNavigatorOpen,
    sourceNavigatorDisplayPersona,
    sourceNavigatorTheme,
    opmlInputRef,
    normalizeSuggestionReason,
    formatShortDate,
    overviewChartRows,
    healthSummary,
    sectionItems,
    pagedSources,
    loadDailyStats,
    loadSuggestions,
    registerSource,
    registerSuggestedSource,
    handleAdd,
    handleToggle,
    handleDelete,
    openEditDialog,
    closeEditDialog,
    handleSaveEdit,
    handleExportOPML,
    handleImportOPMLFile,
    handleImportInoreader,
    openSourceNavigator,
  } = useSourcesPageData();

  return (
    <PageTransition>
      <div className="space-y-5 overflow-x-hidden">
        <PageHeader
          eyebrow={t("sources.title")}
          title={t("nav.sources")}
          titleIcon={Rss}
          description={t("sources.controlRoomSubtitle")}
          meta={
            <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1 text-xs">
              {sources.length.toLocaleString()} {t("common.rows")}
            </span>
          }
        />

        <section className="grid gap-4 xl:grid-cols-[280px_minmax(0,1fr)]">
          <aside className="surface-editorial rounded-[26px] px-4 py-4 xl:sticky xl:top-[6.25rem] xl:self-start">
            <div className="mt-1">
              {sectionItems.map((section, index) => (
                <button
                  key={section.key}
                  type="button"
                  onClick={() => {
                    if (section.key === "add") {
                      loadSuggestions();
                    }
                    setActiveSection(section.key);
                  }}
                  className={`relative block w-full border-t border-[var(--color-editorial-line)] px-3 py-3 text-left transition-colors first:border-t-0 ${
                    activeSection === section.key
                      ? "bg-[linear-gradient(90deg,rgba(243,236,227,0.92),rgba(243,236,227,0.28)_78%,transparent)]"
                      : "hover:bg-[var(--color-editorial-panel-strong)]"
                  }`}
                >
                  {activeSection === section.key ? (
                    <span
                      aria-hidden="true"
                      className={`absolute left-0 w-[3px] rounded-full bg-[var(--color-editorial-ink)] ${
                        index === 0 ? "top-0 bottom-3" : "top-3 bottom-3"
                      }`}
                    />
                  ) : null}
                  <div className="text-[13px] font-semibold text-[var(--color-editorial-ink)]">{section.title}</div>
                  <div className="mt-1 text-xs leading-6 text-[var(--color-editorial-ink-soft)]">{section.meta}</div>
                </button>
              ))}
            </div>

            <div className="mt-4 rounded-[20px] border border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,rgba(255,255,255,0.92),rgba(255,253,249,0.98))] px-4 py-4">
              <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                {t("sources.currentState")}
              </div>
              <div className="mt-2 font-serif text-[30px] leading-none text-[var(--color-editorial-ink)]">{sources.length}</div>
              <div className="mt-2 text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">
                {`${t("sources.currentStateMeta")} ${healthSummary.ok} / stale ${healthSummary.stale} / error ${healthSummary.error}`}
              </div>
            </div>
          </aside>

          <div className="min-w-0 space-y-4">
            {activeSection === "overview" && (
              <>
                <section className="surface-editorial rounded-[28px] px-5 py-5">
                  <h2 className="font-serif text-[30px] leading-[1.16] tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                    {t("sources.section.overviewTitle")}
                  </h2>
                  <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                    {t("sources.section.overviewDescription")}
                  </p>

                  <div className="mt-4 grid gap-3 md:grid-cols-3">
                    <MetricCard label={t("sources.activeSources")} value={String(sources.filter((source) => source.enabled).length)} />
                    <MetricCard
                      label={t("sources.unreadAcrossSources")}
                      value={String(
                        Object.values(sourceItemStatsByID).reduce((sum, stat) => sum + (stat.unread_items ?? 0), 0)
                      )}
                    />
                    <MetricCard label={t("sources.ingestion30d")} value={String(sourcesDailyOverview?.last_30d_total ?? 0)} />
                  </div>

                  <div className="mt-4 rounded-[22px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.66)] px-3 py-3">
                    <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
                      <div>
                        <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("sources.activity.overviewTitle")}</div>
                        <div className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{t("sources.activity.note")}</div>
                      </div>
                      <button
                        type="button"
                        onClick={() => void loadDailyStats()}
                        disabled={loadingDailyStats}
                        className="inline-flex min-h-[40px] items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 text-sm text-[var(--color-editorial-ink-soft)] disabled:opacity-50"
                      >
                        <Activity className="size-4" aria-hidden="true" />
                        {loadingDailyStats ? t("common.loading") : t("sources.activity.refresh")}
                      </button>
                    </div>
                    {dailyStatsError ? (
                      <div className="mb-3 rounded-[18px] border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
                        <div className="font-medium">{t("sources.activity.loadFailed")}</div>
                        <div className="mt-1">{dailyStatsError}</div>
                      </div>
                    ) : null}
                    <div className="h-56 overflow-hidden rounded-[20px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2 py-3">
                      <ResponsiveContainer width="100%" height="100%">
                        <AreaChart data={overviewChartRows} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
                          <defs>
                            <linearGradient id="sourcesOverviewFill" x1="0" y1="0" x2="0" y2="1">
                              <stop offset="5%" stopColor="#5a4735" stopOpacity={0.24} />
                              <stop offset="95%" stopColor="#5a4735" stopOpacity={0.03} />
                            </linearGradient>
                          </defs>
                          <CartesianGrid stroke="#d9d1c4" strokeDasharray="3 3" vertical={false} />
                          <XAxis dataKey="label" tick={{ fill: "#8f877f", fontSize: 11 }} tickLine={false} axisLine={false} minTickGap={24} />
                          <YAxis allowDecimals={false} tick={{ fill: "#8f877f", fontSize: 11 }} tickLine={false} axisLine={false} width={28} />
                          <Tooltip
                            cursor={{ stroke: "#beb3a0", strokeDasharray: "3 3" }}
                            contentStyle={{ borderRadius: 16, borderColor: "#d9d1c4", boxShadow: "0 8px 24px rgba(24,24,27,0.08)" }}
                            formatter={(value: ChartTooltipValue | undefined) => [
                              Array.isArray(value) ? value.map(String).join(", ") : typeof value === "number" ? value.toLocaleString() : String(value ?? 0),
                              t("common.rows"),
                            ]}
                            labelFormatter={(_, payload) => {
                              const row = payload?.[0]?.payload as { day?: string } | undefined;
                              return row?.day ? formatShortDate(row.day) : "";
                            }}
                          />
                          <Area type="monotone" dataKey="count" stroke="#5a4735" strokeWidth={2} fill="url(#sourcesOverviewFill)" dot={{ r: 2, strokeWidth: 0, fill: "#5a4735" }} activeDot={{ r: 4, strokeWidth: 0, fill: "#5a4735" }} />
                        </AreaChart>
                      </ResponsiveContainer>
                    </div>
                  </div>
                </section>

                <section className="surface-editorial rounded-[28px] px-5 py-5">
                  <h2 className="font-serif text-[24px] leading-[1.2] text-[var(--color-editorial-ink)]">{t("sources.section.sourcesTitle")}</h2>
                  <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{t("sources.section.sourcesDescription")}</p>
                  <div className="mt-4 space-y-3">
                    {pagedSources.slice(0, 3).map((src) => (
                      <SourceCard
                        key={src.id}
                        src={src}
                        health={sourceHealthByID[src.id]}
                        stats={sourceItemStatsByID[src.id]}
                        dateLocale={dateLocale}
                        t={t}
                        onToggle={handleToggle}
                        onEdit={openEditDialog}
                        onDelete={handleDelete}
                      />
                    ))}
                  </div>
                </section>
              </>
            )}

            {activeSection === "sources" && (
              <section className="surface-editorial rounded-[28px] px-5 py-5">
                <h2 className="font-serif text-[30px] leading-[1.16] tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                  {t("sources.section.sourcesTitle")}
                </h2>
                <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{t("sources.section.sourcesDescription")}</p>
                {loading && <p className="mt-4 text-sm text-[var(--color-editorial-ink-soft)]">{t("common.loading")}</p>}
                {error && <p className="mt-4 text-sm text-red-600">{error}</p>}
                {!loading && !error && sources.length === 0 && <p className="mt-4 text-sm text-[var(--color-editorial-ink-faint)]">{t("sources.empty")}</p>}
                <div className="mt-4 space-y-3">
                  {pagedSources.map((src) => (
                    <SourceCard
                      key={src.id}
                      src={src}
                      health={sourceHealthByID[src.id]}
                      stats={sourceItemStatsByID[src.id]}
                      dateLocale={dateLocale}
                      t={t}
                      onToggle={handleToggle}
                      onEdit={openEditDialog}
                      onDelete={handleDelete}
                    />
                  ))}
                </div>
                <div className="mt-4">
                  <Pagination total={sources.length} page={page} pageSize={10} onPageChange={setPage} />
                </div>
              </section>
            )}

            {activeSection === "optimization" && (
              <section className="surface-editorial rounded-[28px] px-5 py-5">
                <h2 className="font-serif text-[30px] leading-[1.16] tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                  {t("sources.optimization.title")}
                </h2>
                <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{t("sources.optimization.subtitle")}</p>
                <div className="mt-4 grid gap-3">
                  {sourceOptimization.length === 0 ? (
                    <p className="text-sm text-[var(--color-editorial-ink-faint)]">{t("sources.optimization.empty")}</p>
                  ) : (
                    sourceOptimization.map((item) => {
                      const source = sources.find((candidate) => candidate.id === item.source_id);
                      return (
                        <article key={item.source_id} className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.64)] p-4">
                          <div className="flex flex-wrap items-start justify-between gap-3">
                            <div className="min-w-0">
                              <div className="text-sm font-semibold leading-7 text-[var(--color-editorial-ink)]">
                                {source?.title || source?.url || item.source_id}
                              </div>
                              <p className="mt-1 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{item.reason}</p>
                            </div>
                            <Tag tone="subtle">{item.recommendation}</Tag>
                          </div>
                          <div className="mt-3 grid gap-2 text-xs text-[var(--color-editorial-ink-soft)] md:grid-cols-4">
                            <div>{t("sources.optimization.backlog")}: {item.metrics.unread_backlog}</div>
                            <div>{t("sources.optimization.readRate")}: {Math.round(item.metrics.read_rate * 100)}%</div>
                            <div>{t("sources.optimization.favoriteRate")}: {Math.round(item.metrics.favorite_rate * 100)}%</div>
                            <div>{t("sources.optimization.avgScore")}: {item.metrics.average_summary_score.toFixed(2)}</div>
                          </div>
                        </article>
                      );
                    })
                  )}
                </div>
              </section>
            )}

            {activeSection === "add" && (
              <>
                <section className="rounded-[24px] border border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,rgba(255,255,255,0.92),rgba(255,253,249,0.98))] px-5 py-5 shadow-[var(--shadow-card)]">
                  <h2 className="font-serif text-[24px] leading-[1.2] text-[var(--color-editorial-ink)]">{t("sources.addSource")}</h2>
                  <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{t("sources.section.addDescription")}</p>
                  <form onSubmit={handleAdd} className="mt-4 space-y-3">
                    <div className="flex gap-3 text-sm">
                      {(["rss", "manual"] as const).map((kind) => (
                        <label key={kind} className="flex cursor-pointer items-center gap-1.5">
                          <input type="radio" name="type" value={kind} checked={type === kind} onChange={() => setType(kind)} className="accent-zinc-900" />
                          {kind === "rss" ? t("sources.rss") : t("sources.manual")}
                        </label>
                      ))}
                    </div>
                    <div className="grid gap-2 lg:grid-cols-[minmax(0,1fr)_180px_140px]">
                      <input
                        type="url"
                        placeholder={type === "rss" ? t("sources.placeholder.rss") : t("sources.placeholder.manual")}
                        value={url}
                        onChange={(e) => setUrl(e.target.value)}
                        required
                        className="rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-3 text-sm outline-none"
                      />
                      <input
                        type="text"
                        placeholder={t("sources.placeholder.nameOptional")}
                        value={title}
                        onChange={(e) => setTitle(e.target.value)}
                        className="rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-3 text-sm outline-none"
                      />
                      <button
                        type="submit"
                        disabled={adding}
                        className="inline-flex min-h-[46px] items-center justify-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 text-sm font-semibold text-[var(--color-editorial-panel-strong)] disabled:opacity-50"
                      >
                        {adding ? t("sources.adding") : t("sources.add")}
                      </button>
                    </div>
                    {addError ? <p className="text-sm text-red-600">{addError}</p> : null}
                    {candidates.length > 1 ? (
                      <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.64)] p-4">
                        <p className="mb-2 text-xs text-[var(--color-editorial-ink-soft)]">{t("sources.discover.multiple")}</p>
                        <ul className="space-y-2">
                          {candidates.map((candidate) => (
                            <li key={candidate.url} className="flex items-center justify-between gap-3 rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2">
                              <div className="min-w-0">
                                {candidate.title ? <div className="truncate text-sm font-medium text-[var(--color-editorial-ink)]">{candidate.title}</div> : null}
                                <div className="truncate text-xs text-[var(--color-editorial-ink-soft)]">{candidate.url}</div>
                              </div>
                              <button
                                type="button"
                                onClick={async () => {
                                   try {
                                     await registerSource(candidate.url);
                                   } catch {
                                     // Error handled by parent form state
                                   }
                                }}
                                disabled={adding}
                                className="shrink-0 rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-3 py-2 text-xs font-semibold text-[var(--color-editorial-panel-strong)] disabled:opacity-50"
                              >
                                {t("sources.discover.register")}
                              </button>
                            </li>
                          ))}
                        </ul>
                      </div>
                    ) : null}
                  </form>
                </section>

                <section className="rounded-[24px] border border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,rgba(255,255,255,0.92),rgba(255,253,249,0.98))] px-5 py-5 shadow-[var(--shadow-card)]">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <div>
                      <h2 className="font-serif text-[24px] leading-[1.2] text-[var(--color-editorial-ink)]">{t("sources.opml.title")}</h2>
                      <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{t("sources.opml.desc")}</p>
                    </div>
                    <div className="flex flex-wrap gap-2">
                      <input
                        ref={opmlInputRef}
                        type="file"
                        accept=".opml,.xml,text/xml,application/xml"
                        className="hidden"
                        onChange={(e) => {
                          const f = e.target.files?.[0];
                          if (f) void handleImportOPMLFile(f);
                        }}
                      />
                      <button type="button" onClick={() => opmlInputRef.current?.click()} disabled={importingOPML} className="inline-flex min-h-[42px] items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 text-sm text-[var(--color-editorial-ink-soft)] disabled:opacity-50">
                        <Upload className="size-4" aria-hidden="true" />
                        {importingOPML ? t("sources.opml.importing") : t("sources.opml.import")}
                      </button>
                      <button type="button" onClick={() => void handleExportOPML()} disabled={exportingOPML} className="inline-flex min-h-[42px] items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 text-sm text-[var(--color-editorial-ink-soft)] disabled:opacity-50">
                        <Download className="size-4" aria-hidden="true" />
                        {exportingOPML ? t("sources.opml.exporting") : t("sources.opml.export")}
                      </button>
                    </div>
                  </div>
                </section>

                <section className="rounded-[24px] border border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,rgba(255,255,255,0.92),rgba(255,253,249,0.98))] px-5 py-5 shadow-[var(--shadow-card)]">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <div>
                      <h2 className="font-serif text-[24px] leading-[1.2] text-[var(--color-editorial-ink)]">{t("sources.inoreader.title")}</h2>
                      <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{t("sources.inoreader.desc")}</p>
                    </div>
                    <button type="button" onClick={() => void handleImportInoreader()} disabled={importingInoreader} className="inline-flex min-h-[42px] items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 text-sm font-semibold text-[var(--color-editorial-panel-strong)] disabled:opacity-50">
                      {importingInoreader ? t("sources.inoreader.importing") : t("sources.inoreader.import")}
                    </button>
                  </div>
                </section>

                <section className="surface-editorial rounded-[28px] px-5 py-5">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <div>
                      <h2 className="inline-flex items-center gap-2 font-serif text-[24px] leading-[1.2] text-[var(--color-editorial-ink)]">
                        <Sparkles className="size-5 text-[var(--color-editorial-ink-soft)]" aria-hidden="true" />
                        {t("sources.suggest.title")}
                      </h2>
                      <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{t("sources.suggest.desc")}</p>
                    </div>
                    <button type="button" onClick={() => void loadSuggestions()} disabled={loadingSuggestions} className="inline-flex min-h-[42px] items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 text-sm text-[var(--color-editorial-ink-soft)] disabled:opacity-50">
                      <Lightbulb className="size-4" aria-hidden="true" />
                      {loadingSuggestions ? t("sources.suggest.finding") : t("sources.suggest.button")}
                    </button>
                  </div>
                  {suggestionsLLM ? (
                    <div className="mt-3 space-y-1 text-xs text-[var(--color-editorial-ink-soft)]">
                      <p>
                        {suggestionLLMLabel}: {suggestionsLLM.provider ?? t("common.unknown")} / {suggestionsLLM.model ?? t("common.unknown")}
                      </p>
                      {suggestionsLLM.warning ? <p>{t("sources.suggest.warningPrefix")}: {suggestionsLLM.warning}</p> : null}
                      {suggestionsLLM.error ? <p className="text-red-600">{t("sources.suggest.errorPrefix")}: {suggestionsLLM.error}</p> : null}
                    </div>
                  ) : null}
                  {suggestionsError ? <p className="mt-3 text-sm text-red-600">{suggestionsError}</p> : null}
                  {!suggestionsError && !loadingSuggestions && hasLoadedSuggestions && recommendations.length === 0 ? (
                    <p className="mt-3 text-sm text-[var(--color-editorial-ink-faint)]">{t("sources.suggest.empty")}</p>
                  ) : null}
                  <div className="mt-4 grid gap-3">
                    {recommendations.map((suggestion) => {
                      const normalizedAIReason = normalizeSuggestionReason(suggestion.ai_reason);
                      const hasDistinctAIReason =
                        normalizedAIReason !== "" &&
                        !suggestion.reasons.some((reason) => normalizeSuggestionReason(reason) === normalizedAIReason);
                      return (
                        <article key={suggestion.url} className="rounded-[20px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.64)] p-4">
                          <div className="flex flex-wrap items-start justify-between gap-3">
                            <div className="min-w-0">
                              <div className="text-sm font-semibold leading-7 text-[var(--color-editorial-ink)]">{suggestion.title ?? suggestion.url}</div>
                              {suggestion.title ? <div className="truncate text-xs text-[var(--color-editorial-ink-soft)]">{suggestion.url}</div> : null}
                              {suggestion.reasons.length > 0 ? (
                                <div className="mt-3 space-y-2">
                                  {suggestion.reasons.slice(0, 2).map((reason) => (
                                    <p
                                      key={`${suggestion.url}-${reason}`}
                                      className="rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2 text-sm leading-6 text-[var(--color-editorial-ink-soft)]"
                                    >
                                      {reason}
                                    </p>
                                  ))}
                                </div>
                              ) : null}
                              {suggestion.matched_topics?.length ? (
                                <div className="mt-2 flex flex-wrap gap-2">
                                  {suggestion.matched_topics.slice(0, 3).map((topic) => (
                                    <Tag key={`${suggestion.url}-topic-${topic}`} tone="info">{`${t("sources.suggest.topicPrefix")} ${topic}`}</Tag>
                                  ))}
                                </div>
                              ) : null}
                              {hasDistinctAIReason && suggestion.ai_reason ? (
                                <p className="mt-3 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                                  <span className="font-medium text-[var(--color-editorial-ink)]">{t("sources.suggest.aiReason")}:</span> {suggestion.ai_reason}
                                </p>
                              ) : null}
                            </div>
                            <button type="button" onClick={() => void registerSuggestedSource(suggestion)} disabled={addingSuggestedURL === suggestion.url} className="shrink-0 rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-xs font-semibold text-[var(--color-editorial-panel-strong)] disabled:opacity-50">
                              {addingSuggestedURL === suggestion.url ? t("sources.adding") : t("sources.add")}
                            </button>
                          </div>
                        </article>
                      );
                    })}
                  </div>
                </section>
              </>
            )}
          </div>
        </section>

        <div className="fixed right-4 z-40 bottom-[calc(5rem+env(safe-area-inset-bottom))] md:bottom-6 md:right-6">
          {sourceNavigatorOpen && sourceNavigator ? (
            <aside className="absolute bottom-0 right-0 w-[min(calc(100vw-1.5rem),38rem)]">
              <div className={`flex max-h-[min(78vh,44rem)] flex-col overflow-hidden rounded-[26px] border shadow-[0_24px_80px_rgba(58,42,27,0.18)] ${sourceNavigatorTheme.shell}`}>
                <div className={`flex items-start gap-3 border-b px-4 py-4 ${sourceNavigatorTheme.header} border-[var(--color-editorial-line)]`}>
                  <div className={`shrink-0 rounded-full border border-[var(--color-editorial-line)] p-1.5 shadow-sm ${sourceNavigatorTheme.avatar}`}>
                    <AINavigatorAvatar persona={sourceNavigatorDisplayPersona} className="size-[42px]" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                      {t("briefing.navigator.label")}
                    </div>
                    <div className="mt-1 text-sm font-semibold text-[var(--color-editorial-ink)]">
                      {sourceNavigator.character_name}
                      <span className="ml-2 text-xs font-medium text-[var(--color-editorial-ink-faint)]">{sourceNavigator.character_title}</span>
                    </div>
                    <p className="mt-2 text-sm font-medium leading-6 text-[var(--color-editorial-ink-soft)]">
                      {t("sources.navigator.subtitle")}
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() => setSourceNavigatorOpen(false)}
                    className="inline-flex size-9 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-white/70 text-[var(--color-editorial-ink-soft)] hover:bg-white"
                    aria-label={t("briefing.navigator.close")}
                  >
                    <X className="size-4" aria-hidden="true" />
                  </button>
                </div>
                <div className="space-y-4 overflow-y-auto px-4 py-4">
                  <div className={`rounded-[20px] border px-4 py-4 ${sourceNavigatorTheme.bubble}`}>
                    <div className="space-y-2 whitespace-pre-line text-[15px] leading-7 text-[var(--color-editorial-ink-soft)]">
                      {sourceNavigator.overview}
                    </div>
                  </div>
                  <SourceNavigatorSection
                    title={t("sources.navigator.keep")}
                    items={sourceNavigator.keep}
                    badgeClassName={sourceNavigatorTheme.badge}
                  />
                  <SourceNavigatorSection
                    title={t("sources.navigator.watch")}
                    items={sourceNavigator.watch}
                    badgeClassName={sourceNavigatorTheme.badge}
                  />
                  <SourceNavigatorSection
                    title={t("sources.navigator.standout")}
                    items={sourceNavigator.standout}
                    badgeClassName={sourceNavigatorTheme.badge}
                  />
                </div>
              </div>
            </aside>
          ) : null}

          {!sourceNavigatorOpen && !sourceNavigatorLoading ? (
            <button
              type="button"
              onClick={() => {
                void openSourceNavigator();
              }}
              className={`rounded-full border p-2 shadow-[0_18px_40px_rgba(58,42,27,0.16)] transition hover:-translate-y-0.5 ${sourceNavigatorTheme.shell}`}
              aria-label={t("sources.navigator.open")}
            >
              <AINavigatorAvatar persona={sourceNavigatorDisplayPersona} className="size-11" />
            </button>
          ) : null}

          {sourceNavigatorLoading && !sourceNavigatorOpen ? (
            <div className="flex items-center gap-3 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2 py-2 shadow-[0_18px_40px_rgba(58,42,27,0.16)]">
              <div className={`rounded-full border border-[var(--color-editorial-line)] p-1.5 ${sourceNavigatorTheme.shell}`}>
                <AINavigatorAvatar persona={sourceNavigatorDisplayPersona} className="size-10" />
              </div>
              <div className="pr-2">
                <div className="text-[10px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                  {t("briefing.navigator.label")}
                </div>
                <div className="mt-0.5 text-sm font-medium text-[var(--color-editorial-ink-soft)]">
                  {t("sources.navigator.loading")}
                </div>
              </div>
            </div>
          ) : null}

          {sourceNavigatorError && !sourceNavigatorOpen ? (
            <div className="mt-3 max-w-[min(calc(100vw-2rem),24rem)] rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2 text-xs leading-5 text-[var(--color-editorial-ink-soft)] shadow-[0_12px_32px_rgba(58,42,27,0.12)]">
              {sourceNavigatorError}
            </div>
          ) : null}
        </div>

      {editingSource && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-900/40 px-4">
          <div className="w-full max-w-lg rounded-xl border border-zinc-200 bg-white p-5 shadow-xl">
            <div className="mb-4">
              <h2 className="text-base font-semibold text-zinc-900">
                {t("sources.editModal.title")}
              </h2>
              <p className="mt-1 break-all text-xs text-zinc-500">{editingSource.url}</p>
            </div>

            <form onSubmit={handleSaveEdit} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-zinc-700">
                  {t("sources.editModal.displayName")}
                </label>
                <input
                  type="text"
                  value={editTitle}
                  onChange={(e) => setEditTitle(e.target.value)}
                  placeholder={t("sources.editModal.placeholder")}
                  className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 outline-none placeholder:text-zinc-400 focus:border-zinc-400"
                  autoFocus
                />
              </div>

              <div className="flex items-center justify-end gap-2">
                <button
                  type="button"
                  onClick={closeEditDialog}
                  disabled={savingEdit}
                  className="rounded-lg border border-zinc-300 bg-white px-4 py-2 text-sm font-medium text-zinc-700 disabled:opacity-50"
                >
                  {t("common.cancel")}
                </button>
                <button
                  type="submit"
                  disabled={savingEdit}
                  className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
                >
                  {savingEdit ? t("common.saving") : t("common.save")}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
      </div>
    </PageTransition>
  );
}

function MetricCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.72)] px-4 py-4 shadow-[var(--shadow-card)]">
      <div className="text-[11px] uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{label}</div>
      <div className="mt-2 font-serif text-[28px] text-[var(--color-editorial-ink)]">{value}</div>
    </div>
  );
}

function SourceCard({
  src,
  health,
  stats,
  dateLocale,
  t,
  onToggle,
  onEdit,
  onDelete,
}: {
  src: Source;
  health?: SourceHealth;
  stats?: SourceItemStats;
  dateLocale: string;
  t: (key: string) => string;
  onToggle: (id: string, enabled: boolean) => void;
  onEdit: (src: Source) => void;
  onDelete: (id: string) => void;
}) {
  return (
    <article className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.72)] p-4 shadow-[var(--shadow-card)]">
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_240px]">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <button
              type="button"
              onClick={() => onToggle(src.id, src.enabled)}
              aria-label={src.enabled ? t("sources.toggle.disable") : t("sources.toggle.enable")}
              className={`relative inline-flex h-5 w-9 shrink-0 rounded-full border-2 border-transparent transition-colors ${
                src.enabled ? "bg-[#5a4735]" : "bg-zinc-300"
              }`}
            >
              <span className={`inline-block h-4 w-4 rounded-full bg-white shadow transition-transform ${src.enabled ? "translate-x-4" : "translate-x-0"}`} />
            </button>
            {health ? <Tag tone="subtle">{health.status}</Tag> : null}
            {src.last_fetched_at ? (
              <Tag tone="subtle">
                {t("sources.lastFetched")}: {new Date(src.last_fetched_at).toLocaleString(dateLocale)}
              </Tag>
            ) : null}
          </div>
          <Link
            href={`/items?feed=unread&sort=personal_score&source_id=${src.id}`}
            className="mt-3 block text-[17px] font-semibold leading-7 text-[var(--color-editorial-ink)] hover:underline"
          >
            {src.title ?? src.url}
          </Link>
          {src.title ? <div className="mt-1 truncate text-xs text-[var(--color-editorial-ink-soft)]">{src.url}</div> : null}
          {health ? (
            <div className="mt-2 text-xs text-[var(--color-editorial-ink-soft)]">
              {health.failed_items}/{health.total_items} {t("sources.health.failed")}
            </div>
          ) : null}
          <div className="mt-4 flex flex-wrap gap-2">
            <Link
              href={`/items?feed=unread&sort=personal_score&source_id=${src.id}`}
              className="inline-flex min-h-[40px] items-center justify-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 text-sm font-semibold text-[var(--color-editorial-panel-strong)]"
            >
              {t("sources.openItems")}
            </Link>
            <button type="button" onClick={() => onEdit(src)} className="inline-flex min-h-[40px] items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 text-sm text-[var(--color-editorial-ink-soft)]">
              {t("sources.edit")}
            </button>
            <button type="button" onClick={() => void onDelete(src.id)} className="inline-flex min-h-[40px] items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 text-sm text-[var(--color-editorial-ink-soft)]">
              {t("sources.delete")}
            </button>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
          <SourceStat label={t("sources.stats.unread")} value={(stats?.unread_items ?? 0).toLocaleString()} />
          <SourceStat label={t("sources.stats.read")} value={(stats?.read_items ?? 0).toLocaleString()} />
          <SourceStat label={t("sources.stats.total")} value={(stats?.total_items ?? 0).toLocaleString()} />
          <SourceStat label={t("sources.stats.avgPerDay30d")} value={(stats?.avg_items_per_day_30d ?? 0).toFixed(1)} />
        </div>
      </div>
    </article>
  );
}

function SourceStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="border-l border-[var(--color-editorial-line)] pl-3 first:border-l-0 first:pl-0 xl:first:border-l xl:first:pl-3 xl:[&:first-child]:border-l-0 xl:[&:first-child]:pl-0">
      <div className="text-[10px] uppercase tracking-[0.12em] text-[var(--color-editorial-ink-faint)]">{label}</div>
      <div className="mt-2 text-[17px] font-semibold text-[var(--color-editorial-ink)]">{value}</div>
    </div>
  );
}

function SourceNavigatorSection({
  title,
  items,
  badgeClassName,
}: {
  title: string;
  items: Array<{ source_id: string; title: string; comment: string }>;
  badgeClassName: string;
}) {
  if (items.length === 0) return null;
  return (
    <section className="rounded-[20px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-4">
      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
        {title}
      </div>
      <div className="mt-3 space-y-3">
        {items.map((item, index) => (
          <div key={`${title}-${item.source_id}`} className="flex items-start gap-3 rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-3">
            <div className={`mt-0.5 inline-flex size-7 shrink-0 items-center justify-center rounded-full text-xs font-semibold ${badgeClassName}`}>
              {index + 1}
            </div>
            <div className="min-w-0 flex-1">
              <div className="font-serif text-[1rem] font-semibold leading-[1.35] text-[var(--color-editorial-ink)]">
                {item.title}
              </div>
              <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{item.comment}</p>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
