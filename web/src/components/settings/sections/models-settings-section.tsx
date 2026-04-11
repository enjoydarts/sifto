"use client";

import type { RefObject, FormEvent } from "react";
import { Brain } from "lucide-react";
import type { ModelOption } from "@/components/settings/model-select";
import ModelSelect from "@/components/settings/model-select";
import ProviderModelUpdatesPanel from "@/components/settings/provider-model-updates-panel";
import { SectionCard } from "@/components/ui/section-card";
import type { ProviderModelChangeEvent } from "@/lib/api";

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

type ModelField = {
  value: string;
  options: ModelOption[];
};

type UnavailableWarning = {
  key: string;
  label: string;
  modelLabel: string;
};

export default function ModelsSettingsSection({
  t,
  labels,
  form,
  models,
  actions,
  extras,
  unavailableWarnings,
}: {
  t: Translate;
  labels: ModelSelectLabels;
  form: {
    onSubmit: (event: FormEvent<HTMLFormElement>) => void;
    saving: boolean;
  };
  models: {
    summary: {
      facts: ModelField;
      factsSecondary: ModelField;
      factsSecondaryRatePercent: string;
      factsFallback: ModelField;
      summary: ModelField;
      summarySecondary: ModelField;
      summarySecondaryRatePercent: string;
      summaryFallback: ModelField;
    };
    digest: {
      digestCluster: ModelField;
      digest: ModelField;
    };
    validation: {
      factsCheck: ModelField;
      faithfulnessCheck: ModelField;
    };
    other: {
      sourceSuggestion: ModelField;
      ask: ModelField;
      embeddings: ModelField;
    };
    preprocess: {
      ttsMarkupPreprocess: ModelField;
    };
  };
  actions: {
    onChangeModel: (key: string, value: string) => void;
    onChangeRate: (key: "factsSecondaryRatePercent" | "summarySecondaryRatePercent", value: string) => void;
    onOpenModelGuide: () => void;
    onDismissProviderModelUpdates: () => void;
    onRestoreProviderModelUpdates: () => void;
  };
  extras: {
    llmExtrasOpen: boolean;
    llmExtrasRef: RefObject<HTMLDivElement | null>;
    providerModelUpdates: ProviderModelChangeEvent[];
    visibleProviderModelUpdates: ProviderModelChangeEvent[];
  };
  unavailableWarnings: UnavailableWarning[];
}) {
  const { onSubmit, saving } = form;
  const { summary, digest, validation, other, preprocess } = models;
  const { onChangeModel, onChangeRate, onOpenModelGuide, onDismissProviderModelUpdates, onRestoreProviderModelUpdates } = actions;
  const { llmExtrasOpen, llmExtrasRef, providerModelUpdates, visibleProviderModelUpdates } = extras;

  return (
    <div className="space-y-5">
      <SectionCard>
        <form onSubmit={onSubmit} className="space-y-4">
          <div>
            <h3 className="inline-flex items-center gap-2 text-base font-semibold text-[var(--color-editorial-ink)]">
              <Brain className="size-4 text-[var(--color-editorial-ink-faint)]" aria-hidden="true" />
              {t("settings.modelsTitle")}
            </h3>
            <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("settings.modelsDescription")}</p>
            <p className="mt-1 text-xs text-[var(--color-editorial-ink-faint)]">{t("settings.pricingDescription")}</p>
          </div>

          {unavailableWarnings.length > 0 ? (
            <div className="rounded-[16px] border border-[#e1cb9e] bg-[var(--color-warning-soft)] px-4 py-3 text-sm text-[var(--color-warning)]">
              <div className="font-medium">{t("settings.modelUnavailable.title")}</div>
              <div className="mt-2 space-y-1">
                {unavailableWarnings.map((entry) => (
                  <p key={entry.key}>
                    {t("settings.modelUnavailable.message")
                      .replace("{{field}}", entry.label)
                      .replace("{{model}}", entry.modelLabel)}
                  </p>
                ))}
              </div>
            </div>
          ) : null}

          <div className="space-y-4">
            <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
              <h4 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.group.summary")}</h4>
              <div className="mt-3 grid gap-4 md:grid-cols-2">
                <ModelSelect label={t("settings.model.facts")} value={summary.facts.value} onChange={(value) => onChangeModel("facts", value)} options={summary.facts.options} labels={labels} variant="modal" />
                <ModelSelect label={t("settings.model.factsSecondary")} value={summary.factsSecondary.value} onChange={(value) => onChangeModel("factsSecondary", value)} options={summary.factsSecondary.options} labels={labels} variant="modal" />
                <label className="space-y-2 text-sm text-[var(--color-editorial-ink-soft)]">
                  <span className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("settings.model.factsSecondaryRatePercent")}</span>
                  <input
                    type="number"
                    min={0}
                    max={100}
                    step={1}
                    value={summary.factsSecondaryRatePercent}
                    onChange={(event) => onChangeRate("factsSecondaryRatePercent", event.target.value)}
                    className="w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
                  />
                </label>
                <ModelSelect label={t("settings.model.factsFallback")} value={summary.factsFallback.value} onChange={(value) => onChangeModel("factsFallback", value)} options={summary.factsFallback.options} labels={labels} variant="modal" />
                <ModelSelect label={t("settings.model.summary")} value={summary.summary.value} onChange={(value) => onChangeModel("summary", value)} options={summary.summary.options} labels={labels} variant="modal" />
                <ModelSelect label={t("settings.model.summarySecondary")} value={summary.summarySecondary.value} onChange={(value) => onChangeModel("summarySecondary", value)} options={summary.summarySecondary.options} labels={labels} variant="modal" />
                <label className="space-y-2 text-sm text-[var(--color-editorial-ink-soft)]">
                  <span className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("settings.model.summarySecondaryRatePercent")}</span>
                  <input
                    type="number"
                    min={0}
                    max={100}
                    step={1}
                    value={summary.summarySecondaryRatePercent}
                    onChange={(event) => onChangeRate("summarySecondaryRatePercent", event.target.value)}
                    className="w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
                  />
                </label>
                <ModelSelect label={t("settings.model.summaryFallback")} value={summary.summaryFallback.value} onChange={(value) => onChangeModel("summaryFallback", value)} options={summary.summaryFallback.options} labels={labels} variant="modal" />
              </div>
            </section>

            <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
              <h4 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.group.digest")}</h4>
              <div className="mt-3 grid gap-4 md:grid-cols-2">
                <ModelSelect label={t("settings.model.digestCluster")} value={digest.digestCluster.value} onChange={(value) => onChangeModel("digestCluster", value)} options={digest.digestCluster.options} labels={labels} variant="modal" />
                <ModelSelect label={t("settings.model.digest")} value={digest.digest.value} onChange={(value) => onChangeModel("digest", value)} options={digest.digest.options} labels={labels} variant="modal" />
              </div>
            </section>

            <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
              <h4 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.group.validation")}</h4>
              <div className="mt-3 grid gap-4 md:grid-cols-2">
                <ModelSelect label={t("settings.model.factsCheck")} value={validation.factsCheck.value} onChange={(value) => onChangeModel("factsCheck", value)} options={validation.factsCheck.options} labels={labels} variant="modal" />
                <ModelSelect label={t("settings.model.faithfulnessCheck")} value={validation.faithfulnessCheck.value} onChange={(value) => onChangeModel("faithfulnessCheck", value)} options={validation.faithfulnessCheck.options} labels={labels} variant="modal" />
              </div>
            </section>

            <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
              <h4 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.group.other")}</h4>
              <div className="mt-3 grid gap-4 md:grid-cols-2">
                <ModelSelect label={t("settings.model.sourceSuggestion")} value={other.sourceSuggestion.value} onChange={(value) => onChangeModel("sourceSuggestion", value)} options={other.sourceSuggestion.options} labels={labels} variant="modal" />
                <ModelSelect label={t("settings.model.ask")} value={other.ask.value} onChange={(value) => onChangeModel("ask", value)} options={other.ask.options} labels={labels} variant="modal" />
                <ModelSelect label={t("settings.model.embeddings")} value={other.embeddings.value} onChange={(value) => onChangeModel("embeddings", value)} options={other.embeddings.options} labels={labels} variant="modal" />
              </div>
            </section>

            <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
              <h4 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.group.ttsMarkupPreprocess")}</h4>
              <p className="mt-1 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                {t("settings.group.ttsMarkupPreprocessDescription")}
              </p>
              <div className="mt-3 grid gap-4 md:grid-cols-2">
                <ModelSelect
                  label={t("settings.model.ttsMarkupPreprocess")}
                  value={preprocess.ttsMarkupPreprocess.value}
                  onChange={(value) => onChangeModel("ttsMarkupPreprocess", value)}
                  options={preprocess.ttsMarkupPreprocess.options}
                  labels={labels}
                  variant="modal"
                />
              </div>
            </section>
          </div>

          <button
            type="submit"
            disabled={saving}
            className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:opacity-60"
          >
            {saving ? t("common.saving") : t("settings.saveModels")}
          </button>
        </form>
      </SectionCard>

      {llmExtrasOpen ? (
        <div ref={llmExtrasRef} className="space-y-5">
          <ProviderModelUpdatesPanel
            allEvents={providerModelUpdates}
            visibleEvents={visibleProviderModelUpdates}
            onDismiss={onDismissProviderModelUpdates}
            onRestore={onRestoreProviderModelUpdates}
            labels={{
              title: t("settings.providerModelUpdates"),
              description: t("settings.providerModelUpdatesDescription"),
              dismiss: t("settings.providerModelUpdate.dismiss"),
              empty: t("settings.providerModelUpdate.empty"),
              dismissed: t("settings.providerModelUpdate.dismissed"),
              restore: t("settings.providerModelUpdate.restore"),
              added: t("settings.providerModelUpdate.added", "added"),
              removed: t("settings.providerModelUpdate.removed", "removed"),
            }}
          />
          <SectionCard>
            <div className="flex items-start justify-between gap-3">
              <div>
                <h3 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.modelGuide.title")}</h3>
                <p className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{t("settings.modelGuide.description")}</p>
              </div>
              <button
                type="button"
                onClick={onOpenModelGuide}
                className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
              >
                {t("settings.modelGuide.open")}
              </button>
            </div>
          </SectionCard>
        </div>
      ) : null}
    </div>
  );
}
