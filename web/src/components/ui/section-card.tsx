import type { ReactNode } from "react";

type SectionCardProps = {
  children: ReactNode;
  className?: string;
  compact?: boolean;
};

function joinClassNames(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

export function SectionCard({ children, className = "", compact = false }: SectionCardProps) {
  return (
    <section
      className={joinClassNames(
        "surface-editorial rounded-[var(--radius-panel)]",
        compact ? "p-3 sm:p-4" : "p-4 sm:p-5",
        className
      )}
    >
      {children}
    </section>
  );
}
