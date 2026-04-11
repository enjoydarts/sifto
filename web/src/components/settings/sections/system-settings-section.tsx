"use client";

import type { FormEvent } from "react";
import { KeyRound } from "lucide-react";
import ApiKeyCard from "@/components/settings/api-key-card";
import { SectionCard } from "@/components/ui/section-card";

type Translate = (key: string, fallback?: string) => string;

type AccessCard = {
  id: string;
  title: string;
  description: string;
  configured: boolean;
  last4: string | null | undefined;
  value: string;
  onChange: (value: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onDelete: () => void;
  placeholder: string;
  saving: boolean;
  deleting: boolean;
  notSet: string;
};

type ApiKeyCardLabels = {
  configured: string;
  newApiKey: string;
  saveOrUpdate: string;
  saving: string;
  deleteKey: string;
  deleting: string;
};

type UIFontPreview = {
  label?: string | null;
  family?: string | null;
  preview_ui?: string | null;
};

export default function SystemSettingsSection({
  t,
  uiFonts,
  access,
}: {
  t: Translate;
  uiFonts: {
    onSubmit: (event: FormEvent<HTMLFormElement>) => void;
    saving: boolean;
    dirty: boolean;
    selectedSans: UIFontPreview | null | undefined;
    selectedSerif: UIFontPreview | null | undefined;
    onOpenSansPicker: () => void;
    onOpenSerifPicker: () => void;
  };
  access: {
    configuredProviderCount: number;
    accessCards: AccessCard[];
    activeAccessCard: AccessCard | undefined;
    apiKeyCardLabels: ApiKeyCardLabels;
    onSelectProvider: (provider: string) => void;
  };
}) {
  const { onSubmit, saving, dirty, selectedSans, selectedSerif, onOpenSansPicker, onOpenSerifPicker } = uiFonts;
  const { configuredProviderCount, accessCards, activeAccessCard, apiKeyCardLabels, onSelectProvider } = access;

  return (
    <div className="space-y-5">
      <SectionCard>
        <form onSubmit={onSubmit}>
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div>
              <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.uiFonts.title")}</div>
              <div className="mt-1 text-xs text-[var(--color-editorial-ink-faint)]">{t("settings.uiFonts.description")}</div>
            </div>
            <button
              type="submit"
              disabled={saving || !dirty}
              className="inline-flex min-h-10 items-center rounded-full bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] transition disabled:cursor-not-allowed disabled:opacity-50"
            >
              {saving ? t("common.saving") : t("common.save")}
            </button>
          </div>

          <div className="mt-4 grid gap-3 lg:grid-cols-2">
            <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.uiFonts.sansLabel")}</div>
                  <div className="mt-1 text-sm font-semibold text-[var(--color-editorial-ink)]">
                    {selectedSans?.label ?? t("settings.uiFonts.defaultSansLabel")}
                  </div>
                  <div className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">
                    {selectedSans?.family ?? "Sawarabi Gothic"}
                  </div>
                </div>
                <button
                  type="button"
                  onClick={onOpenSansPicker}
                  className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                >
                  {t("settings.uiFonts.choose")}
                </button>
              </div>
              <div className="mt-4 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-4">
                <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.uiFonts.preview")}</div>
                <div
                  className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink)]"
                  style={{ fontFamily: selectedSans ? `"${selectedSans.family}", sans-serif` : undefined }}
                >
                  {selectedSans?.preview_ui ?? t("settings.uiFonts.defaultSansPreview")}
                </div>
              </div>
            </div>

            <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.uiFonts.serifLabel")}</div>
                  <div className="mt-1 text-sm font-semibold text-[var(--color-editorial-ink)]">
                    {selectedSerif?.label ?? t("settings.uiFonts.defaultSerifLabel")}
                  </div>
                  <div className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">
                    {selectedSerif?.family ?? "Sawarabi Mincho"}
                  </div>
                </div>
                <button
                  type="button"
                  onClick={onOpenSerifPicker}
                  className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                >
                  {t("settings.uiFonts.choose")}
                </button>
              </div>
              <div className="mt-4 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-4">
                <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.uiFonts.preview")}</div>
                <div
                  className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink)]"
                  style={{ fontFamily: selectedSerif ? `"${selectedSerif.family}", serif` : undefined }}
                >
                  {selectedSerif?.preview_ui ?? t("settings.uiFonts.defaultSerifPreview")}
                </div>
              </div>
            </div>
          </div>

          <div className="mt-4 rounded-[20px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-3 text-sm text-[var(--color-editorial-ink-soft)]">
            {dirty ? t("settings.uiFonts.pendingNotice") : t("settings.uiFonts.savedNotice")}
          </div>
        </form>
      </SectionCard>

      <SectionCard>
        <div>
          <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.access.selectProvider")}</div>
          <div className="mt-1 text-xs text-[var(--color-editorial-ink-faint)]">
            {`${t("settings.access.configuredProviders")}: ${configuredProviderCount}/${accessCards.length}`}
          </div>
        </div>
        <div className="mt-4 grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
          {accessCards.map((card) => {
            const selected = card.id === activeAccessCard?.id;
            return (
              <button
                key={card.id}
                type="button"
                onClick={() => onSelectProvider(card.id)}
                className={[
                  "rounded-[18px] border px-4 py-3 text-left transition",
                  selected
                    ? "border-[var(--color-editorial-line-strong)] bg-[var(--color-editorial-panel-strong)] shadow-[var(--shadow-card)]"
                    : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] hover:bg-[var(--color-editorial-panel-strong)]",
                ].join(" ")}
              >
                <div className="flex items-center justify-between gap-2">
                  <div className="text-sm font-medium text-[var(--color-editorial-ink)]">{card.title.replace(/（.*?）|\(.*?\)/g, "").trim()}</div>
                  <span
                    className={[
                      "rounded-full px-2 py-0.5 text-[11px] font-medium",
                      card.configured
                        ? "border border-[var(--color-editorial-success-line)] bg-[var(--color-editorial-success-soft)] text-[var(--color-editorial-success)]"
                        : "border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] text-[var(--color-editorial-ink-faint)]",
                    ].join(" ")}
                  >
                    {card.configured ? t("settings.configured") : t("settings.access.notConfiguredShort")}
                  </span>
                </div>
                <div className="mt-2 text-xs text-[var(--color-editorial-ink-soft)]">
                  {card.configured ? `••••${card.last4 ?? "****"}` : card.notSet}
                </div>
              </button>
            );
          })}
        </div>
      </SectionCard>

      {activeAccessCard ? (
        <ApiKeyCard
          icon={KeyRound}
          title={activeAccessCard.title}
          description={activeAccessCard.description}
          configured={activeAccessCard.configured}
          last4={activeAccessCard.last4}
          value={activeAccessCard.value}
          onChange={activeAccessCard.onChange}
          onSubmit={activeAccessCard.onSubmit}
          onDelete={activeAccessCard.onDelete}
          placeholder={activeAccessCard.placeholder}
          saving={activeAccessCard.saving}
          deleting={activeAccessCard.deleting}
          labels={{ ...apiKeyCardLabels, notSet: activeAccessCard.notSet }}
        />
      ) : null}
    </div>
  );
}
