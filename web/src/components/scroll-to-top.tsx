"use client";

import { useEffect, useState } from "react";
import { ArrowUp } from "lucide-react";

export function ScrollToTop() {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const onScroll = () => {
      setVisible(window.scrollY > 400);
    };
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  if (!visible) return null;

  return (
    <button
      type="button"
      onClick={() => window.scrollTo({ top: 0, behavior: "smooth" })}
      aria-label="トップへ戻る"
      className="fixed bottom-20 right-4 z-40 flex h-10 w-10 items-center justify-center rounded-full border border-zinc-200 bg-white shadow-md hover:shadow-lg hover:bg-zinc-50 transition-all press focus-ring md:bottom-6 motion-safe:animate-fade-in"
    >
      <ArrowUp className="size-4 text-zinc-600" aria-hidden="true" />
    </button>
  );
}
