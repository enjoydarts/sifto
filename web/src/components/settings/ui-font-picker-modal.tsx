"use client";

import { useEffect, useMemo, useState } from "react";
import { RotateCcw, Search, X } from "lucide-react";
import { UIFontCatalogEntry, ensureUIFontPreviewLoaded } from "@/lib/ui-fonts";
import { useI18n } from "@/components/i18n-provider";

function joinClassNames(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

export default function UIFontPickerModal({
  open,
  kind,
  title,
  subtitle,
  fonts,
  currentKey,
  selectedKey,
  defaultKey,
  onClose,
  onSelect,
}: {
  open: boolean;
  kind: "sans" | "serif";
  title: string;
  subtitle: string;
  fonts: UIFontCatalogEntry[];
  currentKey: string;
  selectedKey: string;
  defaultKey: string;
  onClose: () => void;
  onSelect: (key: string) => void;
}) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");

  useEffect(() => {
    if (!open) {
      setQuery("");
    }
  }, [open]);

  const currentFont = useMemo(
    () => fonts.find((font) => font.key === currentKey) ?? fonts.find((font) => font.key === defaultKey) ?? fonts[0] ?? null,
    [currentKey, defaultKey, fonts],
  );
  const selectedFont = useMemo(
    () => fonts.find((font) => font.key === selectedKey) ?? currentFont,
    [currentFont, fonts, selectedKey],
  );

  useEffect(() => {
    if (!open || !selectedFont) return;
    ensureUIFontPreviewLoaded(selectedFont.key);
  }, [open, selectedFont]);

  const filteredFonts = useMemo(() => {
    const needle = query.trim().toLowerCase();
    const base = [...fonts];
    base.sort((a, b) => {
      if (a.key === selectedKey && b.key !== selectedKey) return -1;
      if (b.key === selectedKey && a.key !== selectedKey) return 1;
      if (a.key === currentKey && b.key !== currentKey) return -1;
      if (b.key === currentKey && a.key !== currentKey) return 1;
      return a.label.localeCompare(b.label, "ja");
    });
    if (!needle) return base;
    return base.filter((font) => {
      const haystack = `${font.label} ${font.family} ${font.key}`.toLowerCase();
      return haystack.includes(needle);
    });
  }, [currentKey, fonts, query, selectedKey]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6" onClick={onClose}>
      <div
        className="flex max-h-[92vh] w-full max-w-6xl flex-col overflow-hidden rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] shadow-[0_30px_80px_rgba(35,24,12,0.24)]"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex items-start justify-between gap-4 border-b border-[var(--color-editorial-line)] px-5 py-4">
          <div>
            <h2 className="text-base font-semibold text-[var(--color-editorial-ink)]">{title}</h2>
            <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{subtitle}</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="inline-flex size-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-white text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
            aria-label={t("common.close")}
          >
            <X className="size-4" />
          </button>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto lg:grid lg:overflow-hidden lg:grid-cols-[minmax(0,0.95fr)_minmax(320px,0.8fr)]">
          <div className="min-h-0 border-b border-[var(--color-editorial-line)] lg:flex lg:flex-col lg:border-b-0 lg:border-r">
            <div className="shrink-0 border-b border-[var(--color-editorial-line)] px-5 py-4">
              <div className="flex items-center gap-3 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-3">
                <Search className="size-4 text-[var(--color-editorial-ink-soft)]" />
                <input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder={t("settings.uiFonts.search")}
                  className="w-full bg-transparent text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
                />
              </div>
            </div>
            <div className="min-h-0 px-4 py-4 lg:overflow-auto">
              <div className="space-y-3">
                {filteredFonts.length === 0 ? (
                  <div className="rounded-[22px] border border-dashed border-[var(--color-editorial-line)] bg-white/70 px-5 py-8 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                    {t("settings.uiFonts.noResults")}
                  </div>
                ) : (
                  filteredFonts.map((font) => {
                    const isSelected = selectedFont?.key === font.key;
                    const isCurrent = currentFont?.key === font.key;
                    return (
                      <button
                        key={font.key}
                        type="button"
                        onClick={() => onSelect(font.key)}
                        className={joinClassNames(
                          "flex w-full items-start justify-between gap-3 rounded-[20px] border px-4 py-4 text-left transition",
                          isSelected
                            ? "border-[var(--color-editorial-ink)] bg-[var(--color-editorial-panel)]"
                            : "border-[var(--color-editorial-line)] bg-white hover:bg-[var(--color-editorial-panel)]",
                        )}
                      >
                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-2">
                            <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{font.label}</div>
                            <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2 py-0.5 text-[11px] uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">
                              {font.category}
                            </span>
                            {isCurrent ? (
                              <span className="rounded-full border border-[var(--color-editorial-success-line)] bg-[var(--color-editorial-success-soft)] px-2 py-0.5 text-[11px] text-[var(--color-editorial-success)]">
                                {t("settings.uiFonts.current")}
                              </span>
                            ) : null}
                            {isSelected && !isCurrent ? (
                              <span className="rounded-full border border-[var(--color-editorial-accent-line)] bg-[var(--color-editorial-accent-soft)] px-2 py-0.5 text-[11px] text-[var(--color-editorial-accent)]">
                                {t("settings.uiFonts.pending")}
                              </span>
                            ) : null}
                          </div>
                          <div className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{font.family}</div>
                        </div>
                      </button>
                    );
                  })
                )}
              </div>
            </div>
          </div>

          <div className="flex min-h-0 flex-col bg-[var(--color-editorial-panel-strong)] px-5 py-5">
            {selectedFont ? (
              <>
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.uiFonts.preview")}</div>
                    <h3 className="mt-1 text-lg font-semibold text-[var(--color-editorial-ink)]">{selectedFont.label}</h3>
                    <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{selectedFont.family}</p>
                  </div>
                  <button
                    type="button"
                    onClick={() => onSelect(defaultKey)}
                    className="inline-flex items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-white px-3 py-2 text-xs font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                  >
                    <RotateCcw className="size-3.5" />
                    {t("settings.uiFonts.resetDefault")}
                  </button>
                </div>

                <div className="mt-5 rounded-[24px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-5 py-5">
                  <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">
                    {t("settings.uiFonts.previewHeading")}
                  </div>
                    <div className="mt-3" style={{ fontFamily: kind === "sans" ? `"${selectedFont.family}", sans-serif` : `"${selectedFont.family}", serif` }}>
                    <div className="text-2xl font-semibold leading-snug text-[var(--color-editorial-ink)]">
                      {selectedFont.preview_ui}
                    </div>
                    <p className="mt-4 text-[15px] leading-8 text-[var(--color-editorial-ink-soft)]">
                      {selectedFont.preview_body}
                    </p>
                    <div className="mt-4 text-sm text-[var(--color-editorial-ink-faint)]">2026 / 04 / 08 08:00</div>
                    <div className="mt-2 text-sm text-[var(--color-editorial-ink-faint)]">0123456789 AaBbCc</div>
                  </div>
                </div>

                <div className="mt-4 rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-4 text-sm text-[var(--color-editorial-ink-soft)]">
                  {selectedFont.key === currentKey
                    ? t("settings.uiFonts.currentDescription")
                    : t("settings.uiFonts.pendingDescription")}
                </div>
              </>
            ) : null}
          </div>
        </div>
      </div>
    </div>
  );
}
