"use client";

import { useState } from "react";

type ThumbnailSize = "sm" | "md" | "lg";

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
  className = "",
}: {
  src?: string | null;
  title: string;
  size?: ThumbnailSize;
  className?: string;
}) {
  const [imgError, setImgError] = useState(false);
  const { container, text } = SIZE_MAP[size];
  const showFallback = !src || imgError;
  const gradient = getGradient(title);
  const initial = getInitial(title);

  return (
    <div className={`shrink-0 overflow-hidden rounded-lg ${container} ${className}`}>
      {showFallback ? (
        <div
          className={`flex h-full w-full items-center justify-center bg-gradient-to-br ${gradient}`}
        >
          <span className={`font-bold text-white/90 ${text}`}>{initial}</span>
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
    </div>
  );
}
