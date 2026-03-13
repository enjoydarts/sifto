type ScoreIndicatorSize = "sm" | "md" | "lg";

const SIZE_MAP: Record<ScoreIndicatorSize, { px: number; stroke: number; text: string }> = {
  sm: { px: 28, stroke: 3,   text: "text-[8px]"  },
  md: { px: 36, stroke: 3.5, text: "text-[9px]"  },
  lg: { px: 48, stroke: 4,   text: "text-[11px]" },
};

function scoreColor(score: number) {
  if (score >= 0.8)  return { ring: "#22c55e", text: "text-green-700" };
  if (score >= 0.65) return { ring: "#3b82f6", text: "text-blue-700"  };
  if (score >= 0.5)  return { ring: "#71717a", text: "text-zinc-700"  };
  return              { ring: "#f59e0b", text: "text-amber-700" };
}

function formatPersonalScoreReason(reason: string, locale: "ja" | "en"): string {
  if (reason === "embedding_similarity") {
    return locale === "ja" ? "お気に入りに近い傾向" : "Similar to favorites";
  }
  if (reason.startsWith("topic:")) {
    const topic = reason.slice("topic:".length);
    return locale === "ja" ? `関心の高いトピック: ${topic}` : `Topic of interest: ${topic}`;
  }
  if (reason === "source_affinity") {
    return locale === "ja" ? "よく読むソース" : "Frequently read source";
  }
  if (reason.startsWith("weight:")) {
    const weight = reason.slice("weight:".length);
    return locale === "ja" ? `${weight}が高い` : `High ${weight}`;
  }
  if (reason === "attention") {
    return locale === "ja" ? "注目記事" : "Trending";
  }
  return reason;
}

export function ScoreIndicator({
  score,
  personalScore,
  personalScoreReason,
  locale = "ja",
  size = "sm",
}: {
  score: number | null | undefined;
  personalScore?: number | null;
  personalScoreReason?: string | null;
  locale?: "ja" | "en";
  size?: ScoreIndicatorSize;
}) {
  const displayScore = personalScore != null ? personalScore : score;
  const { px, stroke, text } = SIZE_MAP[size];
  const radius = (px - stroke) / 2;
  const circumference = 2 * Math.PI * radius;
  const pct = displayScore != null ? Math.max(0, Math.min(1, displayScore)) : 0;
  const dashOffset = circumference * (1 - pct);
  const color = displayScore != null ? scoreColor(displayScore) : { ring: "#d4d4d8", text: "text-zinc-400" };
  const center = px / 2;

  const isPersonal = personalScore != null;
  const ringColor = isPersonal ? "#8b5cf6" : color.ring;
  const textColor = isPersonal ? "text-violet-700" : color.text;

  let titleText: string;
  if (isPersonal) {
    const parts: string[] = [`Personal: ${personalScore.toFixed(2)}`];
    if (personalScoreReason) {
      const reasons = personalScoreReason.split(",").map((r) => formatPersonalScoreReason(r.trim(), locale));
      parts.push(...reasons);
    }
    if (score != null) {
      parts.push(`(Base: ${score.toFixed(2)})`);
    }
    titleText = parts.join(" / ");
  } else {
    titleText = displayScore != null ? `Score: ${displayScore.toFixed(2)}` : "No score";
  }

  return (
    <div
      className="relative shrink-0 inline-flex items-center justify-center"
      style={{ width: px, height: px }}
      title={titleText}
    >
      <svg width={px} height={px} className="-rotate-90">
        <circle
          cx={center}
          cy={center}
          r={radius}
          fill="none"
          stroke="#e4e4e7"
          strokeWidth={stroke}
        />
        <circle
          cx={center}
          cy={center}
          r={radius}
          fill="none"
          stroke={ringColor}
          strokeWidth={stroke}
          strokeDasharray={circumference}
          strokeDashoffset={dashOffset}
          strokeLinecap="round"
        />
      </svg>
      <span className={`absolute font-semibold tabular-nums ${text} ${textColor}`}>
        {displayScore != null ? displayScore.toFixed(2) : "—"}
      </span>
    </div>
  );
}
