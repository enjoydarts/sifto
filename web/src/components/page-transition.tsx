import { type ReactNode } from "react";

export function PageTransition({ children }: { children: ReactNode }) {
  return (
    <div className="motion-safe:animate-fade-in-up">
      {children}
    </div>
  );
}
