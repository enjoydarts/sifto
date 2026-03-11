"use client";

import { Brain, X } from "lucide-react";
import type { ProviderModelChangeEvent } from "@/lib/api";

export default function ProviderModelUpdatesPanel({
  allEvents,
  visibleEvents,
  onDismiss,
  onRestore,
  labels,
}: {
  allEvents: ProviderModelChangeEvent[];
  visibleEvents: ProviderModelChangeEvent[];
  onDismiss: () => void;
  onRestore: () => void;
  labels: {
    title: string;
    description: string;
    dismiss: string;
    empty: string;
    dismissed: string;
    restore: string;
    added: string;
    removed: string;
  };
}) {
  return (
    <section className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
      <div className="mb-4 flex items-start justify-between gap-3">
        <div>
          <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
            <Brain className="size-4 text-zinc-500" aria-hidden="true" />
            {labels.title}
          </h2>
          <p className="mt-1 text-sm text-zinc-500">{labels.description}</p>
        </div>
        {allEvents.length > 0 && visibleEvents.length > 0 && (
          <button
            type="button"
            onClick={onDismiss}
            className="inline-flex items-center rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50"
          >
            <X className="mr-1 size-4" aria-hidden="true" />
            {labels.dismiss}
          </button>
        )}
      </div>
      {allEvents.length === 0 ? (
        <p className="text-sm text-zinc-500">{labels.empty}</p>
      ) : visibleEvents.length === 0 ? (
        <div className="flex flex-wrap items-center gap-3">
          <p className="text-sm text-zinc-500">{labels.dismissed}</p>
          <button
            type="button"
            onClick={onRestore}
            className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50"
          >
            {labels.restore}
          </button>
        </div>
      ) : (
        <div className="space-y-2">
          {visibleEvents.map((event) => (
            <div key={event.id} className="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm">
              <div className="flex flex-wrap items-center gap-2">
                <span className="rounded bg-white px-2 py-0.5 text-xs font-medium text-zinc-700 ring-1 ring-zinc-200">
                  {event.provider}
                </span>
                <span
                  className={`rounded px-2 py-0.5 text-xs font-medium ${
                    event.change_type === "added" ? "bg-emerald-50 text-emerald-700" : "bg-red-50 text-red-700"
                  }`}
                >
                  {event.change_type === "added" ? labels.added : labels.removed}
                </span>
                <span className="break-all text-zinc-800">{event.model_id}</span>
                <span className="ml-auto text-xs text-zinc-400">{new Date(event.detected_at).toLocaleString()}</span>
              </div>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
