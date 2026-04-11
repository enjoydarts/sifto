"use client";

import type { Dispatch, SetStateAction } from "react";
import { api } from "@/lib/api";
import { runConfirmedAction, runSavingAction } from "@/components/settings/settings-submit-actions";

type Translator = (key: string, fallback?: string) => string;

export async function saveBudgetSettingsAction(args: {
  budgetUSD: string;
  alertEnabled: boolean;
  thresholdPct: number;
  digestEmailEnabled: boolean;
  setSaving: (value: boolean) => void;
  showToast: (message: string, tone?: "success" | "error" | "info") => void;
  t: Translator;
  reload: () => Promise<unknown>;
}) {
  const { budgetUSD, alertEnabled, thresholdPct, digestEmailEnabled, setSaving, showToast, t, reload } = args;
  await runSavingAction({
    setSaving,
    showToast,
    successMessage: t("settings.toast.budgetSaved"),
    run: async () => {
      const parsed = budgetUSD.trim() === "" ? null : Number(budgetUSD);
      if (parsed != null && (!Number.isFinite(parsed) || parsed < 0)) {
        throw new Error(t("settings.error.invalidBudget"));
      }
      await api.updateSettings({
        monthly_budget_usd: parsed,
        budget_alert_enabled: alertEnabled,
        budget_alert_threshold_pct: thresholdPct,
        digest_email_enabled: digestEmailEnabled,
      });
      await reload();
    },
  });
}

export async function saveDigestDeliveryAction(args: {
  monthlyBudgetUSD: number | null | undefined;
  budgetAlertEnabled: boolean | null | undefined;
  budgetAlertThresholdPct: number | null | undefined;
  digestEmailEnabled: boolean;
  setSaving: (value: boolean) => void;
  showToast: (message: string, tone?: "success" | "error" | "info") => void;
  t: Translator;
  reload: () => Promise<unknown>;
}) {
  const {
    monthlyBudgetUSD,
    budgetAlertEnabled,
    budgetAlertThresholdPct,
    digestEmailEnabled,
    setSaving,
    showToast,
    t,
    reload,
  } = args;
  await runSavingAction({
    setSaving,
    showToast,
    successMessage: t("settings.toast.digestSaved"),
    run: async () => {
      await api.updateSettings({
        monthly_budget_usd: monthlyBudgetUSD ?? null,
        budget_alert_enabled: Boolean(budgetAlertEnabled),
        budget_alert_threshold_pct: budgetAlertThresholdPct ?? 20,
        digest_email_enabled: digestEmailEnabled,
      });
      await reload();
    },
  });
}

export async function saveReadingPlanAction(args: {
  readingPlanWindow: "24h" | "today_jst" | "7d";
  readingPlanSize: string;
  readingPlanDiversifyTopics: boolean;
  setSaving: (value: boolean) => void;
  showToast: (message: string, tone?: "success" | "error" | "info") => void;
  t: Translator;
  reload: () => Promise<unknown>;
}) {
  const { readingPlanWindow, readingPlanSize, readingPlanDiversifyTopics, setSaving, showToast, t, reload } = args;
  await runSavingAction({
    setSaving,
    showToast,
    successMessage: t("settings.toast.readingPlanSaved"),
    run: async () => {
      const parsedSize = Number(readingPlanSize);
      if (!(parsedSize === 7 || parsedSize === 15 || parsedSize === 25)) {
        throw new Error(t("settings.error.invalidSize"));
      }
      await api.updateReadingPlanSettings({
        window: readingPlanWindow,
        size: parsedSize,
        diversify_topics: readingPlanDiversifyTopics,
      });
      await reload();
    },
  });
}

export async function saveObsidianExportAction(args: {
  enabled: boolean;
  repoOwner: string;
  repoName: string;
  repoBranch: string;
  rootPath: string;
  setSaving: (value: boolean) => void;
  showToast: (message: string, tone?: "success" | "error" | "info") => void;
  t: Translator;
  setSettings: Dispatch<SetStateAction<import("@/lib/api").UserSettings | null>>;
}) {
  const { enabled, repoOwner, repoName, repoBranch, rootPath, setSaving, showToast, t, setSettings } = args;
  await runSavingAction({
    setSaving,
    showToast,
    successMessage: t("settings.toast.obsidianExportSaved"),
    run: async () => {
      const resp = await api.updateObsidianExport({
        enabled,
        github_repo_owner: repoOwner.trim() || null,
        github_repo_name: repoName.trim() || null,
        github_repo_branch: repoBranch.trim() || null,
        vault_root_path: rootPath.trim() || null,
        keyword_link_mode: "topics_only",
      });
      setSettings((prev) => (prev ? { ...prev, obsidian_export: resp.obsidian_export } : prev));
    },
  });
}

export async function runObsidianExportNowAction(args: {
  setSaving: (value: boolean) => void;
  showToast: (message: string, tone?: "success" | "error" | "info") => void;
  t: Translator;
  reload: () => Promise<unknown>;
}) {
  const { setSaving, showToast, t, reload } = args;
  await runSavingAction({
    setSaving,
    showToast,
    run: async () => {
      const res = await api.runObsidianExportNow();
      await reload();
      showToast(
        `${t("settings.toast.obsidianExportRunNowResult")} updated=${res.updated} skipped=${res.skipped} failed=${res.failed}`,
        res.failed > 0 ? "error" : "success",
      );
    },
  });
}

export async function saveAivisUserDictionaryAction(args: {
  dictionaryUUID: string;
  setSaving: (value: boolean) => void;
  showToast: (message: string, tone?: "success" | "error" | "info") => void;
  t: Translator;
  setDictionaryUUID: (value: string) => void;
  setSettings: Dispatch<SetStateAction<import("@/lib/api").UserSettings | null>>;
}) {
  const { dictionaryUUID, setSaving, showToast, t, setDictionaryUUID, setSettings } = args;
  if (!dictionaryUUID) {
    showToast(t("settings.aivisDictionarySelectRequired"), "error");
    return;
  }
  await runSavingAction({
    setSaving,
    showToast,
    successMessage: t("settings.toast.aivisDictionarySaved"),
    run: async () => {
      const next = await api.setAivisUserDictionary(dictionaryUUID);
      setDictionaryUUID(next.aivis_user_dictionary_uuid ?? "");
      setSettings((prev) => prev ? {
        ...prev,
        aivis_user_dictionary_uuid: next.aivis_user_dictionary_uuid ?? null,
      } : prev);
    },
  });
}

export async function clearAivisUserDictionaryAction(args: {
  setSaving: (value: boolean) => void;
  showToast: (message: string, tone?: "success" | "error" | "info") => void;
  t: Translator;
  setDictionaryUUID: (value: string) => void;
  setSettings: Dispatch<SetStateAction<import("@/lib/api").UserSettings | null>>;
}) {
  const { setSaving, showToast, t, setDictionaryUUID, setSettings } = args;
  await runSavingAction({
    setSaving,
    showToast,
    successMessage: t("settings.toast.aivisDictionaryDeleted"),
    run: async () => {
      const next = await api.deleteAivisUserDictionary();
      setDictionaryUUID("");
      setSettings((prev) => prev ? {
        ...prev,
        aivis_user_dictionary_uuid: next.aivis_user_dictionary_uuid ?? null,
      } : prev);
    },
  });
}

export async function deleteInoreaderOAuthAction(args: {
  confirm: (options: {
    title: string;
    message: string;
    confirmLabel?: string;
    tone?: "danger" | "default";
  }) => Promise<boolean>;
  setSaving: (value: boolean) => void;
  showToast: (message: string, tone?: "success" | "error" | "info") => void;
  t: Translator;
  reload: () => Promise<unknown>;
}) {
  const { confirm, setSaving, showToast, t, reload } = args;
  await runConfirmedAction({
    confirm,
    confirmOptions: {
      title: t("settings.inoreaderDeleteTitle"),
      message: t("settings.inoreaderDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    },
    setSaving,
    showToast,
    successMessage: t("settings.toast.inoreaderDisconnected"),
    run: async () => {
      await api.deleteInoreaderOAuth();
      await reload();
    },
  });
}

export async function resetPreferenceProfileAction(args: {
  confirm: (options: {
    title: string;
    message: string;
    confirmLabel?: string;
    tone?: "danger" | "default";
  }) => Promise<boolean>;
  setSaving: (value: boolean) => void;
  showToast: (message: string, tone?: "success" | "error" | "info") => void;
  t: Translator;
  reload: () => Promise<unknown>;
}) {
  const { confirm, setSaving, showToast, t, reload } = args;
  await runConfirmedAction({
    confirm,
    confirmOptions: {
      title: t("settings.personalization.resetTitle"),
      message: t("settings.personalization.resetMessage"),
      confirmLabel: t("settings.personalization.reset"),
      tone: "danger",
    },
    setSaving,
    showToast,
    successMessage: t("settings.personalization.resetDone"),
    run: async () => {
      await api.resetPreferenceProfile();
      await reload();
    },
  });
}
