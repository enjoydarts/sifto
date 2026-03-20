"use client";

import { FormEvent, useState } from "react";
import { Info } from "lucide-react";
import { ItemHighlight } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";

type ItemHighlightListProps = {
  highlights: ItemHighlight[];
  onCreate: (input: { quote_text: string; anchor_text?: string; section?: string }) => Promise<void>;
  onDelete: (highlightId: string) => Promise<void>;
  disabled?: boolean;
};

export function ItemHighlightList({ highlights, onCreate, onDelete, disabled = false }: ItemHighlightListProps) {
  const { t } = useI18n();
  const [quoteText, setQuoteText] = useState("");
  const [anchorText, setAnchorText] = useState("");
  const [section, setSection] = useState("summary");
  const [saving, setSaving] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!quoteText.trim()) return;
    setSaving(true);
    try {
      await onCreate({
        quote_text: quoteText.trim(),
        anchor_text: anchorText.trim() || undefined,
        section: section.trim() || undefined,
      });
      setQuoteText("");
      setAnchorText("");
      setSection("summary");
    } finally {
      setSaving(false);
    }
  }

  return (
    <section className="flex h-full flex-col rounded-xl border border-zinc-200 bg-zinc-50 p-4">
      <div className="flex items-center justify-between gap-3">
        <h4 className="text-sm font-semibold text-zinc-900">{t("itemHighlight.title")}</h4>
        <button
          type="submit"
          form="inline-reader-highlight-form"
          disabled={saving || disabled}
          className="shrink-0 rounded-lg bg-zinc-900 px-3 py-2 text-xs font-medium text-white disabled:opacity-60"
        >
          {saving ? t("common.saving") : t("itemHighlight.add")}
        </button>
      </div>
      <p className="mt-3 text-xs leading-5 text-zinc-500">{t("itemHighlight.help")}</p>
      <form id="inline-reader-highlight-form" onSubmit={submit} className="mt-3 space-y-2">
        <div>
          <div className="mb-1 flex items-center gap-1 text-xs font-medium text-zinc-600">
            <span>{t("itemHighlight.sectionLabel")}</span>
            <span title={t("itemHighlight.sectionHelp")}>
              <Info className="size-3.5 text-zinc-400" aria-hidden="true" />
            </span>
          </div>
          <input
            value={section}
            onChange={(e) => setSection(e.target.value)}
            disabled={disabled}
            className="w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900"
            placeholder={t("itemHighlight.sectionPlaceholder")}
          />
        </div>
        <div>
          <div className="mb-1 flex items-center gap-1 text-xs font-medium text-zinc-600">
            <span>{t("itemHighlight.quoteLabel")}</span>
            <span title={t("itemHighlight.quoteHelp")}>
              <Info className="size-3.5 text-zinc-400" aria-hidden="true" />
            </span>
          </div>
        <textarea
          value={quoteText}
          onChange={(e) => setQuoteText(e.target.value)}
          rows={3}
          disabled={disabled}
          className="w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900"
          placeholder={t("itemHighlight.quotePlaceholder")}
        />
        </div>
        <div className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_140px]">
          <div>
            <div className="mb-1 flex items-center gap-1 text-xs font-medium text-zinc-600">
              <span>{t("itemHighlight.anchorLabel")}</span>
              <span title={t("itemHighlight.anchorHelp")}>
                <Info className="size-3.5 text-zinc-400" aria-hidden="true" />
              </span>
            </div>
            <input
              value={anchorText}
              onChange={(e) => setAnchorText(e.target.value)}
              disabled={disabled}
              className="w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900"
              placeholder={t("itemHighlight.anchorPlaceholder")}
            />
          </div>
          <div className="rounded-lg border border-dashed border-zinc-200 bg-white px-3 py-2 text-xs text-zinc-500">
            {highlights.length} {t("itemHighlight.countSuffix")}
          </div>
        </div>
      </form>
      <div className="mt-4 flex-1 space-y-2">
        {highlights.map((highlight) => (
          <article key={highlight.id} className="rounded-lg border border-zinc-200 bg-white px-3 py-3">
            <div className="flex flex-wrap items-start justify-between gap-2">
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap gap-1.5 text-[11px] font-medium text-zinc-500">
                  {[highlight.section, highlight.anchor_text]
                    .filter(Boolean)
                    .map((value) => (
                      <span key={`${highlight.id}-${value}`} className="rounded-full bg-zinc-100 px-2 py-0.5 text-zinc-600">
                        {value}
                      </span>
                    ))}
                </div>
                <p className="mt-2 break-words text-sm leading-6 text-zinc-800">{highlight.quote_text}</p>
              </div>
              <button
                type="button"
                disabled={disabled}
                onClick={() => void onDelete(highlight.id)}
                className="shrink-0 rounded-md px-2 py-1 text-xs font-medium text-rose-600 hover:bg-rose-50 hover:text-rose-700 disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-transparent"
              >
                {t("itemHighlight.delete")}
              </button>
            </div>
          </article>
        ))}
      </div>
    </section>
  );
}
