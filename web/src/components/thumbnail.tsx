"use client";

import { useState } from "react";

type ThumbnailSize = "sm" | "md" | "lg";
type ThumbnailTone = "default" | "editorial";

const SIZE_MAP: Record<ThumbnailSize, { container: string; text: string }> = {
  sm: { container: "h-12 w-12",   text: "text-base" },
  md: { container: "h-[72px] w-[72px]", text: "text-xl" },
  lg: { container: "h-full w-full", text: "text-3xl" },
};

function hashString(str: string): number {
  let h = 0;
  for (let i = 0; i < str.length; i++) {
    h = (Math.imul(31, h) + str.charCodeAt(i)) | 0;
  }
  return Math.abs(h);
}

const GRADIENTS = [
  "from-blue-400 to-indigo-500",
  "from-emerald-400 to-teal-500",
  "from-rose-400 to-pink-500",
  "from-amber-400 to-orange-500",
  "from-violet-400 to-purple-500",
  "from-cyan-400 to-sky-500",
  "from-lime-400 to-green-500",
  "from-fuchsia-400 to-rose-500",
];

function getGradient(title: string): string {
  return GRADIENTS[hashString(title) % GRADIENTS.length] ?? GRADIENTS[0]!;
}

function getInitial(title: string): string {
  return (title.trim()[0] ?? "?").toUpperCase();
}

export function Thumbnail({
  src,
  title,
  size = "md",
  tone = "default",
  className = "",
}: {
  src?: string | null;
  title: string;
  size?: ThumbnailSize;
  tone?: ThumbnailTone;
  className?: string;
}) {
  const [imgError, setImgError] = useState(false);
  const { container, text } = SIZE_MAP[size];
  const showFallback = !src || imgError;
  const gradient = getGradient(title);
  const initial = getInitial(title);

  return (
    <div className={`relative shrink-0 overflow-hidden rounded-lg ${container} ${className}`}>
      {showFallback ? (
        <div
          className={
            tone === "editorial"
              ? "flex h-full w-full items-center justify-center bg-[linear-gradient(135deg,rgba(157,60,36,0.18),transparent),linear-gradient(135deg,#d6d0c5,#f7f4ee)]"
              : `flex h-full w-full items-center justify-center bg-gradient-to-br ${gradient}`
          }
        >
          <span className={`${tone === "editorial" ? "text-[rgba(23,20,18,0.76)]" : "text-white/90"} font-bold ${text}`}>{initial}</span>
        </div>
      ) : (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={src}
          alt=""
          loading="lazy"
          referrerPolicy="no-referrer"
          className="h-full w-full object-cover"
          onError={() => setImgError(true)}
        />
      )}
      {tone === "editorial" && (
        <span
          aria-hidden="true"
          className="pointer-events-none absolute inset-0 bg-[linear-gradient(180deg,transparent,rgba(23,20,18,0.08)),repeating-linear-gradient(90deg,transparent,transparent_24px,rgba(255,255,255,0.16)_24px,rgba(255,255,255,0.16)_25px)]"
        />
      )}
    </div>
  );
}
