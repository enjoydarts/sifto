"use client";

import {
  SettingsRailNote,
  SettingsSectionID,
  SettingsSectionMeta,
  SettingsSectionNavItem,
} from "@/components/settings/settings-page-shell";

type Translator = (key: string, fallback?: string) => string;

type BuildSettingsSectionNavItemsArgs = {
  t: Translator;
  configuredProviderCount: number;
  accessCardCount: number;
  readingPlanWindow: string;
  readingPlanSize: string;
  readingPlanDiversifyTopics: boolean;
  navigatorEnabled: boolean;
  navigatorPersonaMode: "fixed" | "random";
  navigatorPersona: string;
  navigatorModel: string;
  audioBriefingEnabled: boolean;
  audioBriefingScheduleSummary: string;
  audioBriefingArticlesPerEpisode: string;
  summaryAudioProvider: string;
  summaryAudioVoiceModel: string;
  preferenceProfileStatus: string | null;
  preferenceProfileConfidence: number | null;
  preferenceProfileError: boolean;
  digestEmailEnabled: boolean;
  notificationBriefingEnabled: boolean;
  notificationDailyCap: number;
  hasInoreaderOAuth?: boolean;
  hasObsidianGithubInstallation: boolean;
  monthlyBudgetUSD: number | null;
  remainingBudgetPct: number | null;
};

type BuildSettingsRailNotesArgs = {
  t: Translator;
  providerModelUpdateCount: number;
  notificationBriefingEnabled: boolean;
  notificationImmediateEnabled: boolean;
  notificationDailyCap: number;
  currentMonthJST: string;
  remainingBudgetPct: number | null;
};

export function buildSettingsSectionNavItems({
  t,
  configuredProviderCount,
  accessCardCount,
  readingPlanWindow,
  readingPlanSize,
  readingPlanDiversifyTopics,
  navigatorEnabled,
  navigatorPersonaMode,
  navigatorPersona,
  navigatorModel,
  audioBriefingEnabled,
  audioBriefingScheduleSummary,
  audioBriefingArticlesPerEpisode,
  summaryAudioProvider,
  summaryAudioVoiceModel,
  preferenceProfileStatus,
  preferenceProfileConfidence,
  preferenceProfileError,
  digestEmailEnabled,
  notificationBriefingEnabled,
  notificationDailyCap,
  hasInoreaderOAuth,
  hasObsidianGithubInstallation,
  monthlyBudgetUSD,
  remainingBudgetPct,
}: BuildSettingsSectionNavItemsArgs): SettingsSectionNavItem[] {
  return [
    {
      id: "models",
      title: t("settings.section.llm"),
      summary: `${configuredProviderCount}/${accessCardCount} ${t("settings.access.configuredProviders")}`,
    },
    {
      id: "reading-plan",
      title: t("settings.recommendedTitle"),
      summary: `${t(`settings.window.${readingPlanWindow}`)} / ${readingPlanSize} / ${readingPlanDiversifyTopics ? t("settings.on") : t("settings.off")}`,
    },
    {
      id: "navigator",
      title: t("settings.group.navigator"),
      summary: navigatorEnabled
        ? `${navigatorPersonaMode === "random" ? t("settings.personaMode.random") : t(`settings.navigator.persona.${navigatorPersona}`, navigatorPersona)} / ${navigatorModel || t("settings.default")}`
        : t("settings.off"),
    },
    {
      id: "audio-briefing",
      title: t("settings.section.audioBriefing"),
      summary: audioBriefingEnabled
        ? `${audioBriefingScheduleSummary} / ${audioBriefingArticlesPerEpisode}${t("settings.audioBriefing.articlesSuffix")}`
        : t("settings.off"),
    },
    {
      id: "summary-audio",
      title: t("settings.section.summaryAudio"),
      summary: summaryAudioProvider
        ? `${t(`settings.summaryAudio.provider.${summaryAudioProvider}`, summaryAudioProvider)} / ${summaryAudioVoiceModel || t("settings.summaryAudio.unconfiguredShort")}`
        : t("settings.off"),
    },
    {
      id: "personalization",
      title: t("settings.personalization.title"),
      summary: preferenceProfileStatus != null && preferenceProfileConfidence != null
        ? `${t(`settings.personalization.status.${preferenceProfileStatus}`, preferenceProfileStatus)} / ${Math.round(preferenceProfileConfidence * 100)}%`
        : preferenceProfileError
          ? t("settings.personalization.loadFailedShort")
          : t("settings.personalization.unavailable"),
    },
    {
      id: "digest",
      title: t("settings.digestTitle"),
      summary: digestEmailEnabled ? t("settings.controlRoom.digestEnabled") : t("settings.controlRoom.digestDisabled"),
    },
    {
      id: "notifications",
      title: t("settings.section.notifications"),
      summary: `${notificationBriefingEnabled ? t("settings.pushTypeBriefing") : t("settings.controlRoom.briefingOff")} / cap ${notificationDailyCap}`,
    },
    {
      id: "integrations",
      title: t("settings.section.integrations"),
      summary: `${hasInoreaderOAuth ? t("settings.inoreaderConnected") : t("settings.inoreaderNotConnected")} / ${hasObsidianGithubInstallation ? t("settings.obsidianGithubConnected") : t("settings.obsidianGithubNotConnected")}`,
    },
    {
      id: "budget",
      title: t("settings.budgetTitle"),
      summary: monthlyBudgetUSD == null
        ? t("settings.controlRoom.budgetUnset")
        : `$${monthlyBudgetUSD.toFixed(2)} / ${remainingBudgetPct == null ? "—" : `${remainingBudgetPct.toFixed(1)}%`}`,
    },
    {
      id: "system",
      title: t("settings.section.system"),
      summary: `${configuredProviderCount}/${accessCardCount} ${t("settings.configured")}`,
    },
  ];
}

export function buildSettingsRailNotes({
  t,
  providerModelUpdateCount,
  notificationBriefingEnabled,
  notificationImmediateEnabled,
  notificationDailyCap,
  currentMonthJST,
  remainingBudgetPct,
}: BuildSettingsRailNotesArgs): SettingsRailNote[] {
  return [
    {
      title: t("settings.controlRoom.providerUpdatesTitle"),
      body: providerModelUpdateCount > 0
        ? t("settings.controlRoom.providerUpdatesBody").replace("{{count}}", String(providerModelUpdateCount))
        : t("settings.controlRoom.providerUpdatesEmpty"),
    },
    {
      title: t("settings.controlRoom.notificationHealthTitle"),
      body: `${notificationBriefingEnabled ? t("settings.pushTypeBriefing") : t("settings.controlRoom.briefingOff")} / ${notificationImmediateEnabled ? t("settings.pushTypeImmediate") : t("settings.controlRoom.immediateOff")} / cap ${notificationDailyCap}`,
    },
    {
      title: t("settings.controlRoom.budgetStatusTitle"),
      body: remainingBudgetPct == null
        ? t("settings.controlRoom.budgetUnset")
        : t("settings.controlRoom.budgetStatusBody")
            .replace("{{month}}", currentMonthJST)
            .replace("{{remaining}}", `${remainingBudgetPct.toFixed(1)}%`),
    },
  ];
}

export function buildSettingsSectionMeta(
  activeSection: SettingsSectionID,
  t: Translator,
): SettingsSectionMeta {
  return {
    "audio-briefing": {
      kicker: t("settings.section.audioBriefing"),
      title: t("settings.controlRoom.audioBriefingTitle"),
      description: t("settings.controlRoom.audioBriefingDescription"),
    },
    "summary-audio": {
      kicker: t("settings.section.summaryAudio"),
      title: t("settings.summaryAudio.title"),
      description: t("settings.summaryAudio.description"),
    },
    "reading-plan": {
      kicker: t("settings.recommendedTitle"),
      title: t("settings.controlRoom.readingPlanTitle"),
      description: t("settings.controlRoom.readingPlanDescription"),
    },
    personalization: {
      kicker: t("settings.personalization.title"),
      title: t("settings.personalization.title"),
      description: t("settings.personalization.description.default"),
    },
    digest: {
      kicker: t("settings.digestTitle"),
      title: t("settings.controlRoom.digestTitle"),
      description: t("settings.controlRoom.digestDescription"),
    },
    notifications: {
      kicker: t("settings.section.notifications"),
      title: t("settings.controlRoom.notificationsTitle"),
      description: t("settings.controlRoom.notificationsDescription"),
    },
    integrations: {
      kicker: t("settings.section.integrations"),
      title: t("settings.controlRoom.integrationsTitle"),
      description: t("settings.controlRoom.integrationsDescription"),
    },
    navigator: {
      kicker: t("settings.group.navigator"),
      title: t("settings.controlRoom.navigatorTitle"),
      description: t("settings.controlRoom.navigatorDescription"),
    },
    models: {
      kicker: t("settings.section.llm"),
      title: t("settings.controlRoom.modelsTitle"),
      description: t("settings.controlRoom.modelsDescription"),
    },
    budget: {
      kicker: t("settings.budgetTitle"),
      title: t("settings.controlRoom.budgetTitle"),
      description: t("settings.controlRoom.budgetDescription"),
    },
    system: {
      kicker: t("settings.section.system"),
      title: t("settings.controlRoom.systemTitle"),
      description: t("settings.controlRoom.systemDescription"),
    },
  }[activeSection];
}
