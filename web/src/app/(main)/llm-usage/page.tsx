"use client";

import { Brain, CalendarDays } from "lucide-react";
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  ReferenceLine,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { providerLabel } from "@/lib/model-display";
import {
  CurrentMonthByProviderTable,
  CurrentMonthByPurposeTable,
  DailySummaryGroups,
  MetricCard,
  RecentLogsTable,
  ReliabilityTable,
} from "@/components/llm-usage/tables";
import { PageHeader } from "@/components/ui/page-header";
import { SectionCard } from "@/components/ui/section-card";
import {
  useLLMUsageData,
  fmtUSD,
  fmtNum,
  fmtUSDShort,
} from "./use-llm-usage-data";

type ChartTooltipValue = number | string | ReadonlyArray<number | string>;
type ChartTooltipName = number | string;

function tooltipValueToNumber(value: ChartTooltipValue | undefined) {
  if (Array.isArray(value)) return Number(value[0] ?? 0);
  return Number(value ?? 0);
}

function tooltipNameToText(name: ChartTooltipName | undefined) {
  return String(name ?? "");
}

function joinClassNames(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

export default function LLMUsagePage() {
  const {
    t, locale,
    activeSection, setActiveSection,
    daysFilter, setDaysFilter,
    limit, setLimit,
    forecastMode, setForecastMode,
    forecastMonth, setForecastMonth,
    selectedMonth,
    logPage, setLogPage,
    providerSortKey, setProviderSortKey,
    providerSortDir, setProviderSortDir,
    purposeSortKey, setPurposeSortKey,
    purposeSortDir, setPurposeSortDir,
    reliabilitySortKey,
    reliabilitySortDir,
    modelSortKey, setModelSortKey,
    modelSortDir, setModelSortDir,
    logSortKey, setLogSortKey,
    logSortDir, setLogSortDir,
    loading, error,
    settings,
    totals,
    providerCardRows,
    currentMonthProviderTableRows,
    currentMonthPurposeTableRows,
    currentMonthExecutionTableRows,
    groupedByDate,
    dailyChartRows,
    chartProviders,
    providerColorMap,
    mergedModelRows,
    modelChartRows,
    sortedLogs,
    pagedLogs,
    logsPageSize,
    availableForecastMonths,
    monthlyForecast,
    railSections,
    activeSectionTitle,
    activeSectionDescription,
    totalCostLabel,
    handleReliabilitySort,
    toggleSort,
  } = useLLMUsageData();

  return (
    <div className="space-y-6 overflow-x-hidden">
      <PageHeader
        eyebrow={t("llm.title")}
        title={t("llm.title")}
        titleIcon={Brain}
        description={t("llm.subtitle")}
        compact
        meta={
          <>
            <span>{`${t("llm.currentMonth")}: ${settings?.current_month?.month_jst ?? "—"}`}</span>
            <span>{`${t("llm.totalCost")}: ${fmtUSD(totals.cost)}`}</span>
            <span>{`${t("llm.totalCalls")}: ${fmtNum(totals.calls)}`}</span>
          </>
        }
        actions={
          <div className="grid w-full grid-cols-2 gap-2 sm:flex sm:w-auto sm:flex-wrap sm:justify-end">
            <label className="text-sm">
              <span className="mb-1 block text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("llm.days")}</span>
              <select
                value={daysFilter}
                onChange={(e) => setDaysFilter(e.target.value as "7" | "14" | "30" | "90" | "mtd" | "prev_month")}
                className="min-h-10 w-full rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm text-[var(--color-editorial-ink)]"
              >
                {(["7", "14", "30", "90"] as const).map((d) => (
                  <option key={d} value={d}>
                    {`${d}${t("llm.daysSuffix")}`}
                  </option>
                ))}
                <option value="mtd">{t("llm.currentMonth")}</option>
                <option value="prev_month">{t("llm.previousMonth")}</option>
              </select>
            </label>
            <label className="text-sm">
              <span className="mb-1 block text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("llm.limit")}</span>
              <select
                value={limit}
                onChange={(e) => setLimit(Number(e.target.value))}
                className="min-h-10 w-full rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm text-[var(--color-editorial-ink)]"
              >
                {[50, 100, 200, 500].map((v) => (
                  <option key={v} value={v}>
                    {v}
                  </option>
                ))}
              </select>
            </label>
          </div>
        }
      />

      {loading && <p className="text-sm text-[var(--color-editorial-ink-soft)]">{t("common.loading")}</p>}
      {error && (
        <div className="rounded-[16px] border border-[var(--color-editorial-error-line)] bg-[var(--color-editorial-error-soft)] px-4 py-3 text-sm text-[var(--color-editorial-error)]">
          {error}
        </div>
      )}

      <div className="grid gap-6 xl:grid-cols-[248px_minmax(0,1fr)]">
        <aside className="space-y-4">
          <SectionCard>
            <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              Usage Sections
            </div>
            <div className="mt-4 grid gap-0">
              {railSections.map((section) => (
                <button
                  key={section.id}
                  type="button"
                  onClick={() => setActiveSection(section.id)}
                  className={joinClassNames(
                    "relative border-t border-[var(--color-editorial-line)] px-4 py-3 text-left first:border-t-0 first:pt-0",
                    activeSection === section.id
                      ? "bg-[linear-gradient(90deg,rgba(243,236,227,0.92),rgba(243,236,227,0.28)_78%,transparent)]"
                      : ""
                  )}
                >
                  {activeSection === section.id ? (
                    <span className="absolute bottom-3 left-0 top-3 w-[3px] rounded-full bg-[var(--color-editorial-ink)] first:top-0" />
                  ) : null}
                  <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{section.label}</div>
                  <div className="mt-1 text-[11px] leading-5 text-[var(--color-editorial-ink-soft)] sm:text-xs">{section.meta}</div>
                </button>
              ))}
            </div>
          </SectionCard>

          <SectionCard>
            <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              Status
            </div>
            <div className="mt-4 grid gap-3">
              <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3">
                <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("llmUsage.status.currentMonth")}</div>
                <div className="mt-1 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                  {selectedMonth ?? "—"} / {fmtUSD(currentMonthProviderTableRows.reduce((acc, row) => acc + row.estimated_cost_usd, 0))}
                </div>
              </div>
              <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3">
                <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("llmUsage.status.budget")}</div>
                <div className="mt-1 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                  {monthlyForecast?.budget == null ? "—" : `${fmtUSD(monthlyForecast.budget)} / ${fmtUSD(monthlyForecast.forecastTotal)}`}
                </div>
              </div>
              <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3">
                <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("llmUsage.status.reliability")}</div>
                <div className="mt-1 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                  {currentMonthExecutionTableRows.length} rows / {fmtNum(sortedLogs.length)} logs
                </div>
              </div>
            </div>
          </SectionCard>
        </aside>

        <div className="min-w-0 space-y-6">
          <SectionCard>
            <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              {railSections.find((section) => section.id === activeSection)?.label}
            </div>
            <h2 className="mt-2 font-serif text-[1.8rem] leading-[1.1] tracking-[-0.03em] text-[var(--color-editorial-ink)]">
              {activeSectionTitle}
            </h2>
            <p className="mt-3 max-w-3xl text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
              {activeSectionDescription}
            </p>
          </SectionCard>

          {activeSection === "overview" ? (
            <>
              <section className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
                <MetricCard label={totalCostLabel} value={fmtUSD(totals.cost)} />
                <MetricCard label={t("llm.totalCalls")} value={fmtNum(totals.calls)} />
                <MetricCard label={t("llm.input")} value={fmtNum(totals.input)} />
                <MetricCard label={t("llm.output")} value={fmtNum(totals.output)} />
              </section>

              <section className="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,2fr)_minmax(0,1.2fr)]">
                <SectionCard>
                  <div className="mb-3 flex items-center justify-between">
                    <h2 className="font-serif text-[1.25rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("llm.providerCost")}</h2>
                    <span className="text-xs text-[var(--color-editorial-ink-faint)]">
                      {providerCardRows.length} {t("llm.providers")}
                    </span>
                  </div>
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 2xl:grid-cols-3">
                    {providerCardRows.map((row) => (
                      <MetricCard
                        key={row.provider}
                        className="w-full"
                        label={`${providerLabel(row.provider)}${row.selectedCost <= 0 && row.monthCost > 0 ? " (MTD)" : ""}`}
                        value={fmtUSD(row.selectedCost > 0 ? row.selectedCost : row.monthCost)}
                      />
                    ))}
                  </div>
                </SectionCard>

                <SectionCard>
                  <div className="mb-3 flex items-center justify-between">
                    <h2 className="font-serif text-[1.25rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("llm.cacheTitle")}</h2>
                    <span className="text-xs text-[var(--color-editorial-ink-faint)]">{totals.input > 0 ? ((totals.cacheRead / totals.input) * 100).toFixed(1) : "0.0"}%</span>
                  </div>
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-3 xl:grid-cols-1 2xl:grid-cols-3">
                    <MetricCard className="w-full" label={t("llm.cacheWriteTokens")} value={fmtNum(totals.cacheWrite)} />
                    <MetricCard className="w-full" label={t("llm.cacheReadTokens")} value={fmtNum(totals.cacheRead)} />
                    <MetricCard
                      className="w-full"
                      label={t("llm.cacheReadRatio")}
                      value={`${totals.input > 0 ? ((totals.cacheRead / totals.input) * 100).toFixed(1) : "0.0"}%`}
                    />
                  </div>
                </SectionCard>
              </section>

              <SectionCard>
                <div className="mb-3 flex items-center justify-between">
                  <h2 className="inline-flex items-center gap-2 font-serif text-[1.25rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                    <CalendarDays className="size-4 text-[var(--color-editorial-ink-faint)]" aria-hidden="true" />
                    <span>{t("llm.dailyCostTrend")}</span>
                  </h2>
                  <span className="text-xs text-[var(--color-editorial-ink-faint)]">{dailyChartRows.length} days</span>
                </div>
                {dailyChartRows.length === 0 ? (
                  <p className="text-sm text-[var(--color-editorial-ink-faint)]">{t("llm.noSummary")}</p>
                ) : (
                  <div className="h-72 w-full overflow-visible rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-3">
                    <ResponsiveContainer width="100%" height="100%">
                      <AreaChart data={dailyChartRows} margin={{ top: 8, right: 8, left: 8, bottom: 0 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="#d9d1c4" vertical={false} />
                        <XAxis dataKey="date" tick={{ fontSize: 12, fill: "#8f877f" }} tickLine={false} axisLine={false} />
                        <YAxis
                          tick={{ fontSize: 12, fill: "#8f877f" }}
                          tickLine={false}
                          axisLine={false}
                          tickFormatter={(v) => fmtUSDShort(Number(v))}
                        />
                        <Tooltip
                          formatter={(value: ChartTooltipValue | undefined, name?: ChartTooltipName) => [
                            fmtUSD(tooltipValueToNumber(value)),
                            providerLabel(tooltipNameToText(name)),
                          ]}
                          labelFormatter={(label) => `${label}`}
                          contentStyle={{ borderRadius: 16, borderColor: "#d9d1c4", background: "#fff" }}
                        />
                        {chartProviders.map((provider) => {
                          const colors = providerColorMap.get(provider);
                          if (!colors) return null;
                          return (
                            <Area
                              key={provider}
                              type="monotone"
                              dataKey={provider}
                              name={providerLabel(provider)}
                              stackId="cost"
                              stroke={colors.stroke}
                              fill={colors.fill}
                              fillOpacity={colors.fillOpacity}
                            />
                          );
                        })}
                      </AreaChart>
                    </ResponsiveContainer>
                  </div>
                )}
              </SectionCard>
            </>
          ) : null}

          {activeSection === "forecast" ? (
            <SectionCard>
              <div className="mb-3 flex items-center justify-between">
                <h2 className="inline-flex items-center gap-2 font-serif text-[1.25rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                  <CalendarDays className="size-4 text-[var(--color-editorial-ink-faint)]" aria-hidden="true" />
                  <span>{t("llm.monthEndForecast")}</span>
                </h2>
                <div className="flex items-center gap-2">
                  <select
                    value={forecastMonth ?? ""}
                    onChange={(e) => setForecastMonth(e.target.value)}
                    className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-1.5 text-xs text-[var(--color-editorial-ink)]"
                  >
                    {availableForecastMonths.map((m) => (
                      <option key={m} value={m}>
                        {m}
                      </option>
                    ))}
                  </select>
                  <div className="grid min-w-0 grid-cols-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-0.5 text-xs">
                    <button
                      type="button"
                      onClick={() => setForecastMode("month_avg")}
                      className={joinClassNames(
                        "min-w-0 rounded-full px-2 py-1 text-center sm:px-3",
                        forecastMode === "month_avg" ? "bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]" : "text-[var(--color-editorial-ink-soft)]"
                      )}
                    >
                      {t("llm.forecast.refMonthAvg")}
                    </button>
                    <button
                      type="button"
                      onClick={() => setForecastMode("recent_7d")}
                      className={joinClassNames(
                        "min-w-0 rounded-full px-2 py-1 text-center sm:px-3",
                        forecastMode === "recent_7d" ? "bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]" : "text-[var(--color-editorial-ink-soft)]"
                      )}
                    >
                      {t("llm.forecast.refRecent7d")}
                    </button>
                  </div>
                </div>
              </div>
              {!monthlyForecast ? (
                <p className="text-sm text-[var(--color-editorial-ink-faint)]">{t("common.loading")}</p>
              ) : (
                <div className="space-y-4">
                  <div className="grid grid-cols-1 gap-3 min-[520px]:grid-cols-2 lg:grid-cols-4">
                    <MetricCard className="w-full" label={t("llm.monthToDate")} value={fmtUSD(monthlyForecast.actualTotal)} />
                    <MetricCard className="w-full" label={monthlyForecast.isCurrentMonth ? t("llm.forecastEom") : t("llm.monthTotal")} value={fmtUSD(monthlyForecast.forecastTotal)} />
                    <MetricCard className="w-full" label={t("llm.currentPacePerDay")} value={fmtUSD(monthlyForecast.dailyPace)} />
                    <MetricCard
                      className="w-full"
                      label={t("llm.budgetDelta")}
                      value={
                        monthlyForecast.budget == null
                          ? "—"
                          : `${monthlyForecast.forecastTotal - monthlyForecast.budget >= 0 ? "+" : ""}${fmtUSD(monthlyForecast.forecastTotal - monthlyForecast.budget)}`
                      }
                    />
                  </div>
                  <p className="text-xs text-[var(--color-editorial-ink-soft)]">
                    {monthlyForecast.isCurrentMonth
                      ? `${t("llm.forecast.modeLabel")} ${forecastMode === "month_avg" ? t("llm.forecast.monthAvg") : t("llm.forecast.recent7d")}${t("llm.forecast.refOpen")}${t("llm.forecast.refMonthAvg")} ${fmtUSD(monthlyForecast.monthAvgDailyPace)} / ${t("llm.forecast.refRecent7d")} ${fmtUSD(monthlyForecast.recent7dDailyPace)}${t("llm.forecast.refClose")}`
                      : t("llm.forecast.pastMonthsOnly")}
                  </p>
                  <div className="h-80 w-full overflow-visible rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-3">
                    <ResponsiveContainer width="100%" height="100%">
                      <LineChart data={monthlyForecast.rows} margin={{ top: 8, right: 16, left: 8, bottom: 0 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="#d9d1c4" vertical={false} />
                        <XAxis dataKey="label" tick={{ fontSize: 12, fill: "#8f877f" }} tickLine={false} axisLine={false} />
                        <YAxis
                          tick={{ fontSize: 12, fill: "#8f877f" }}
                          tickLine={false}
                          axisLine={false}
                          tickFormatter={(v) => fmtUSDShort(Number(v))}
                        />
                        <Tooltip
                          formatter={(value: ChartTooltipValue | undefined, name?: ChartTooltipName) => [
                            fmtUSD(tooltipValueToNumber(value)),
                            name === "actual" ? t("llm.actualCumulative") : t("llm.forecastLabel"),
                          ]}
                          labelFormatter={(label) => `${monthlyForecast.monthLabel}-${String(label).padStart(2, "0")}`}
                          contentStyle={{ borderRadius: 16, borderColor: "#d9d1c4", background: "#fff" }}
                        />
                        {monthlyForecast.budget != null && (
                          <ReferenceLine
                            y={monthlyForecast.budget}
                            stroke="#c05032"
                            strokeDasharray="5 5"
                            label={{
                              value: `${t("llm.budget")} ${fmtUSDShort(monthlyForecast.budget)}`,
                              fill: "#c05032",
                              fontSize: 11,
                              position: "insideTopRight",
                            }}
                          />
                        )}
                        <Line type="monotone" dataKey="actual" name="actual" stroke="#171412" strokeWidth={2.5} dot={false} connectNulls={false} />
                        {monthlyForecast.isCurrentMonth ? (
                          <Line type="monotone" dataKey="forecast" name="forecast" stroke="#275d8a" strokeWidth={2} strokeDasharray="6 4" dot={false} connectNulls={false} />
                        ) : null}
                      </LineChart>
                    </ResponsiveContainer>
                  </div>
                </div>
              )}
            </SectionCard>
          ) : null}

          {activeSection === "daily" ? (
            <SectionCard>
              <DailySummaryGroups
                title={t("llm.dailySummary")}
                groupedByDate={groupedByDate}
                noSummaryLabel={t("llm.noSummary")}
                fmtNum={fmtNum}
                fmtUSD={fmtUSD}
              />
            </SectionCard>
          ) : null}

          {activeSection === "providers" ? (
            <CurrentMonthByProviderTable
              title={t("llm.currentMonthByProvider")}
              rows={currentMonthProviderTableRows}
              monthLabel={selectedMonth ?? currentMonthProviderTableRows[0]?.month_jst ?? "—"}
              totalCostLabel={fmtUSD(currentMonthProviderTableRows.reduce((acc, row) => acc + row.estimated_cost_usd, 0))}
              noSummaryLabel={t("llm.noSummary")}
              fmtNum={fmtNum}
              fmtUSD={fmtUSD}
              sortKey={providerSortKey}
              sortDir={providerSortDir}
              onSort={(key) => toggleSort(key, providerSortKey, setProviderSortKey, setProviderSortDir)}
            />
          ) : null}

          {activeSection === "purposes" ? (
            <CurrentMonthByPurposeTable
              title={t("llm.currentMonthByPurpose")}
              rows={currentMonthPurposeTableRows}
              monthLabel={selectedMonth ?? currentMonthPurposeTableRows[0]?.month_jst ?? "—"}
              noSummaryLabel={t("llm.noSummary")}
              fmtNum={fmtNum}
              fmtUSD={fmtUSD}
              sortKey={purposeSortKey}
              sortDir={purposeSortDir}
              onSort={(key) => toggleSort(key, purposeSortKey, setPurposeSortKey, setPurposeSortDir)}
            />
          ) : null}

          {activeSection === "reliability" ? (
            <ReliabilityTable
              rows={currentMonthExecutionTableRows}
              monthLabel={selectedMonth ?? currentMonthExecutionTableRows[0]?.month_jst ?? "—"}
              noSummaryLabel={t("llm.noSummary")}
              fmtNum={fmtNum}
              fmtUSD={fmtUSD}
              sortKey={reliabilitySortKey}
              sortDir={reliabilitySortDir}
              onSort={handleReliabilitySort}
              labels={{
                title: t("llm.currentMonthReliability"),
                attempts: t("llm.attempts"),
                cost: t("llm.totalCost"),
                failures: t("llm.failures"),
                failureRate: t("llm.failureRate"),
                retries: t("llm.retries"),
                retryRate: t("llm.retryRate"),
                emptyResponses: t("llm.emptyResponses"),
                emptyRate: t("llm.emptyRate"),
              }}
            />
          ) : null}

          {activeSection === "models" ? (
            <section className="rounded-lg border border-zinc-200 bg-white p-4">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="inline-flex items-center gap-2 text-sm font-semibold text-zinc-800">
            <Brain className="size-4 text-zinc-500" aria-hidden="true" />
            <span>{t("llm.usageByModel")}</span>
          </h2>
          <span className="text-xs text-zinc-400">{mergedModelRows.length} models</span>
        </div>
        {mergedModelRows.length === 0 ? (
          <p className="text-sm text-zinc-400">{t("llm.noSummary")}</p>
        ) : (
          <div className="space-y-4">
            <div className="h-80 w-full overflow-visible rounded border border-zinc-100 p-2">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart
                  data={modelChartRows}
                  layout="vertical"
                  margin={{ top: 8, right: 24, left: 8, bottom: 8 }}
                >
                  <CartesianGrid strokeDasharray="3 3" stroke="#e4e4e7" horizontal={true} vertical={false} />
                  <XAxis
                    type="number"
                    tick={{ fontSize: 12, fill: "#71717a" }}
                    tickLine={false}
                    axisLine={false}
                    tickFormatter={(v) => fmtUSDShort(Number(v))}
                  />
                  <YAxis
                    type="category"
                    dataKey="shortLabel"
                    width={220}
                    tick={{ fontSize: 12, fill: "#3f3f46" }}
                    tickLine={false}
                    axisLine={false}
                  />
                  <Tooltip
                    formatter={(value: ChartTooltipValue | undefined, name?: ChartTooltipName) => [
                      name === "calls" ? fmtNum(tooltipValueToNumber(value)) : fmtUSD(tooltipValueToNumber(value)),
                      tooltipNameToText(name),
                    ]}
                    labelFormatter={(_, payload) => {
                      const row = payload?.[0]?.payload as { label?: string; pricingSource?: string } | undefined;
                      if (!row) return "";
                      return `${row.label} (${row.pricingSource ?? ""})`;
                    }}
                    contentStyle={{ borderRadius: 10, borderColor: "#e4e4e7" }}
                  />
                  <Bar dataKey="cost" name="Cost (USD)" fill="#18181b" radius={[0, 4, 4, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
            <div className="overflow-x-auto">
              <table className="min-w-full text-sm">
                <thead className="text-xs text-zinc-500">
                  <tr className="border-b border-zinc-100">
                    <th className="px-3 py-2 text-left font-medium"><button type="button" onClick={() => toggleSort("provider", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">provider{modelSortKey === "provider" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                    <th className="px-3 py-2 text-left font-medium"><button type="button" onClick={() => toggleSort("model", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">model{modelSortKey === "model" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                    <th className="px-3 py-2 text-left font-medium"><button type="button" onClick={() => toggleSort("pricing_source", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">pricing{modelSortKey === "pricing_source" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                    <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("calls", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">calls{modelSortKey === "calls" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                    <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("input_tokens", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">input{modelSortKey === "input_tokens" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                    <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("output_tokens", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">output{modelSortKey === "output_tokens" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                    <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("avg_input_tokens_per_call", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">avg in/call{modelSortKey === "avg_input_tokens_per_call" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                    <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("avg_output_tokens_per_call", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">avg out/call{modelSortKey === "avg_output_tokens_per_call" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                    <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("cache_creation_input_tokens", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">cache w{modelSortKey === "cache_creation_input_tokens" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                    <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("cache_read_input_tokens", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">cache r{modelSortKey === "cache_read_input_tokens" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                    <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("avg_cost_per_call_usd", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">avg/call{modelSortKey === "avg_cost_per_call_usd" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                    <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("estimated_cost_usd", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">cost{modelSortKey === "estimated_cost_usd" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                  </tr>
                </thead>
                <tbody>
                  {mergedModelRows.map((r) => (
                    <tr key={`${r.provider}:${r.model}:${r.pricing_source}`} className="border-b border-zinc-100 last:border-0">
                      <td className="px-3 py-2">{providerLabel(r.provider)}</td>
                      <td className="px-3 py-2 text-xs whitespace-nowrap">{r.model}</td>
                      <td className="px-3 py-2 text-xs">
                        <div className="relative inline-flex items-center gap-1">
                          <span>{r.pricing_source}</span>
                          {"pricing_sources" in r &&
                            Array.isArray(r.pricing_sources) &&
                            r.pricing_sources.length > 1 && (
                              <span className="group relative inline-flex">
                                <button
                                  type="button"
                                  className="inline-flex size-4 items-center justify-center rounded-full border border-zinc-300 text-[10px] leading-none text-zinc-500 hover:bg-zinc-50"
                                  aria-label="pricing source breakdown"
                                >
                                  i
                                </button>
                                <span className="pointer-events-none absolute left-1/2 top-full z-20 mt-1 hidden w-72 -translate-x-1/2 rounded-md border border-zinc-200 bg-white p-2 text-[11px] text-zinc-700 shadow-lg group-hover:block">
                                  <span className="mb-1 block font-medium text-zinc-900">Pricing sources</span>
                                  <span className="block whitespace-pre-line">
                                    {r.pricing_sources.join("\n")}
                                  </span>
                                </span>
                              </span>
                            )}
                        </div>
                      </td>
                      <td className="px-3 py-2 text-right">{fmtNum(r.calls)}</td>
                      <td className="px-3 py-2 text-right">{fmtNum(r.input_tokens)}</td>
                      <td className="px-3 py-2 text-right">{fmtNum(r.output_tokens)}</td>
                      <td className="px-3 py-2 text-right">{fmtNum(Math.round(r.avg_input_tokens_per_call ?? 0))}</td>
                      <td className="px-3 py-2 text-right">{fmtNum(Math.round(r.avg_output_tokens_per_call ?? 0))}</td>
                      <td className="px-3 py-2 text-right">{fmtNum(r.cache_creation_input_tokens)}</td>
                      <td className="px-3 py-2 text-right">{fmtNum(r.cache_read_input_tokens)}</td>
                      <td className="px-3 py-2 text-right">{fmtUSD(r.calls > 0 ? r.estimated_cost_usd / r.calls : 0)}</td>
                      <td className="px-3 py-2 text-right">{fmtUSD(r.estimated_cost_usd)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
            </section>
          ) : null}

          {activeSection === "logs" ? (
            <RecentLogsTable
              logs={sortedLogs}
              pagedLogs={pagedLogs}
              logPage={logPage}
              setLogPage={setLogPage}
              logsPageSize={logsPageSize}
              locale={locale}
              noLogsLabel={t("llm.noLogs")}
              labels={{ title: t("llm.recentLogs"), time: t("llm.time") }}
              fmtNum={fmtNum}
              fmtUSD={fmtUSD}
              sortKey={logSortKey}
              sortDir={logSortDir}
              onSort={(key) => toggleSort(key, logSortKey, setLogSortKey, setLogSortDir)}
            />
          ) : null}
        </div>
      </div>
    </div>
  );
}
