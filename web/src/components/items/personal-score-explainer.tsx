"use client";

import type { ReactNode } from "react";
import { PersonalScoreBreakdown } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { Tag } from "@/components/ui/tag";

type PersonalScoreExplainerProps = {
  score?: number | null;
  reason?: string | null;
  breakdown?: PersonalScoreBreakdown | null;
};

function toneForScore(score: number): "success" | "accent" | "warning" | "info" {
  if (score >= 0.75) return "success";
  if (score >= 0.55) return "accent";
  if (score >= 0.4) return "warning";
  return "info";
}

function reasonLabel(reason: string | null | undefined, t: (key: string, fallback?: string) => string) {
  const value = (reason ?? "").trim();
  if (!value) return null;
  if (value.startsWith("topic:")) {
    return t("itemDetail.personal.reason.topic").replace("{{topic}}", value.slice("topic:".length));
  }
  if (value.startsWith("weight:")) {
    return t("itemDetail.personal.reason.weight").replace("{{dimension}}", t(`itemDetail.score.${value.slice("weight:".length)}`, value.slice("weight:".length)));
  }
  if (value === "embedding_similarity") return t("itemDetail.personal.reason.embedding");
  if (value === "source_affinity") return t("itemDetail.personal.reason.source");
  return t("itemDetail.personal.reason.attention");
}

export function PersonalScoreExplainer({ score, reason, breakdown }: PersonalScoreExplainerProps) {
  const { t } = useI18n();
  if (score == null || !breakdown) return null;

  const rows = [
    ["learned_weight_score", t("itemDetail.personal.breakdown.learnedWeight"), breakdown.learned_weight_score],
    ["topic_relevance", t("itemDetail.personal.breakdown.topic"), breakdown.topic_relevance],
    ["embedding_similarity", t("itemDetail.personal.breakdown.embedding"), breakdown.embedding_similarity],
    ["source_affinity", t("itemDetail.personal.breakdown.source"), breakdown.source_affinity],
  ] as const;

  return (
    <DetailLikeBox title={t("itemDetail.personal.title")}>
      <div className="flex flex-wrap items-center gap-2">
        <Tag tone={toneForScore(score)}>{t("itemDetail.personal.score").replace("{{value}}", score.toFixed(2))}</Tag>
        {reasonLabel(reason, t) ? <Tag>{reasonLabel(reason, t)}</Tag> : null}
        {breakdown.dominant_dimension ? (
          <Tag tone="accent">
            {t("itemDetail.personal.dominant").replace("{{dimension}}", t(`itemDetail.score.${breakdown.dominant_dimension}`, breakdown.dominant_dimension))}
          </Tag>
        ) : null}
      </div>

      <p className="mt-3 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
        {t("itemDetail.personal.description")}
      </p>

      <div className="mt-4 grid gap-3">
        {rows.map(([key, label, component]) => (
          <div key={key} className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-3">
            <div className="flex items-center justify-between gap-3 text-xs text-[var(--color-editorial-ink-faint)]">
              <span>{label}</span>
              <span className="tabular-nums">{component.weight.toFixed(2)}</span>
            </div>
            <div className="mt-2 flex items-center gap-3">
              <div className="h-2 flex-1 rounded-full bg-[#e9e1d3]">
                <div className="h-2 rounded-full bg-[var(--color-editorial-ink)]" style={{ width: `${Math.max(component.value * 100, 4)}%` }} />
              </div>
              <span className="w-10 text-right text-xs font-medium tabular-nums text-[var(--color-editorial-ink-soft)]">
                {component.value.toFixed(2)}
              </span>
            </div>
          </div>
        ))}
      </div>

      {breakdown.matched_topics && breakdown.matched_topics.length > 0 ? (
        <div className="mt-4 flex flex-wrap gap-2">
          {breakdown.matched_topics.map((topic) => (
            <Tag key={topic} tone="accent">{topic}</Tag>
          ))}
        </div>
      ) : null}
    </DetailLikeBox>
  );
}

function DetailLikeBox({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="mt-5 rounded-[18px] border border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,rgba(245,240,233,0.78),rgba(255,255,255,0.9))] px-4 py-4">
      <h3 className="mb-2 text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
        {title}
      </h3>
      {children}
    </div>
  );
}
