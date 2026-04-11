"use client";

import OneSignalSettings from "@/components/onesignal-settings";
import { SectionCard } from "@/components/ui/section-card";
import type { NotificationPriorityRule } from "@/lib/api";

export default function NotificationsSettingsSection({
  rule,
  onSaveRule,
}: {
  rule: NotificationPriorityRule;
  onSaveRule: (rule: NotificationPriorityRule) => Promise<void>;
}) {
  return (
    <SectionCard>
      <OneSignalSettings rule={rule} onSaveRule={onSaveRule} />
    </SectionCard>
  );
}
