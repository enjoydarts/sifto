"use client";

import { ReactNode } from "react";
import { Settings as SettingsIcon } from "lucide-react";
import { PageHeader } from "@/components/ui/page-header";
import { SectionCard } from "@/components/ui/section-card";

export type SettingsSectionID =
  | "audio-briefing"
  | "summary-audio"
  | "reading-plan"
  | "personalization"
  | "digest"
  | "notifications"
  | "integrations"
  | "models"
  | "navigator"
  | "budget"
  | "system";

export type SettingsSectionNavItem = {
  id: SettingsSectionID;
  title: string;
  summary: string;
};

export type SettingsRailNote = {
  title: string;
  body: string;
};

export type SettingsSectionMeta = {
  kicker: string;
  title: string;
  description: string;
};

type SettingsPageShellProps = {
  t: (key: string, fallback?: string) => string;
  activeSection: SettingsSectionID;
  sectionNavItems: SettingsSectionNavItem[];
  railNotes: SettingsRailNote[];
  selectedSectionMeta: SettingsSectionMeta;
  onSelectSection: (section: SettingsSectionID) => void;
  heroActions?: ReactNode;
  children: ReactNode;
};

function joinClassNames(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

export default function SettingsPageShell({
  t,
  activeSection,
  sectionNavItems,
  railNotes,
  selectedSectionMeta,
  onSelectSection,
  heroActions,
  children,
}: SettingsPageShellProps) {
  return (
    <div className="mx-auto max-w-[1360px] space-y-6">
      <PageHeader
        eyebrow={t("settings.controlRoomEyebrow")}
        title={t("nav.settings")}
        titleIcon={SettingsIcon}
        description={t("settings.controlRoomSubtitle")}
      />

      <div className="grid gap-6 lg:grid-cols-[248px_minmax(0,1fr)] xl:grid-cols-[268px_minmax(0,1fr)]">
        <aside className="space-y-4 lg:sticky lg:top-24 lg:self-start">
          <SectionCard className="p-0">
            <div className="px-5 pt-5 text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              {t("settings.controlRoomSections")}
            </div>
            <div className="mt-3">
              {sectionNavItems.map((item, index) => {
                const active = item.id === activeSection;
                return (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => onSelectSection(item.id)}
                    className={joinClassNames(
                      "relative block w-full border-t border-[var(--color-editorial-line)] px-4 py-3 text-left transition-colors first:border-t-0",
                      active
                        ? "bg-[linear-gradient(90deg,rgba(243,236,227,0.92),rgba(243,236,227,0.28)_78%,transparent)]"
                        : "hover:bg-[var(--color-editorial-panel-strong)]"
                    )}
                  >
                    {active ? (
                      <span
                        aria-hidden="true"
                        className={joinClassNames(
                          "absolute left-0 w-[3px] rounded-full bg-[var(--color-editorial-ink)]",
                          index === 0 ? "top-0 bottom-3" : "bottom-3 top-3"
                        )}
                      />
                    ) : null}
                    <div className="text-[13px] font-semibold text-[var(--color-editorial-ink)]">{item.title}</div>
                    <div className="mt-1 text-[12px] leading-5 text-[var(--color-editorial-ink-soft)]">{item.summary}</div>
                  </button>
                );
              })}
            </div>
          </SectionCard>

          <SectionCard className="p-0">
            <div className="px-5 pt-5 text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              {t("settings.controlRoomStatusNotes")}
            </div>
            <div className="mt-3">
              {railNotes.map((note) => (
                <div key={note.title} className="border-t border-[var(--color-editorial-line)] px-4 py-3 first:border-t-0">
                  <div className="text-[13px] font-semibold text-[var(--color-editorial-ink)]">{note.title}</div>
                  <div className="mt-1 text-[12px] leading-5 text-[var(--color-editorial-ink-soft)]">{note.body}</div>
                </div>
              ))}
            </div>
          </SectionCard>
        </aside>

        <div className="space-y-5">
          <SectionCard>
            <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
              <div>
                <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                  {selectedSectionMeta.kicker}
                </div>
                <h2 className="mt-2 font-serif text-[1.85rem] leading-[1.1] tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                  {selectedSectionMeta.title}
                </h2>
                <p className="mt-2 max-w-3xl text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">
                  {selectedSectionMeta.description}
                </p>
              </div>
              {heroActions}
            </div>
          </SectionCard>

          {children}
        </div>
      </div>
    </div>
  );
}
