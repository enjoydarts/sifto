import Link from "next/link";
import { type LucideIcon } from "lucide-react";

export function EmptyState({
  icon: Icon,
  title,
  description,
  action,
}: {
  icon: LucideIcon;
  title: string;
  description: string;
  action?: { label: string; href?: string; onClick?: () => void };
}) {
  return (
    <div className="flex flex-col items-center justify-center px-4 py-16 text-center motion-safe:animate-fade-in-up">
      <div className="rounded-2xl bg-zinc-100 p-4">
        <Icon className="size-12 text-zinc-400" aria-hidden="true" />
      </div>
      <h3 className="mt-4 text-base font-semibold text-zinc-800">{title}</h3>
      <p className="mt-1 max-w-sm text-sm text-zinc-500">{description}</p>
      {action?.href && (
        <Link
          href={action.href}
          className="mt-5 inline-flex items-center rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-700 press focus-ring"
        >
          {action.label}
        </Link>
      )}
      {!action?.href && action?.onClick && (
        <button
          type="button"
          onClick={action.onClick}
          className="mt-5 inline-flex items-center rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-700 press focus-ring"
        >
          {action.label}
        </button>
      )}
    </div>
  );
}
