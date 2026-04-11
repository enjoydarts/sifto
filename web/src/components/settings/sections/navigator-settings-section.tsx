"use client";

import type { FormEvent } from "react";
import { AINavigatorAvatar } from "@/components/briefing/ai-navigator-avatar";
import ModelSelect, { type ModelOption } from "@/components/settings/model-select";
import { SectionCard } from "@/components/ui/section-card";

type Translate = (key: string, fallback?: string) => string;

type ModelSelectLabels = {
  defaultOption: string;
  searchPlaceholder: string;
  noResults: string;
  providerAll: string;
  modalChoose: string;
  close: string;
  confirmTitle: string;
  confirmYes: string;
  confirmNo: string;
  confirmSuffix: string;
  providerColumn: string;
  modelColumn: string;
  pricingColumn: string;
};

type NavigatorPersonaCard = {
  key: string;
  name: string;
  occupation?: string;
  gender?: string;
  age_vibe?: string;
  personality: string;
  first_person: string;
  speech_style: string;
  experience: string;
  values: string;
  interests: string;
  dislikes: string;
  voice: string;
  briefing?: {
    comment_range?: string;
    intro_range?: string;
    intro_style?: string;
  };
  item?: {
    style?: string;
  };
};

export default function NavigatorSettingsSection({
  t,
  modelSelectLabels,
  form,
  state,
  actions,
}: {
  t: Translate;
  modelSelectLabels: ModelSelectLabels;
  form: {
    onSubmit: (event: FormEvent<HTMLFormElement>) => void;
    saving: boolean;
  };
  state: {
    enabled: boolean;
    aiNavigatorBriefEnabled: boolean;
    personaMode: "fixed" | "random";
    persona: string;
    navigatorPersonaCards: NavigatorPersonaCard[];
    navigatorModel: string;
    navigatorModelOptions: ModelOption[];
    navigatorFallbackModel: string;
    navigatorFallbackModelOptions: ModelOption[];
    aiNavigatorBriefModel: string;
    aiNavigatorBriefModelOptions: ModelOption[];
    aiNavigatorBriefFallbackModel: string;
    aiNavigatorBriefFallbackModelOptions: ModelOption[];
  };
  actions: {
    onChangeEnabled: (value: boolean) => void;
    onChangeBriefEnabled: (value: boolean) => void;
    onChangePersonaMode: (value: "fixed" | "random") => void;
    onSelectPersona: (personaKey: string) => void;
    onChangeModel: (key: "navigator" | "navigatorFallback" | "aiNavigatorBrief" | "aiNavigatorBriefFallback", value: string) => void;
  };
}) {
  const { onSubmit, saving } = form;
  const {
    enabled,
    aiNavigatorBriefEnabled,
    personaMode,
    persona,
    navigatorPersonaCards,
    navigatorModel,
    navigatorModelOptions,
    navigatorFallbackModel,
    navigatorFallbackModelOptions,
    aiNavigatorBriefModel,
    aiNavigatorBriefModelOptions,
    aiNavigatorBriefFallbackModel,
    aiNavigatorBriefFallbackModelOptions,
  } = state;
  const {
    onChangeEnabled,
    onChangeBriefEnabled,
    onChangePersonaMode,
    onSelectPersona,
    onChangeModel,
  } = actions;

  return (
    <SectionCard>
      <form onSubmit={onSubmit} className="space-y-5">
        <div>
          <h3 className="text-base font-semibold text-[var(--color-editorial-ink)]">{t("settings.group.navigator")}</h3>
          <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("settings.navigator.description")}</p>
        </div>

        <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
          <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
            <div>
              <h4 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.enabled")}</h4>
              <p className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{t("settings.navigator.enabledHelp")}</p>
            </div>
            <label className="inline-flex min-h-10 items-center gap-2 self-center text-sm text-[var(--color-editorial-ink)] md:justify-self-end">
              <input
                type="checkbox"
                checked={enabled}
                onChange={(e) => onChangeEnabled(e.target.checked)}
                className="size-4 rounded border-[var(--color-editorial-line)] text-[var(--color-editorial-ink)] focus:ring-[var(--color-editorial-ink)]"
              />
              {enabled ? t("settings.on") : t("settings.off")}
            </label>
          </div>
        </section>

        <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
          <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
            <div>
              <h4 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.briefEnabled")}</h4>
              <p className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{t("settings.navigator.briefEnabledHelp")}</p>
            </div>
            <label className="inline-flex min-h-10 items-center gap-2 self-center text-sm text-[var(--color-editorial-ink)] md:justify-self-end">
              <input
                type="checkbox"
                checked={aiNavigatorBriefEnabled}
                onChange={(e) => onChangeBriefEnabled(e.target.checked)}
                className="size-4 rounded border-[var(--color-editorial-line)] text-[var(--color-editorial-ink)] focus:ring-[var(--color-editorial-ink)]"
              />
              {aiNavigatorBriefEnabled ? t("settings.on") : t("settings.off")}
            </label>
          </div>
        </section>

        <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
          <h4 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.persona")}</h4>
          <div className="mt-4 flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <label className="flex min-w-[220px] flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                {t("settings.personaMode.label")}
              </div>
              <select
                value={personaMode}
                onChange={(e) => onChangePersonaMode(e.target.value === "random" ? "random" : "fixed")}
                className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
              >
                <option value="fixed">{t("settings.personaMode.fixed")}</option>
                <option value="random">{t("settings.personaMode.random")}</option>
              </select>
            </label>
            <p className="max-w-[560px] text-xs leading-6 text-[var(--color-editorial-ink-soft)]">
              {personaMode === "random" ? t("settings.navigator.randomPersonaHelp") : t("settings.navigator.fixedPersonaHelp")}
            </p>
          </div>
          <div className="mt-4 grid gap-3 lg:grid-cols-2">
            {navigatorPersonaCards.map((item) => {
              const selected = personaMode === "fixed" && item.key === persona;
              const briefingHints = item.briefing ?? {};
              const itemHints = item.item ?? {};
              return (
                <button
                  key={item.key}
                  type="button"
                  onClick={() => onSelectPersona(item.key)}
                  className={[
                    "rounded-[18px] border bg-[var(--color-editorial-panel)] p-4 text-left transition hover:bg-[var(--color-editorial-panel-strong)]",
                    personaMode !== "fixed" ? "cursor-default opacity-70" : "",
                    selected
                      ? "border-[var(--color-editorial-ink)] shadow-[0_12px_32px_rgba(58,42,27,0.08)]"
                      : "border-[var(--color-editorial-line)]",
                  ].join(" ")}
                  aria-pressed={selected}
                  disabled={personaMode !== "fixed" || saving}
                >
                  <div className="flex items-start gap-3">
                    <div className="shrink-0 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-1.5">
                      <AINavigatorAvatar persona={item.key} className="size-11" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{item.name}</div>
                        {selected ? (
                          <span className="rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--color-editorial-panel-strong)]">
                            {t("settings.navigator.card.selected")}
                          </span>
                        ) : null}
                      </div>
                      <p className="mt-1 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                        {[item.occupation, item.gender, item.age_vibe].filter(Boolean).join(" / ")}
                      </p>
                    </div>
                  </div>
                  <dl className="mt-4 space-y-3 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                    <div>
                      <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.personalityLabel")}</dt>
                      <dd>{item.personality}</dd>
                    </div>
                    <div>
                      <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.firstPersonLabel")}</dt>
                      <dd>{item.first_person}</dd>
                    </div>
                    <div>
                      <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.speechLabel")}</dt>
                      <dd>{item.speech_style}</dd>
                    </div>
                    <div>
                      <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.experienceLabel")}</dt>
                      <dd>{item.experience}</dd>
                    </div>
                    <div>
                      <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.valuesLabel")}</dt>
                      <dd>{item.values}</dd>
                    </div>
                    <div>
                      <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.interestsLabel")}</dt>
                      <dd>{item.interests}</dd>
                    </div>
                    <div>
                      <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.dislikesLabel")}</dt>
                      <dd>{item.dislikes}</dd>
                    </div>
                    <div>
                      <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.voiceLabel")}</dt>
                      <dd>{item.voice}</dd>
                    </div>
                    <div>
                      <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.briefingCommentRangeLabel")}</dt>
                      <dd>{briefingHints.comment_range || "-"}</dd>
                    </div>
                    <div>
                      <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.briefingIntroRangeLabel")}</dt>
                      <dd>{briefingHints.intro_range || "-"}</dd>
                    </div>
                    <div>
                      <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.briefingIntroStyleLabel")}</dt>
                      <dd>{briefingHints.intro_style || "-"}</dd>
                    </div>
                    <div>
                      <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.itemStyleLabel")}</dt>
                      <dd>{itemHints.style || "-"}</dd>
                    </div>
                  </dl>
                </button>
              );
            })}
          </div>
        </section>

        <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
          <h4 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.modelsTitle")}</h4>
          <div className="mt-3 grid gap-4 md:grid-cols-2">
            <ModelSelect label={t("settings.model.navigator")} value={navigatorModel} onChange={(value) => onChangeModel("navigator", value)} options={navigatorModelOptions} labels={modelSelectLabels} variant="modal" />
            <ModelSelect label={t("settings.model.navigatorFallback")} value={navigatorFallbackModel} onChange={(value) => onChangeModel("navigatorFallback", value)} options={navigatorFallbackModelOptions} labels={modelSelectLabels} variant="modal" />
            <ModelSelect label={t("settings.model.aiNavigatorBrief")} value={aiNavigatorBriefModel} onChange={(value) => onChangeModel("aiNavigatorBrief", value)} options={aiNavigatorBriefModelOptions} labels={modelSelectLabels} variant="modal" />
            <ModelSelect label={t("settings.model.aiNavigatorBriefFallback")} value={aiNavigatorBriefFallbackModel} onChange={(value) => onChangeModel("aiNavigatorBriefFallback", value)} options={aiNavigatorBriefFallbackModelOptions} labels={modelSelectLabels} variant="modal" />
          </div>
        </section>

        <button
          type="submit"
          disabled={saving}
          className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:opacity-60"
        >
          {saving ? t("common.saving") : t("settings.saveModels")}
        </button>
      </form>
    </SectionCard>
  );
}
