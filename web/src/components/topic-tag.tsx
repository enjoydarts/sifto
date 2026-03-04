import Link from "next/link";

function hashString(str: string): number {
  let h = 0;
  for (let i = 0; i < str.length; i++) {
    h = (Math.imul(31, h) + str.charCodeAt(i)) | 0;
  }
  return Math.abs(h);
}

const DOT_COLORS = [
  "bg-blue-400",
  "bg-emerald-400",
  "bg-rose-400",
  "bg-amber-400",
  "bg-violet-400",
  "bg-cyan-400",
  "bg-lime-400",
  "bg-fuchsia-400",
];

function getDotColor(topic: string): string {
  return DOT_COLORS[hashString(topic) % DOT_COLORS.length] ?? DOT_COLORS[0]!;
}

export function TopicTag({ topic }: { topic: string }) {
  const dotColor = getDotColor(topic);
  const href = `/items?feed=all&sort=score&topic=${encodeURIComponent(topic)}`;

  return (
    <Link
      href={href}
      className="inline-flex items-center gap-1.5 rounded-full border border-zinc-200 bg-white px-2.5 py-1 text-xs font-medium text-zinc-700 shadow-sm hover:shadow-md hover:border-zinc-300 transition-all press focus-ring"
    >
      <span className={`size-2 rounded-full shrink-0 ${dotColor}`} />
      {topic}
    </Link>
  );
}
