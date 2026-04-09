"use client";

import { Brain, X } from "lucide-react";
import type { ProviderModelChangeEvent } from "@/lib/api";

function providerLabel(provider: string): string {
  switch (provider) {
    case "anthropic":
      return "Anthropic";
    case "google":
      return "Google";
    case "groq":
      return "Groq";
    case "deepseek":
      return "DeepSeek";
    case "alibaba":
      return "Alibaba";
    case "mistral":
      return "Mistral";
    case "moonshot":
      return "Moonshot";
    case "xai":
      return "xAI";
    case "zai":
      return "Z.ai";
    case "fireworks":
      return "Fireworks";
    case "together":
      return "Together AI";
    case "openrouter":
      return "OpenRouter";
    case "poe":
      return "Poe";
    case "siliconflow":
      return "SiliconFlow";
    case "openai":
      return "OpenAI";
    default:
      return provider;
  }
}

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
    <section className="surface-editorial rounded-[var(--radius-panel)] p-5">
      <div className="mb-4 flex items-start justify-between gap-3">
        <div>
          <h2 className="inline-flex items-center gap-2 text-base font-semibold text-[var(--color-editorial-ink)]">
            <Brain className="size-4 text-[var(--color-editorial-ink-faint)]" aria-hidden="true" />
            {labels.title}
          </h2>
          <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{labels.description}</p>
        </div>
        {allEvents.length > 0 && visibleEvents.length > 0 && (
          <button
            type="button"
            onClick={onDismiss}
            className="inline-flex items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
          >
            <X className="mr-1 size-4" aria-hidden="true" />
            {labels.dismiss}
          </button>
        )}
      </div>
      {allEvents.length === 0 ? (
        <p className="text-sm text-[var(--color-editorial-ink-soft)]">{labels.empty}</p>
      ) : visibleEvents.length === 0 ? (
        <div className="flex flex-wrap items-center gap-3">
          <p className="text-sm text-[var(--color-editorial-ink-soft)]">{labels.dismissed}</p>
          <button
            type="button"
            onClick={onRestore}
            className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
          >
            {labels.restore}
          </button>
        </div>
      ) : (
        <div className="space-y-2">
          {visibleEvents.map((event) => (
            <div key={event.id} className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2 text-sm">
              <div className="flex flex-wrap items-center gap-2">
                <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2 py-0.5 text-xs font-medium text-[var(--color-editorial-ink-soft)]">
                  {providerLabel(event.provider)}
                </span>
                <span
                  className={`rounded px-2 py-0.5 text-xs font-medium ${
                    event.change_type === "added"
                      ? "border border-[var(--color-editorial-success-line)] bg-[var(--color-editorial-success-soft)] text-[var(--color-editorial-success)]"
                      : "border border-[var(--color-editorial-error-line)] bg-[var(--color-editorial-error-soft)] text-[var(--color-editorial-error)]"
                  }`}
                >
                  {event.change_type === "added" ? labels.added : labels.removed}
                </span>
                <span className="break-all text-[var(--color-editorial-ink)]">{event.model_id}</span>
                <span className="ml-auto text-xs text-[var(--color-editorial-ink-faint)]">{new Date(event.detected_at).toLocaleString()}</span>
              </div>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
