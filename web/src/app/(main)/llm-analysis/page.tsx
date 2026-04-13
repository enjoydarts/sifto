"use client";

import Link from "next/link";
import { Brain, Filter, TableProperties } from "lucide-react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  ResponsiveContainer,
  Scatter,
  ScatterChart,
  Tooltip,
  XAxis,
  YAxis,
  ZAxis,
} from "recharts";
import { formatModelDisplayName, normalizeProvider, providerLabel } from "@/lib/model-display";
import ModelSelect from "@/components/settings/model-select";
import { PageHeader } from "@/components/ui/page-header";
import { SectionCard } from "@/components/ui/section-card";
import {
  fmtUSD,
  type RankedModelRow,
  type UsageScatterRow,
  useLLMAnalysisPageData,
} from "./use-llm-analysis-data";

function fmtNum(v: number) {
  return new Intl.NumberFormat("ja-JP").format(v);
}

function providerColor(provider: string) {
  switch (normalizeProvider(provider)) {
    case "openai":
      return "#10b981";
    case "anthropic":
      return "#3b82f6";
    case "google":
      return "#f59e0b";
    case "groq":
      return "#8b5cf6";
    case "deepseek":
      return "#ef4444";
    case "alibaba":
      return "#14b8a6";
    case "minimax":
      return "#65a30d";
    case "mistral":
      return "#fb7185";
    case "together":
      return "#0ea5a4";
    case "xai":
      return "#818cf8";
    case "zai":
      return "#22d3ee";
    case "fireworks":
      return "#f97316";
    case "moonshot":
      return "#db2777";
    case "poe":
      return "#0f766e";
    case "siliconflow":
      return "#2563eb";
    default:
      return "#71717a";
  }
}

function joinClassNames(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

type ChartTooltipValue = number | string | ReadonlyArray<number | string>;
type ChartTooltipName = number | string;

function tooltipValueToNumber(value: ChartTooltipValue | undefined) {
  if (Array.isArray(value)) return Number(value[0] ?? 0);
  return Number(value ?? 0);
}

function tooltipValueToText(value: ChartTooltipValue | undefined) {
  if (Array.isArray(value)) return value.map(String).join(", ");
  return String(value ?? "");
}

function tooltipNameToText(name: ChartTooltipName | undefined) {
  return String(name ?? "");
}

function UsageScatterChart({
  rows,
  variant,
  t,
  onPointClick,
}: {
  rows: UsageScatterRow[];
  variant: "efficiency" | "costCalls";
  t: (key: string) => string;
  onPointClick: (row: UsageScatterRow) => void;
}) {
  const isEfficiency = variant === "efficiency";
  return (
    <div className="h-[24rem] w-full">
      <ResponsiveContainer width="100%" height="100%">
        <ScatterChart margin={{ top: 12, right: 20, left: 8, bottom: 12 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#d9d1c4" />
          <XAxis
            type="number"
            dataKey={isEfficiency ? "totalTokensPerCall" : "calls"}
            name={isEfficiency ? "Tokens / call" : "Calls"}
            tick={{ fontSize: 12, fill: "#8f877f" }}
            tickLine={false}
            axisLine={false}
            tickFormatter={(v) => fmtNum(Number(v))}
          />
          <YAxis
            type="number"
            dataKey="avgCostPerCall"
            name="Cost / call"
            tick={{ fontSize: 12, fill: "#8f877f" }}
            tickLine={false}
            axisLine={false}
            tickFormatter={(v) => fmtUSD(Number(v))}
          />
          <ZAxis type="number" dataKey={isEfficiency ? "bubbleSize" : "costCallBubbleSize"} range={[80, 460]} />
          <Tooltip
            cursor={{ strokeDasharray: "4 4" }}
            formatter={(value: ChartTooltipValue | undefined, name?: ChartTooltipName) => {
              if (name === "Cost / call") return [fmtUSD(tooltipValueToNumber(value)), tooltipNameToText(name)];
              if (name === "Tokens / call" || name === "Calls") return [fmtNum(tooltipValueToNumber(value)), tooltipNameToText(name)];
              return [tooltipValueToText(value), tooltipNameToText(name)];
            }}
            labelFormatter={(_, payload) => {
              const row = payload?.[0]?.payload as UsageScatterRow | undefined;
              if (!row) return "";
              return `${providerLabel(row.provider)} / ${formatModelDisplayName(row.model)} (${row.purpose})`;
            }}
            content={({ active, payload }) => {
              const row = payload?.[0]?.payload as UsageScatterRow | undefined;
              if (!active || !row) return null;
              return (
                <div className="rounded-[14px] border border-[var(--color-editorial-line)] bg-white px-3 py-2 shadow-[var(--shadow-dropdown)]">
                  <div className="text-xs font-semibold text-[var(--color-editorial-ink)]">{providerLabel(row.provider)} / {formatModelDisplayName(row.model)}</div>
                  <div className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{row.purpose}</div>
                  <div className="mt-2 grid gap-1 text-xs text-[var(--color-editorial-ink-soft)]">
                    <div>{t("llm.totalCalls")}: {fmtNum(row.calls)}</div>
                    <div>tokens/call: {fmtNum(row.totalTokensPerCall)}</div>
                    <div>avg cost/call: {fmtUSD(row.avgCostPerCall)}</div>
                    <div>total cost: {fmtUSD(row.estimated_cost_usd)}</div>
                    <div>pricing: {row.pricingLabel}</div>
                  </div>
                </div>
              );
            }}
          />
          <Scatter
            data={rows}
            name={isEfficiency ? "Calls" : "Tokens / call"}
            shape={(props: { cx?: number; cy?: number; payload?: UsageScatterRow }) => {
              const row = props.payload;
              if (props.cx == null || props.cy == null || !row) return null;
              return (
                <circle
                  cx={props.cx}
                  cy={props.cy}
                  r={isEfficiency ? row.bubbleSize : row.costCallBubbleSize}
                  fill={providerColor(row.provider)}
                  fillOpacity={0.72}
                  stroke="#ffffff"
                  strokeWidth={2}
                  className="cursor-pointer transition-opacity hover:opacity-100"
                  onClick={() => onPointClick(row)}
                />
              );
            }}
          />
        </ScatterChart>
      </ResponsiveContainer>
    </div>
  );
}

export default function LLMAnalysisPage() {
  const {
    t,
    activeSection,
    setActiveSection,
    days,
    setDays,
    providerFilter,
    setProviderFilter,
    purposeFilter,
    setPurposeFilter,
    scatterPurpose,
    setScatterPurpose,
    rankingPurpose,
    setRankingPurpose,
    modelQuery,
    setModelQuery,
    selectedModelKey,
    setSelectedModelKey,
    loading,
    error,
    providers,
    purposes,
    modelOptions,
    sortedRows,
    totals,
    scatterRows,
    providerMixRows,
    rankingRows,
    bestCostModel,
    bestTokenModel,
    bestQualityModel,
    rankingMedianCost,
    rankingPurposeLabel,
    rankingModelOptions,
    selectedRankingRow,
    toggleSort,
    sortMark,
    applyRowFilter,
    clearFilters,
    hasActiveDrilldown,
    hasScatterFilter,
    clearScatterFilters,
    railSections,
    activeSectionTitle,
    activeSectionDescription,
    qaLoading,
    qaSamples,
  } = useLLMAnalysisPageData();

  return (
    <div className="space-y-6 overflow-x-hidden">
      <PageHeader
        eyebrow={t("llmAnalysis.title")}
        title={t("llmAnalysis.title")}
        titleIcon={TableProperties}
        description={t("llmAnalysis.subtitle")}
        compact
        meta={
          <>
            <span>{`${t("llm.totalCost")}: ${fmtUSD(totals.cost)}`}</span>
            <span>{`${t("llm.totalCalls")}: ${fmtNum(totals.calls)}`}</span>
            <span>{`${t("llm.input")}: ${fmtNum(totals.input)}`}</span>
            <span>{`${t("llm.output")}: ${fmtNum(totals.output)}`}</span>
          </>
        }
        actions={
          <Link
            href="/llm-usage"
            className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] press focus-ring"
          >
            <Brain className="size-4" aria-hidden="true" />
            <span>{t("llmAnalysis.backToUsage")}</span>
          </Link>
        }
      />

      {loading && <p className="text-sm text-[var(--color-editorial-ink-soft)]">{t("common.loading")}</p>}
      {error && <div className="rounded-[16px] border border-[var(--color-editorial-error-line)] bg-[var(--color-editorial-error-soft)] px-4 py-3 text-sm text-[var(--color-editorial-error)]">{error}</div>}

      <SectionCard>
        <div className="mb-3 flex items-center gap-2 text-sm font-semibold text-[var(--color-editorial-ink)]">
          <Filter className="size-4 text-[var(--color-editorial-ink-faint)]" aria-hidden="true" />
          <span>{t("llmAnalysis.filters")}</span>
        </div>
        <div className="grid gap-3 md:grid-cols-4">
          <label className="text-sm">
            <span className="mb-1 block text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("llm.days")}</span>
            <select value={days} onChange={(e) => setDays(e.target.value as typeof days)} className="w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)]">
              <option value="14">14{t("llm.daysSuffix")}</option>
              <option value="30">30{t("llm.daysSuffix")}</option>
              <option value="90">90{t("llm.daysSuffix")}</option>
              <option value="180">180{t("llm.daysSuffix")}</option>
            </select>
          </label>
          <label className="text-sm">
            <span className="mb-1 block text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("llmAnalysis.provider")}</span>
            <select value={providerFilter} onChange={(e) => setProviderFilter(e.target.value)} className="w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)]">
              <option value="all">{t("llmAnalysis.all")}</option>
              {providers.map((provider) => (
                <option key={provider} value={provider}>{providerLabel(provider)}</option>
              ))}
            </select>
          </label>
          <label className="text-sm">
            <span className="mb-1 block text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("llmAnalysis.purpose")}</span>
            <select value={purposeFilter} onChange={(e) => setPurposeFilter(e.target.value)} className="w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)]">
              <option value="all">{t("llmAnalysis.all")}</option>
              {purposes.map((purpose) => (
                <option key={purpose} value={purpose}>{purpose}</option>
              ))}
            </select>
          </label>
          <label className="text-sm">
            <span className="mb-1 block text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("llmAnalysis.modelSearch")}</span>
            <ModelSelect
              label={t("llmAnalysis.modelSearch")}
              value={modelQuery}
              onChange={setModelQuery}
              options={modelOptions}
              showMeta={false}
              hideLabel
              labels={{
                defaultOption: t("llmAnalysis.modelSearchDefault"),
                searchPlaceholder: t("settings.modelSelect.searchPlaceholder"),
                noResults: t("settings.modelSelect.noResults"),
                providerAll: t("settings.modelSelect.providerAll"),
                modalChoose: t("settings.modelSelect.modalChoose"),
                close: t("common.close"),
                confirmTitle: t("settings.modelSelect.confirmTitle"),
                confirmYes: t("settings.modelSelect.confirmYes"),
                confirmNo: t("settings.modelSelect.confirmNo"),
                confirmSuffix: t("settings.modelSelect.confirmSuffix"),
                providerColumn: t("settings.modelSelect.providerColumn"),
                modelColumn: t("settings.modelSelect.modelColumn"),
                pricingColumn: t("settings.modelSelect.pricingColumn"),
              }}
            />
          </label>
        </div>
        {hasActiveDrilldown ? (
          <div className="mt-3 flex items-center justify-between gap-3 rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-3">
            <div className="text-xs text-[var(--color-editorial-ink-soft)]">{t("llmAnalysis.drilldownActive")}</div>
            <button
              type="button"
              onClick={clearFilters}
              className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1.5 text-xs font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
            >
              {t("llmAnalysis.clearDrilldown")}
            </button>
          </div>
        ) : null}
      </SectionCard>

      <div className="grid gap-6 xl:grid-cols-[248px_minmax(0,1fr)]">
        <aside className="space-y-4">
          <SectionCard>
            <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              Analysis Sections
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
                <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("llm.totalCost")}</div>
                <div className="mt-1 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                  {fmtUSD(totals.cost)} / {fmtNum(totals.calls)} calls
                </div>
              </div>
              <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3">
                <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("llmAnalysis.section.mix")}</div>
                <div className="mt-1 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                  {providerMixRows.length} purposes / {providers.length} providers
                </div>
              </div>
              <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3">
                <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("llmAnalysis.section.details")}</div>
                <div className="mt-1 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                  {sortedRows.length} rows / {rankingRows.length} ranked
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
                <InsightCard title={t("llm.totalCost")} body={fmtUSD(totals.cost)} meta={`${fmtNum(totals.calls)} calls`} />
                <InsightCard title={t("llm.input")} body={fmtNum(totals.input)} meta={`${fmtNum(totals.output)} output`} />
                <InsightCard
                  title={t("llmAnalysis.insight.topCost")}
                  body={sortedRows[0] ? `${providerLabel(sortedRows[0].provider)} / ${formatModelDisplayName(sortedRows[0].model)}` : "—"}
                  meta={sortedRows[0] ? `${sortedRows[0].purpose} · ${fmtUSD(sortedRows[0].estimated_cost_usd)}` : "—"}
                />
                <InsightCard
                  title={t("llmAnalysis.insight.topPurpose")}
                  body={providerMixRows[0]?.purpose ?? "—"}
                  meta={providerMixRows[0] ? `${fmtNum(Math.round(Object.values(providerMixRows[0]).filter((value) => typeof value === "number").reduce((sum, value) => sum + Number(value), 0)))}% mix tracked` : "—"}
                />
              </section>

              <section className="grid gap-4 xl:grid-cols-[minmax(0,1.65fr)_minmax(0,1fr)]">
                <SectionCard>
                  <SectionTitle
                    eyebrow={t("llmAnalysis.section.charts")}
                    title={t("llmAnalysis.efficiencyScatter")}
                    description={t("llmAnalysis.efficiencyScatterHelp")}
                    compact
                  />
                  <div className="mt-4 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                    {scatterRows.length === 0 ? (
                      <p className="text-sm text-[var(--color-editorial-ink-faint)]">{t("llm.noSummary")}</p>
                    ) : (
                      <div className="h-[24rem] w-full">
                        <ResponsiveContainer width="100%" height="100%">
                          <ScatterChart margin={{ top: 12, right: 20, left: 8, bottom: 12 }}>
                            <CartesianGrid strokeDasharray="3 3" stroke="#d9d1c4" />
                            <XAxis type="number" dataKey="totalTokensPerCall" name="Tokens / call" tick={{ fontSize: 12, fill: "#8f877f" }} tickLine={false} axisLine={false} tickFormatter={(v) => fmtNum(Number(v))} />
                            <YAxis type="number" dataKey="avgCostPerCall" name="Cost / call" tick={{ fontSize: 12, fill: "#8f877f" }} tickLine={false} axisLine={false} tickFormatter={(v) => fmtUSD(Number(v))} />
                            <ZAxis type="number" dataKey="bubbleSize" range={[80, 460]} />
                            <Tooltip
                              cursor={{ strokeDasharray: "4 4" }}
                              formatter={(value: ChartTooltipValue | undefined, name?: ChartTooltipName) => {
                                if (name === "Cost / call") return [fmtUSD(tooltipValueToNumber(value)), tooltipNameToText(name)];
                                if (name === "Tokens / call") return [fmtNum(tooltipValueToNumber(value)), tooltipNameToText(name)];
                                if (name === "Calls") return [fmtNum(tooltipValueToNumber(value)), tooltipNameToText(name)];
                                return [tooltipValueToText(value), tooltipNameToText(name)];
                              }}
                              labelFormatter={(_, payload) => {
                                const row = payload?.[0]?.payload as (typeof scatterRows)[number] | undefined;
                                if (!row) return "";
                                return `${row.label} (${row.purpose})`;
                              }}
                              content={({ active, payload }) => {
                                const row = payload?.[0]?.payload as (typeof scatterRows)[number] | undefined;
                                if (!active || !row) return null;
                                return (
                                  <div className="rounded-[14px] border border-[var(--color-editorial-line)] bg-white px-3 py-2 shadow-[var(--shadow-dropdown)]">
                                    <div className="text-xs font-semibold text-[var(--color-editorial-ink)]">{providerLabel(row.provider)} / {formatModelDisplayName(row.model)}</div>
                                    <div className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{row.purpose}</div>
                                    <div className="mt-2 grid gap-1 text-xs text-[var(--color-editorial-ink-soft)]">
                                      <div>{t("llm.totalCalls")}: {fmtNum(row.calls)}</div>
                                      <div>tokens/call: {fmtNum(row.totalTokensPerCall)}</div>
                                      <div>avg cost/call: {fmtUSD(row.avgCostPerCall)}</div>
                                      <div>pricing: {row.pricingLabel}</div>
                                    </div>
                                  </div>
                                );
                              }}
                            />
                            <Scatter
                              data={scatterRows}
                              name="Calls"
                              shape={(props: { cx?: number; cy?: number; payload?: (typeof scatterRows)[number] }) => {
                                const row = props.payload;
                                if (props.cx == null || props.cy == null || !row) return null;
                                return (
                                  <circle
                                    cx={props.cx}
                                    cy={props.cy}
                                    r={row.bubbleSize}
                                    fill={providerColor(row.provider)}
                                    fillOpacity={0.72}
                                    stroke="#ffffff"
                                    strokeWidth={2}
                                    className="cursor-pointer transition-opacity hover:opacity-100"
                                    onClick={() => applyRowFilter(row)}
                                  />
                                );
                              }}
                            />
                          </ScatterChart>
                        </ResponsiveContainer>
                      </div>
                    )}
                  </div>
                </SectionCard>

                <SectionCard>
                  <SectionTitle
                    eyebrow={t("llmAnalysis.section.mix")}
                    title={t("llmAnalysis.section.mixTitle")}
                    description={t("llmAnalysis.section.mixDescription")}
                    compact
                  />
                  <div className="mt-4 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                    {providerMixRows.length === 0 ? (
                      <p className="text-sm text-[var(--color-editorial-ink-faint)]">{t("llm.noSummary")}</p>
                    ) : (
                      <div className="h-[24rem] w-full">
                        <ResponsiveContainer width="100%" height="100%">
                          <BarChart data={providerMixRows} layout="vertical" margin={{ top: 8, right: 16, left: 12, bottom: 0 }}>
                            <CartesianGrid strokeDasharray="3 3" stroke="#e4dbcf" horizontal={false} />
                            <XAxis type="number" domain={[0, 100]} tick={{ fontSize: 12, fill: "#8f877f" }} tickFormatter={(v) => `${v}%`} tickLine={false} axisLine={false} />
                            <YAxis type="category" dataKey="purpose" width={130} tick={{ fontSize: 12, fill: "#5f5750" }} tickLine={false} axisLine={false} />
                            <Tooltip
                              formatter={(value: ChartTooltipValue | undefined, name?: ChartTooltipName) => [`${tooltipValueToNumber(value).toFixed(1)}%`, providerLabel(tooltipNameToText(name))]}
                              contentStyle={{ borderRadius: 16, borderColor: "#d9d1c4", background: "#fff" }}
                            />
                            {providers.map((provider, index) => (
                              <Bar key={provider} dataKey={provider} stackId="purpose" fill={providerColor(provider)} radius={index === providers.length - 1 ? [999, 999, 999, 999] : 0}>
                                {providerMixRows.map((row) => (
                                  <Cell key={`${provider}:${row.purpose}`} fill={providerColor(provider)} />
                                ))}
                              </Bar>
                            ))}
                          </BarChart>
                        </ResponsiveContainer>
                      </div>
                    )}
                    <div className="mt-3 flex flex-wrap gap-x-4 gap-y-2 text-xs text-[var(--color-editorial-ink-soft)]">
                      {providers.map((provider) => (
                        <span key={provider} className="inline-flex items-center gap-2">
                          <span className="size-2.5 rounded-full" style={{ backgroundColor: providerColor(provider) }} />
                          {providerLabel(provider)}
                        </span>
                      ))}
                    </div>
                  </div>
                </SectionCard>
              </section>
            </>
          ) : null}

          {activeSection === "charts" ? (
            <SectionCard>
              <SectionTitle eyebrow={t("llmAnalysis.section.charts")} title={t("llmAnalysis.section.chartsTitle")} description={t("llmAnalysis.section.chartsDescription")} />
              <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
                  <div>
                    <h2 className="font-serif text-[1.45rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("llmAnalysis.efficiencyScatter")}</h2>
                    <p className="mt-2 text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("llmAnalysis.efficiencyScatterHelp")}</p>
                  </div>
                  <div className="flex items-center gap-2">
                    {hasScatterFilter ? (
                      <button
                        type="button"
                        onClick={clearScatterFilters}
                        className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-1.5 text-xs font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                      >
                        {t("llmAnalysis.clearDrilldown")}
                      </button>
                    ) : null}
                    <span className="text-xs text-[var(--color-editorial-ink-faint)]">{scatterRows.length} rows</span>
                  </div>
                </div>
                <div className="mb-3 flex flex-wrap gap-2">
                  <button
                    type="button"
                    onClick={() => setScatterPurpose("all")}
                    className={joinClassNames(
                      "rounded-full border px-3 py-1 text-xs font-medium transition",
                      scatterPurpose === "all"
                        ? "border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                        : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                    )}
                  >
                    {t("llmAnalysis.all")}
                  </button>
                  {purposes.map((purpose) => (
                    <button
                      key={purpose}
                      type="button"
                      onClick={() => setScatterPurpose(purpose)}
                      className={joinClassNames(
                        "rounded-full border px-3 py-1 text-xs font-medium transition",
                        scatterPurpose === purpose
                          ? "border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                          : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                      )}
                    >
                      {purpose}
                    </button>
                  ))}
                </div>
                {scatterRows.length === 0 ? (
                  <p className="text-sm text-[var(--color-editorial-ink-faint)]">{t("llm.noSummary")}</p>
                ) : (
                  <div className="space-y-4">
                    <UsageScatterChart rows={scatterRows} variant="efficiency" t={t} onPointClick={applyRowFilter} />
                    <div>
                      <h3 className="font-serif text-[1.2rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("llmAnalysis.costCallsScatter")}</h3>
                      <p className="mt-2 text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("llmAnalysis.costCallsScatterHelp")}</p>
                      <div className="mt-4">
                        <UsageScatterChart rows={scatterRows} variant="costCalls" t={t} onPointClick={applyRowFilter} />
                      </div>
                    </div>
                  </div>
                )}
              </section>
            </SectionCard>
          ) : null}

          {activeSection === "mix" ? (
            <SectionCard>
              <SectionTitle eyebrow={t("llmAnalysis.section.mix")} title={t("llmAnalysis.section.mixTitle")} description={t("llmAnalysis.section.mixDescription")} />
              <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                {providerMixRows.length === 0 ? (
                  <p className="text-sm text-[var(--color-editorial-ink-faint)]">{t("llm.noSummary")}</p>
                ) : (
                  <div className="h-[24rem] w-full">
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart data={providerMixRows} layout="vertical" margin={{ top: 8, right: 16, left: 12, bottom: 0 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="#e4dbcf" horizontal={false} />
                        <XAxis type="number" domain={[0, 100]} tick={{ fontSize: 12, fill: "#8f877f" }} tickFormatter={(v) => `${v}%`} tickLine={false} axisLine={false} />
                        <YAxis type="category" dataKey="purpose" width={130} tick={{ fontSize: 12, fill: "#5f5750" }} tickLine={false} axisLine={false} />
                        <Tooltip
                          formatter={(value: ChartTooltipValue | undefined, name?: ChartTooltipName) => [`${tooltipValueToNumber(value).toFixed(1)}%`, providerLabel(tooltipNameToText(name))]}
                          contentStyle={{ borderRadius: 16, borderColor: "#d9d1c4", background: "#fff" }}
                        />
                        {providers.map((provider, index) => (
                          <Bar key={provider} dataKey={provider} stackId="purpose" fill={providerColor(provider)} radius={index === providers.length - 1 ? [999, 999, 999, 999] : 0}>
                            {providerMixRows.map((row) => (
                              <Cell key={`${provider}:${row.purpose}`} fill={providerColor(provider)} />
                            ))}
                          </Bar>
                        ))}
                      </BarChart>
                    </ResponsiveContainer>
                  </div>
                )}
                <div className="mt-3 flex flex-wrap gap-x-4 gap-y-2 text-xs text-[var(--color-editorial-ink-soft)]">
                  {providers.map((provider) => (
                    <span key={provider} className="inline-flex items-center gap-2">
                      <span className="size-2.5 rounded-full" style={{ backgroundColor: providerColor(provider) }} />
                      {providerLabel(provider)}
                    </span>
                  ))}
                </div>
              </section>
            </SectionCard>
          ) : null}

          {activeSection === "quality" ? (
            <SectionCard>
              <SectionTitle eyebrow={t("llmAnalysis.section.quality")} title={t("llmAnalysis.section.qualityTitle")} description={t("llmAnalysis.section.qualityDescription")} />
              <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                {rankingRows.length === 0 ? (
                  <p className="text-sm text-[var(--color-editorial-ink-faint)]">{t("llm.noSummary")}</p>
                ) : (
                  <div className="h-[24rem] w-full">
                    <ResponsiveContainer width="100%" height="100%">
                      <ScatterChart margin={{ top: 12, right: 20, left: 8, bottom: 12 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="#d9d1c4" />
                        <XAxis type="number" dataKey="calls" name="Calls" tick={{ fontSize: 12, fill: "#8f877f" }} tickLine={false} axisLine={false} />
                        <YAxis type="number" dataKey="failure_rate_pct" name="Failure Rate" tick={{ fontSize: 12, fill: "#8f877f" }} tickLine={false} axisLine={false} tickFormatter={(v) => `${Number(v).toFixed(1)}%`} />
                        <ZAxis type="number" dataKey="calls" range={[80, 420]} />
                        <Tooltip
                          formatter={(value: ChartTooltipValue | undefined, name?: ChartTooltipName) => {
                            if (name === "Failure Rate") return [`${tooltipValueToNumber(value).toFixed(1)}%`, tooltipNameToText(name)];
                            if (name === "Calls") return [fmtNum(tooltipValueToNumber(value)), tooltipNameToText(name)];
                            return [tooltipValueToText(value), tooltipNameToText(name)];
                          }}
                          labelFormatter={(_, payload) => {
                            const row = payload?.[0]?.payload as RankedModelRow | undefined;
                            if (!row) return "";
                            return `${providerLabel(row.provider)} / ${formatModelDisplayName(row.model)} (${row.purpose})`;
                          }}
                          content={({ active, payload }) => {
                            const row = payload?.[0]?.payload as RankedModelRow | undefined;
                            if (!active || !row) return null;
                            return (
                              <div className="rounded-[14px] border border-[var(--color-editorial-line)] bg-white px-3 py-2 shadow-[var(--shadow-dropdown)]">
                                <div className="text-xs font-semibold text-[var(--color-editorial-ink)]">{providerLabel(row.provider)} / {formatModelDisplayName(row.model)}</div>
                                <div className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{row.purpose}</div>
                                <div className="mt-2 grid gap-1 text-xs text-[var(--color-editorial-ink-soft)]">
                                  <div>{t("llm.totalCalls")}: {fmtNum(row.calls)}</div>
                                  <div>avg/call: {fmtUSD(row.avg_cost_per_call_usd)}</div>
                                  <div>tokens/call: {fmtNum(Math.round(row.total_tokens_per_call))}</div>
                                  <div>failure: {row.failure_rate_pct == null ? "—" : `${row.failure_rate_pct.toFixed(1)}%`}</div>
                                  <div>retry: {row.retry_rate_pct == null ? "—" : `${row.retry_rate_pct.toFixed(1)}%`}</div>
                                  <div>empty: {row.empty_rate_pct == null ? "—" : `${row.empty_rate_pct.toFixed(1)}%`}</div>
                                </div>
                              </div>
                            );
                          }}
                        />
                        <Scatter
                          data={rankingRows.filter((row) => row.failure_rate_pct != null)}
                          shape={(props: { cx?: number; cy?: number; payload?: RankedModelRow }) => {
                            const row = props.payload;
                            if (props.cx == null || props.cy == null || !row) return null;
                            return (
                              <circle
                                cx={props.cx}
                                cy={props.cy}
                                r={Math.max(6, Math.min(20, 6 + Math.sqrt(row.calls)))}
                                fill={providerColor(row.provider)}
                                fillOpacity={0.75}
                                stroke="#ffffff"
                                strokeWidth={2}
                              />
                            );
                          }}
                        />
                      </ScatterChart>
                    </ResponsiveContainer>
                  </div>
                )}
              </section>
            </SectionCard>
          ) : null}

          {activeSection === "recommend" ? (
            <SectionCard>
              <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
                <SectionTitle eyebrow={t("llmAnalysis.section.recommend")} title={t("llmAnalysis.rankingTitle")} description={t("llmAnalysis.rankingHelp")} compact />
                <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2.5 py-1 text-xs text-[var(--color-editorial-ink-faint)]">{t("llmAnalysis.rankingThreshold")}</span>
              </div>
              <div className="mb-4 flex flex-wrap gap-2">
                {purposes.map((purpose) => (
                  <button
                    key={purpose}
                    type="button"
                    onClick={() => setRankingPurpose(purpose)}
                    className={joinClassNames(
                      "rounded-full border px-3 py-1 text-xs font-medium transition",
                      rankingPurpose === purpose
                        ? "border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                        : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                    )}
                  >
                    {purpose}
                  </button>
                ))}
              </div>
              <div className="grid gap-3 lg:grid-cols-3">
                <InsightCard
                  title={t("llmAnalysis.ranking.bestCost")}
                  body={bestCostModel ? `${providerLabel(bestCostModel.provider)} / ${bestCostModel.model}` : "—"}
                  meta={bestCostModel ? `${fmtUSD(bestCostModel.avg_cost_per_call_usd)} avg/call · ${fmtNum(bestCostModel.calls)} calls` : "—"}
                />
                <InsightCard
                  title={t("llmAnalysis.ranking.bestTokens")}
                  body={bestTokenModel ? `${providerLabel(bestTokenModel.provider)} / ${bestTokenModel.model}` : "—"}
                  meta={bestTokenModel ? `${fmtNum(Math.round(bestTokenModel.total_tokens_per_call))} tokens/call · ${fmtNum(bestTokenModel.calls)} calls` : "—"}
                />
                <InsightCard
                  title={t("llmAnalysis.ranking.bestQuality")}
                  body={bestQualityModel ? `${providerLabel(bestQualityModel.provider)} / ${bestQualityModel.model}` : "—"}
                  meta={bestQualityModel ? `${(bestQualityModel.failure_rate_pct ?? 0).toFixed(1)}% fail · ${(bestQualityModel.retry_rate_pct ?? 0).toFixed(1)}% retry` : t("llmAnalysis.ranking.noQualityData")}
                />
              </div>
              <div className="mt-3 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-4">
                <div className="grid gap-3 lg:grid-cols-[minmax(0,320px)_1fr] lg:items-end">
                  <ModelSelect
                    label={t("llmAnalysis.diff.selectLabel")}
                    value={selectedModelKey ?? ""}
                    onChange={(value) => setSelectedModelKey(value || null)}
                    options={rankingModelOptions}
                    labels={{
                      defaultOption: t("llmAnalysis.diff.selectPlaceholder"),
                      searchPlaceholder: t("settings.modelSelect.searchPlaceholder"),
                      noResults: t("settings.modelSelect.noResults"),
                      providerAll: t("settings.modelSelect.providerAll"),
                      modalChoose: t("settings.modelSelect.modalChoose"),
                      close: t("common.close"),
                      confirmTitle: t("settings.modelSelect.confirmTitle"),
                      confirmYes: t("settings.modelSelect.confirmYes"),
                      confirmNo: t("settings.modelSelect.confirmNo"),
                      confirmSuffix: t("settings.modelSelect.confirmSuffix"),
                      providerColumn: t("settings.modelSelect.providerColumn"),
                      modelColumn: t("settings.modelSelect.modelColumn"),
                      pricingColumn: t("settings.modelSelect.pricingColumn"),
                    }}
                  />
                  <p className="text-xs text-[var(--color-editorial-ink-soft)] lg:pb-2">{t("llmAnalysis.diff.selectHelp")}</p>
                </div>
              </div>
              <div className="mt-3 grid gap-3 lg:grid-cols-2">
                <InsightCard
                  title={`${rankingPurposeLabel} ${t("llmAnalysis.diff.spread")}`}
                  body={bestCostModel && rankingMedianCost != null ? `${fmtUSD(bestCostModel.avg_cost_per_call_usd)} -> ${fmtUSD(rankingMedianCost)}` : "—"}
                  meta={bestCostModel && rankingMedianCost != null && bestCostModel.avg_cost_per_call_usd > 0 ? `${t("llmAnalysis.diff.betweenCheapestAndMedian")} · ${(((rankingMedianCost - bestCostModel.avg_cost_per_call_usd) / bestCostModel.avg_cost_per_call_usd) * 100).toFixed(0)}% ${t("llmAnalysis.diff.moreThanCheapest")}` : "—"}
                />
                <InsightCard
                  title={`${rankingPurposeLabel} ${t("llmAnalysis.diff.selected")}`}
                  body={selectedRankingRow ? `${providerLabel(selectedRankingRow.provider)} / ${selectedRankingRow.model}` : t("llmAnalysis.diff.selectPlaceholder")}
                  meta={selectedRankingRow ? `${selectedRankingRow.vs_cheapest_pct >= 0 ? "+" : ""}${selectedRankingRow.vs_cheapest_pct.toFixed(0)}% ${t("llmAnalysis.diff.vsCheapestInPurpose")} · ${selectedRankingRow.vs_median_pct >= 0 ? "+" : ""}${selectedRankingRow.vs_median_pct.toFixed(0)}% ${t("llmAnalysis.diff.vsMedianInPurpose")}` : t("llmAnalysis.diff.selectedHelp")}
                />
              </div>
              <div className="mt-4 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
                  <div>
                    <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">{t("llmAnalysis.section.qa")}</div>
                    <h3 className="mt-2 font-serif text-[1.25rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("llmAnalysis.qa.title")}</h3>
                    <p className="mt-2 text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("llmAnalysis.qa.description")}</p>
                  </div>
                  <div className="flex flex-wrap items-center justify-end gap-2">
                    {selectedRankingRow ? (
                      <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-xs text-[var(--color-editorial-ink-faint)]">
                        {providerLabel(selectedRankingRow.provider)} / {selectedRankingRow.model}
                      </span>
                    ) : null}
                    {selectedModelKey ? (
                      <button
                        type="button"
                        onClick={() => setSelectedModelKey(null)}
                        className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1.5 text-xs font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                      >
                        {t("llmAnalysis.diff.clearSelection")}
                      </button>
                    ) : null}
                  </div>
                </div>
                {qaLoading ? (
                  <p className="text-sm text-[var(--color-editorial-ink-soft)]">{t("common.loading")}</p>
                ) : !selectedRankingRow ? (
                  <p className="text-sm text-[var(--color-editorial-ink-soft)]">{t("llmAnalysis.qa.emptySelection")}</p>
                ) : qaSamples.length === 0 ? (
                  <p className="text-sm text-[var(--color-editorial-ink-soft)]">{t("llmAnalysis.qa.noSamples")}</p>
                ) : (
                  <div className="grid gap-3 lg:grid-cols-3">
                    {qaSamples.map((item) => (
                      <article key={item.id} className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4 shadow-[var(--shadow-card)]">
                        <div className="text-xs text-[var(--color-editorial-ink-faint)]">
                          {item.summary_llm ? `${providerLabel(item.summary_llm.provider)} / ${item.summary_llm.model}` : providerLabel(selectedRankingRow.provider)}
                        </div>
                        <h4 className="mt-2 line-clamp-2 font-serif text-[1.05rem] leading-[1.35] text-[var(--color-editorial-ink)]">{item.translated_title || item.title || t("llmAnalysis.qa.untitled")}</h4>
                        <div className="mt-3 flex flex-wrap gap-1 text-[11px] text-[var(--color-editorial-ink-soft)]">
                          {item.summary ? <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2 py-0.5">summary</span> : null}
                          {item.facts?.facts?.length ? <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2 py-0.5">facts {item.facts.facts.length}</span> : null}
                          {item.facts_check?.final_result ? <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2 py-0.5">facts check {item.facts_check.final_result}</span> : null}
                          {item.faithfulness?.final_result ? <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2 py-0.5">faithfulness {item.faithfulness.final_result}</span> : null}
                        </div>
                        {selectedRankingRow.purpose === "facts" || selectedRankingRow.purpose === "facts_check" ? (
                          <ul className="mt-3 space-y-1 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                            {(item.facts?.facts ?? []).slice(0, 4).map((fact, idx) => (
                              <li key={idx} className="line-clamp-2">• {fact}</li>
                            ))}
                          </ul>
                        ) : (
                          <p className="mt-3 line-clamp-6 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">{item.summary?.summary || t("llmAnalysis.qa.noSummary")}</p>
                        )}
                        <Link href={`/items/${item.id}`} className="mt-3 inline-flex text-xs font-medium text-[var(--color-editorial-ink-soft)] hover:text-[var(--color-editorial-ink)]">
                          {t("llmAnalysis.qa.openItem")}
                        </Link>
                      </article>
                    ))}
                  </div>
                )}
              </div>
              <div className="mt-4 overflow-x-auto rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)]">
                <table className="min-w-full border-separate border-spacing-0 text-sm">
                  <thead className="bg-[var(--color-editorial-panel)] text-[11px] text-[var(--color-editorial-ink-faint)]">
                    <tr>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-left font-medium uppercase tracking-[0.14em]">{t("llmAnalysis.provider")}</th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-left font-medium uppercase tracking-[0.14em]">{t("llmAnalysis.purpose")}</th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-left font-medium uppercase tracking-[0.14em]">{t("llmAnalysis.model")}</th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]">calls</th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]">cost</th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]">avg/call</th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]">tokens/call</th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]">{t("llmAnalysis.quality.failureRate")}</th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]">{t("llmAnalysis.quality.retryRate")}</th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]">{t("llmAnalysis.quality.emptyRate")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {rankingRows.length === 0 ? (
                      <tr>
                        <td colSpan={10} className="px-4 py-8 text-center text-sm text-[var(--color-editorial-ink-faint)]">
                          {t("llm.noSummary")}
                        </td>
                      </tr>
                    ) : (
                      rankingRows.slice(0, 18).map((row) => (
                        <tr
                          key={`${row.provider}:${row.model}:${row.purpose}`}
                          className="cursor-pointer transition hover:bg-[var(--color-editorial-panel)]"
                          onClick={() => applyRowFilter(row)}
                        >
                          <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-[var(--color-editorial-ink)]">{providerLabel(row.provider)}</td>
                          <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-[var(--color-editorial-ink-soft)]">{row.purpose}</td>
                          <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 whitespace-nowrap text-xs text-[var(--color-editorial-ink)]">{formatModelDisplayName(row.model)}</td>
                          <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink)]">{fmtNum(row.calls)}</td>
                          <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink)]">{fmtUSD(row.estimated_cost_usd)}</td>
                          <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink-soft)]">{fmtUSD(row.avg_cost_per_call_usd)}</td>
                          <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink-soft)]">{fmtNum(Math.round(row.total_tokens_per_call))}</td>
                          <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink-soft)]">{row.failure_rate_pct == null ? "—" : `${row.failure_rate_pct.toFixed(1)}%`}</td>
                          <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink-soft)]">{row.retry_rate_pct == null ? "—" : `${row.retry_rate_pct.toFixed(1)}%`}</td>
                          <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink-soft)]">{row.empty_rate_pct == null ? "—" : `${row.empty_rate_pct.toFixed(1)}%`}</td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            </SectionCard>
          ) : null}

          {activeSection === "details" ? (
            <SectionCard>
              <SectionTitle eyebrow={t("llmAnalysis.section.details")} title={t("llmAnalysis.section.detailsTitle")} description={t("llmAnalysis.section.detailsDescription")} />
              <div className="mb-3 flex items-center justify-between">
                <h2 className="font-serif text-[1.45rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("llmAnalysis.matrixTitle")}</h2>
                <span className="text-xs text-[var(--color-editorial-ink-faint)]">{sortedRows.length} rows</span>
              </div>
              <div className="overflow-x-auto rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)]">
                <table className="min-w-full border-separate border-spacing-0 text-sm">
                  <thead className="bg-[var(--color-editorial-panel)] text-[11px] text-[var(--color-editorial-ink-faint)]">
                    <tr>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-left font-medium uppercase tracking-[0.14em]"><button type="button" onClick={() => toggleSort("provider")} className="inline-flex items-center hover:text-[var(--color-editorial-ink)]">{t("llmAnalysis.provider")}{sortMark("provider")}</button></th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-left font-medium uppercase tracking-[0.14em]"><button type="button" onClick={() => toggleSort("model")} className="inline-flex items-center hover:text-[var(--color-editorial-ink)]">{t("llmAnalysis.model")}{sortMark("model")}</button></th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-left font-medium uppercase tracking-[0.14em]"><button type="button" onClick={() => toggleSort("purpose")} className="inline-flex items-center hover:text-[var(--color-editorial-ink)]">{t("llmAnalysis.purpose")}{sortMark("purpose")}</button></th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-left font-medium uppercase tracking-[0.14em]"><button type="button" onClick={() => toggleSort("pricing_source")} className="inline-flex items-center hover:text-[var(--color-editorial-ink)]">{t("llmAnalysis.pricing")}{sortMark("pricing_source")}</button></th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]"><button type="button" onClick={() => toggleSort("calls")} className="inline-flex items-center hover:text-[var(--color-editorial-ink)]">calls{sortMark("calls")}</button></th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]"><button type="button" onClick={() => toggleSort("avg_input_tokens_per_call")} className="inline-flex items-center hover:text-[var(--color-editorial-ink)]">avg in/call{sortMark("avg_input_tokens_per_call")}</button></th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]"><button type="button" onClick={() => toggleSort("avg_output_tokens_per_call")} className="inline-flex items-center hover:text-[var(--color-editorial-ink)]">avg out/call{sortMark("avg_output_tokens_per_call")}</button></th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]"><button type="button" onClick={() => toggleSort("avg_cost_per_call_usd")} className="inline-flex items-center hover:text-[var(--color-editorial-ink)]">avg/call{sortMark("avg_cost_per_call_usd")}</button></th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]"><button type="button" onClick={() => toggleSort("cost_share_pct")} className="inline-flex items-center hover:text-[var(--color-editorial-ink)]">share{sortMark("cost_share_pct")}</button></th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]"><button type="button" onClick={() => toggleSort("estimated_cost_usd")} className="inline-flex items-center hover:text-[var(--color-editorial-ink)]">cost{sortMark("estimated_cost_usd")}</button></th>
                    </tr>
                  </thead>
                  <tbody>
                    {sortedRows.map((row) => (
                      <tr key={`${row.provider}:${row.model}:${row.purpose}:${row.pricing_source}`} className="transition hover:bg-[var(--color-editorial-panel)]">
                        <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-[var(--color-editorial-ink)]">{providerLabel(row.provider)}</td>
                        <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 whitespace-nowrap text-xs text-[var(--color-editorial-ink)]">{formatModelDisplayName(row.model)}</td>
                        <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-[var(--color-editorial-ink-soft)]">{row.purpose}</td>
                        <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-xs text-[var(--color-editorial-ink-faint)]">{row.pricing_source}</td>
                        <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink)]">{fmtNum(row.calls)}</td>
                        <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink-soft)]">{fmtNum(Math.round(row.avg_input_tokens_per_call))}</td>
                        <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink-soft)]">{fmtNum(Math.round(row.avg_output_tokens_per_call))}</td>
                        <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink-soft)]">{fmtUSD(row.avg_cost_per_call_usd)}</td>
                        <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink-soft)]">{row.cost_share_pct.toFixed(1)}%</td>
                        <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink)]">{fmtUSD(row.estimated_cost_usd)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </SectionCard>
          ) : null}
        </div>
      </div>
    </div>
  );
}

function SectionTitle({
  eyebrow,
  title,
  description,
  compact = false,
}: {
  eyebrow: string;
  title: string;
  description: string;
  compact?: boolean;
}) {
  return (
    <div className={compact ? "" : "mb-4"}>
      <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">{eyebrow}</div>
      <h2 className="mt-2 font-serif text-[1.45rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">{title}</h2>
      <p className="mt-2 text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">{description}</p>
    </div>
  );
}

function InsightCard({ title, body, meta, onClick }: { title: string; body: string; meta: string; onClick?: () => void }) {
  const Comp = onClick ? "button" : "div";
  return (
    <Comp
      {...(onClick ? { type: "button", onClick } : {})}
      className={joinClassNames(
        "rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-4 text-left shadow-[var(--shadow-card)]",
        onClick ? "transition hover:bg-[var(--color-editorial-panel)]" : ""
      )}
    >
      <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">{title}</div>
      <div className="mt-2 text-sm font-semibold leading-6 text-[var(--color-editorial-ink)]">{body}</div>
      <div className="mt-2 text-xs text-[var(--color-editorial-ink-soft)]">{meta}</div>
    </Comp>
  );
}
