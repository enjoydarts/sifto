"use client";

import { SummaryMetricCard } from "@/components/ui/summary-metric-card";
import { SummaryStrip } from "@/components/ui/summary-strip";

type SummaryMetric = {
  key: string;
  label: string;
  value: string;
  hint: string;
  tone?: "default" | "muted" | "accent";
};

export function ItemsSummaryStrip({ metrics }: { metrics: SummaryMetric[] }) {
  if (metrics.length === 0) return null;

  return (
    <SummaryStrip compact>
      <div className="grid grid-cols-2 gap-2 md:grid-cols-2 md:gap-3 xl:grid-cols-4">
        {metrics.map((metric) => (
          <SummaryMetricCard
            key={metric.key}
            label={metric.label}
            value={metric.value}
            hint={metric.hint}
            tone={metric.tone}
            compact
            className="min-w-0"
          />
        ))}
      </div>
    </SummaryStrip>
  );
}
