import type { ReactNode } from "react";

type ActionRowProps = {
  children: ReactNode;
  align?: "start" | "center" | "end" | "between";
  compact?: boolean;
  wrap?: boolean;
  className?: string;
};

function joinClassNames(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

export function ActionRow({
  children,
  align = "end",
  compact = false,
  wrap = true,
  className = "",
}: ActionRowProps) {
  return (
    <div
      className={joinClassNames(
        "flex items-center gap-2",
        wrap ? "flex-wrap" : "flex-nowrap",
        align === "start" ? "justify-start" : align === "center" ? "justify-center" : align === "between" ? "justify-between" : "justify-end",
        compact ? "gap-1.5" : "",
        className
      )}
    >
      {children}
    </div>
  );
}
