"use client";

import type { FormEvent } from "react";
import { KeyRound, Settings as SettingsIcon } from "lucide-react";
import { SectionCard } from "@/components/ui/section-card";

type Translate = (key: string, fallback?: string) => string;

export default function IntegrationsSettingsSection({
  t,
  state,
  actions,
}: {
  t: Translate;
  state: {
    hasInoreaderOAuth: boolean;
    inoreaderTokenExpiresAt: string | null | undefined;
    deletingInoreaderOAuth: boolean;
    obsidianEnabled: boolean;
    obsidianGithubConnected: boolean;
    obsidianRepoOwner: string;
    obsidianRepoName: string;
    obsidianRepoBranch: string;
    obsidianRootPath: string;
    obsidianLastSuccessAt: string | null | undefined;
    savingObsidianExport: boolean;
    runningObsidianExport: boolean;
  };
  actions: {
    onDeleteInoreaderOAuth: () => void;
    onSubmitObsidianExport: (event: FormEvent<HTMLFormElement>) => void;
    onChangeObsidianEnabled: (value: boolean) => void;
    onChangeObsidianRepoOwner: (value: string) => void;
    onChangeObsidianRepoName: (value: string) => void;
    onChangeObsidianRepoBranch: (value: string) => void;
    onChangeObsidianRootPath: (value: string) => void;
    onRunObsidianExportNow: () => void;
  };
}) {
  const {
    hasInoreaderOAuth,
    inoreaderTokenExpiresAt,
    deletingInoreaderOAuth,
    obsidianEnabled,
    obsidianGithubConnected,
    obsidianRepoOwner,
    obsidianRepoName,
    obsidianRepoBranch,
    obsidianRootPath,
    obsidianLastSuccessAt,
    savingObsidianExport,
    runningObsidianExport,
  } = state;
  const {
    onDeleteInoreaderOAuth,
    onSubmitObsidianExport,
    onChangeObsidianEnabled,
    onChangeObsidianRepoOwner,
    onChangeObsidianRepoName,
    onChangeObsidianRepoBranch,
    onChangeObsidianRootPath,
    onRunObsidianExportNow,
  } = actions;

  return (
    <div className="space-y-5">
      <SectionCard>
        <div className="mb-4">
          <h3 className="inline-flex items-center gap-2 text-base font-semibold text-[var(--color-editorial-ink)]">
            <KeyRound className="size-4 text-[var(--color-editorial-ink-faint)]" aria-hidden="true" />
            {t("settings.inoreaderTitle")}
          </h3>
          <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("settings.inoreaderDescription")}</p>
        </div>
        <div className="rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink-soft)]">
          {hasInoreaderOAuth ? t("settings.inoreaderConnected") : t("settings.inoreaderNotConnected")}
        </div>
        {inoreaderTokenExpiresAt ? (
          <p className="mt-2 break-words text-xs text-[var(--color-editorial-ink-faint)]">
            {t("settings.inoreaderTokenExpiresAt")}: {new Date(inoreaderTokenExpiresAt).toLocaleString()}
          </p>
        ) : null}
        <div className="mt-4 flex flex-col gap-2 sm:flex-row sm:flex-wrap">
          <a
            href="/api/settings/inoreader/connect"
            className="inline-flex min-h-10 w-full items-center justify-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] sm:w-auto"
          >
            {t("settings.inoreaderConnect")}
          </a>
          <button
            type="button"
            disabled={deletingInoreaderOAuth || !hasInoreaderOAuth}
            onClick={onDeleteInoreaderOAuth}
            className="min-h-10 w-full rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] disabled:opacity-50 sm:w-auto"
          >
            {deletingInoreaderOAuth ? t("settings.deleting") : t("settings.inoreaderDisconnect")}
          </button>
        </div>
      </SectionCard>

      <SectionCard>
        <form onSubmit={onSubmitObsidianExport} className="space-y-4">
          <div className="mb-4">
            <h3 className="inline-flex items-center gap-2 text-base font-semibold text-[var(--color-editorial-ink)]">
              <SettingsIcon className="size-4 text-[var(--color-editorial-ink-faint)]" aria-hidden="true" />
              {t("settings.obsidianTitle")}
            </h3>
            <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("settings.obsidianDescription")}</p>
          </div>

          <div className="flex items-center justify-between gap-3 rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3">
            <div className="min-w-0">
              <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.obsidianEnabled")}</div>
              <div className="mt-1 text-[12px] leading-5 text-[var(--color-editorial-ink-soft)]">{t("settings.obsidianEnabledHint")}</div>
            </div>
            <label className="inline-flex shrink-0 items-center gap-2 text-sm text-[var(--color-editorial-ink-soft)]">
              <input
                type="checkbox"
                checked={obsidianEnabled}
                onChange={(e) => onChangeObsidianEnabled(e.target.checked)}
                className="size-4 rounded border-[var(--color-editorial-line-strong)]"
              />
              {obsidianEnabled ? t("settings.on") : t("settings.off")}
            </label>
          </div>

          <div className="rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink-soft)]">
            {obsidianGithubConnected ? t("settings.obsidianGithubConnected") : t("settings.obsidianGithubNotConnected")}
          </div>
          <div className="flex flex-wrap gap-2">
            <a
              href="/api/settings/obsidian-github/connect"
              className="inline-flex min-h-10 items-center justify-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)]"
            >
              {t("settings.obsidianGithubConnect")}
            </a>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <div>
              <label className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("settings.obsidianRepoOwner")}</label>
              <input
                type="text"
                value={obsidianRepoOwner}
                onChange={(e) => onChangeObsidianRepoOwner(e.target.value)}
                className="mt-1 w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)]"
                placeholder="your-org"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("settings.obsidianRepoName")}</label>
              <input
                type="text"
                value={obsidianRepoName}
                onChange={(e) => onChangeObsidianRepoName(e.target.value)}
                className="mt-1 w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)]"
                placeholder="obsidian-vault"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("settings.obsidianBranch")}</label>
              <input
                type="text"
                value={obsidianRepoBranch}
                onChange={(e) => onChangeObsidianRepoBranch(e.target.value)}
                className="mt-1 w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)]"
                placeholder="main"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("settings.obsidianRootPath")}</label>
              <input
                type="text"
                value={obsidianRootPath}
                onChange={(e) => onChangeObsidianRootPath(e.target.value)}
                className="mt-1 w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)]"
                placeholder="Sifto/Favorites"
              />
              <p className="mt-1 text-xs text-[var(--color-editorial-ink-faint)]">{t("settings.obsidianRootPathHint")}</p>
            </div>
          </div>

          {obsidianLastSuccessAt ? (
            <p className="text-xs text-[var(--color-editorial-ink-faint)]">
              {t("settings.obsidianLastSuccess")}: {new Date(obsidianLastSuccessAt).toLocaleString()}
            </p>
          ) : null}

          <div className="flex flex-wrap gap-2">
            <button
              type="submit"
              disabled={savingObsidianExport}
              className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:opacity-60"
            >
              {savingObsidianExport ? t("common.saving") : t("settings.obsidianSave")}
            </button>
            <button
              type="button"
              onClick={onRunObsidianExportNow}
              disabled={runningObsidianExport}
              className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] disabled:opacity-60"
            >
              {runningObsidianExport ? t("settings.obsidianRunNowRunning") : t("settings.obsidianRunNow")}
            </button>
          </div>
        </form>
      </SectionCard>
    </div>
  );
}
