"use client";

import { PreferenceProfilePanel } from "@/components/settings/preference-profile-panel";
import { SectionCard } from "@/components/ui/section-card";
import type { PreferenceProfile } from "@/lib/api";

export default function PersonalizationSettingsSection({
  profile,
  error,
  resetting,
  actions,
}: {
  profile: PreferenceProfile | null;
  error: string | null;
  resetting: boolean;
  actions: {
    onReset: () => void;
    onRetry: () => void;
  };
}) {
  return (
    <SectionCard>
      <PreferenceProfilePanel
        profile={profile}
        error={error}
        onReset={actions.onReset}
        onRetry={actions.onRetry}
        resetting={resetting}
      />
    </SectionCard>
  );
}
