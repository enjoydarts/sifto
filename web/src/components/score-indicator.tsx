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

export function ScoreIndicator({
  score,
  size = "sm",
}: {
  score: number | null | undefined;
  size?: ScoreIndicatorSize;
}) {
  const { px, stroke, text } = SIZE_MAP[size];
  const radius = (px - stroke) / 2;
  const circumference = 2 * Math.PI * radius;
  const pct = score != null ? Math.max(0, Math.min(1, score)) : 0;
  const dashOffset = circumference * (1 - pct);
  const color = score != null ? scoreColor(score) : { ring: "#d4d4d8", text: "text-zinc-400" };
  const center = px / 2;

  return (
    <div
      className="relative shrink-0 inline-flex items-center justify-center"
      style={{ width: px, height: px }}
      title={score != null ? `Score: ${score.toFixed(2)}` : "No score"}
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
          stroke={color.ring}
          strokeWidth={stroke}
          strokeDasharray={circumference}
          strokeDashoffset={dashOffset}
          strokeLinecap="round"
        />
      </svg>
      <span className={`absolute font-semibold tabular-nums ${text} ${color.text}`}>
        {score != null ? score.toFixed(2) : "—"}
      </span>
    </div>
  );
}
