"use client";

import { useEffect, useState } from "react";
import { Bell } from "lucide-react";
import { NotificationPriorityRule } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";

interface OneSignalSettingsProps {
  rule?: NotificationPriorityRule | null;
  onSaveRule?: (rule: NotificationPriorityRule) => Promise<void>;
}

export default function OneSignalSettings({ rule, onSaveRule }: OneSignalSettingsProps) {
  const { t } = useI18n();
  const { showToast } = useToast();
  const [supported, setSupported] = useState(false);
  const [enabled, setEnabled] = useState(false);
  const [permission, setPermission] = useState<string>("default");
  const [busy, setBusy] = useState(false);
  const [ready, setReady] = useState(false);
  const [sensitivity, setSensitivity] = useState(rule?.sensitivity ?? "medium");
  const [dailyCap, setDailyCap] = useState(rule?.daily_cap ?? 3);
  const [themeWeight, setThemeWeight] = useState(rule?.theme_weight ?? 1);
  const [immediateEnabled, setImmediateEnabled] = useState(rule?.immediate_enabled ?? true);
  const [briefingEnabled, setBriefingEnabled] = useState(rule?.briefing_enabled ?? true);
  const [reviewEnabled, setReviewEnabled] = useState(rule?.review_enabled ?? true);
  const [goalMatchEnabled, setGoalMatchEnabled] = useState(rule?.goal_match_enabled ?? true);

  useEffect(() => {
    setSensitivity(rule?.sensitivity ?? "medium");
    setDailyCap(rule?.daily_cap ?? 3);
    setThemeWeight(rule?.theme_weight ?? 1);
    setImmediateEnabled(rule?.immediate_enabled ?? true);
    setBriefingEnabled(rule?.briefing_enabled ?? true);
    setReviewEnabled(rule?.review_enabled ?? true);
    setGoalMatchEnabled(rule?.goal_match_enabled ?? true);
  }, [rule?.briefing_enabled, rule?.daily_cap, rule?.goal_match_enabled, rule?.immediate_enabled, rule?.review_enabled, rule?.sensitivity, rule?.theme_weight]);

  useEffect(() => {
    if (typeof window === "undefined") return;
    const canPush = "Notification" in window && "serviceWorker" in navigator && "PushManager" in window;
    setSupported(canPush);
    setPermission(canPush ? Notification.permission : "default");
  }, []);

  useEffect(() => {
    if (!supported) return;
    const timer = window.setInterval(() => {
      const rawOs = window.OneSignal;
      const os =
        rawOs && !Array.isArray(rawOs) && typeof (rawOs as { init?: unknown }).init === "function"
          ? rawOs
          : undefined;
      const optedIn = Boolean(os?.User?.PushSubscription?.optedIn);
      setEnabled(optedIn);
      setReady(Boolean(window.__siftoOneSignalReady));
      if (typeof Notification !== "undefined") setPermission(Notification.permission);
    }, 1200);
    return () => window.clearInterval(timer);
  }, [supported]);

  if (!supported) {
    return (
      <section className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
        <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
          <Bell className="size-4 text-zinc-500" aria-hidden="true" />
          {t("settings.pushTitle")}
        </h2>
        <p className="mt-2 text-sm text-zinc-500">{t("settings.pushUnsupported")}</p>
      </section>
    );
  }

  return (
    <section className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
      <div className="mb-4">
        <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
          <Bell className="size-4 text-zinc-500" aria-hidden="true" />
          {t("settings.pushTitle")}
        </h2>
        <p className="mt-1 text-sm text-zinc-500">{t("settings.pushDescription")}</p>
      </div>

      <div className="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-700">
        {enabled ? t("settings.pushEnabled") : permission === "denied" ? t("settings.pushDenied") : t("settings.pushDisabled")}
      </div>

      <div className="mt-4 flex flex-wrap gap-2">
        <button
          type="button"
          disabled={busy || !ready}
          onClick={async () => {
            const rawOs = window.OneSignal;
            const os =
              rawOs && !Array.isArray(rawOs) && typeof (rawOs as { init?: unknown }).init === "function"
                ? rawOs
                : undefined;
            if (!os) return;
            setBusy(true);
            try {
              if (permission === "denied") {
                showToast(t("settings.pushDeniedHint"), "info");
                return;
              }
              await os.Notifications?.requestPermission?.();
              if (os.User?.PushSubscription?.optIn) {
                await os.User.PushSubscription.optIn();
              }
              setEnabled(Boolean(os?.User?.PushSubscription?.optedIn));
              showToast(t("settings.pushEnabledToast"), "success");
            } catch (e) {
              showToast(String(e), "error");
            } finally {
              if (typeof Notification !== "undefined") setPermission(Notification.permission);
              setBusy(false);
            }
          }}
          className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
        >
          {busy ? t("common.saving") : t("settings.pushEnable")}
        </button>
        <button
          type="button"
          disabled={busy || !ready}
          onClick={async () => {
            const rawOs = window.OneSignal;
            const os =
              rawOs && !Array.isArray(rawOs) && typeof (rawOs as { init?: unknown }).init === "function"
                ? rawOs
                : undefined;
            if (!os) return;
            setBusy(true);
            try {
              if (os.User?.PushSubscription?.optOut) {
                await os.User.PushSubscription.optOut();
              }
              setEnabled(false);
              showToast(t("settings.pushDisabledToast"), "success");
            } catch (e) {
              showToast(String(e), "error");
            } finally {
              setBusy(false);
            }
          }}
          className="rounded-lg border border-zinc-300 bg-white px-4 py-2 text-sm font-medium text-zinc-700 disabled:opacity-50"
        >
          {t("settings.pushDisable")}
        </button>
      </div>
      {onSaveRule ? (
        <div className="mt-5 rounded-lg border border-zinc-200 bg-zinc-50 p-4">
          <div className="grid gap-3 sm:grid-cols-3">
            <label className="text-sm text-zinc-700">
              <span className="mb-1 block">{t("settings.pushSensitivity")}</span>
              <select value={sensitivity} onChange={(e) => setSensitivity(e.target.value)} className="w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm">
                <option value="low">{t("settings.pushSensitivityLow")}</option>
                <option value="medium">{t("settings.pushSensitivityMedium")}</option>
                <option value="high">{t("settings.pushSensitivityHigh")}</option>
              </select>
            </label>
            <label className="text-sm text-zinc-700">
              <span className="mb-1 block">{t("settings.pushDailyCap")}</span>
              <input type="number" min={0} max={20} value={dailyCap} onChange={(e) => setDailyCap(Number(e.target.value))} className="w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm" />
            </label>
            <label className="text-sm text-zinc-700">
              <span className="mb-1 block">{t("settings.pushThemeWeight")}</span>
              <input type="number" min={0.5} max={2} step={0.1} value={themeWeight} onChange={(e) => setThemeWeight(Number(e.target.value))} className="w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm" />
            </label>
          </div>
          <div className="mt-4 grid gap-2 sm:grid-cols-2">
            <label className="flex items-center gap-2 rounded-lg border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-700">
              <input type="checkbox" checked={immediateEnabled} onChange={(e) => setImmediateEnabled(e.target.checked)} />
              <span>{t("settings.pushTypeImmediate")}</span>
            </label>
            <label className="flex items-center gap-2 rounded-lg border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-700">
              <input type="checkbox" checked={goalMatchEnabled} onChange={(e) => setGoalMatchEnabled(e.target.checked)} />
              <span>{t("settings.pushTypeGoalMatch")}</span>
            </label>
            <label className="flex items-center gap-2 rounded-lg border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-700">
              <input type="checkbox" checked={briefingEnabled} onChange={(e) => setBriefingEnabled(e.target.checked)} />
              <span>{t("settings.pushTypeBriefing")}</span>
            </label>
            <label className="flex items-center gap-2 rounded-lg border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-700">
              <input type="checkbox" checked={reviewEnabled} onChange={(e) => setReviewEnabled(e.target.checked)} />
              <span>{t("settings.pushTypeReview")}</span>
            </label>
          </div>
          <div className="mt-3">
            <button
              type="button"
              onClick={async () => {
                try {
                  await onSaveRule({
                    sensitivity,
                    daily_cap: dailyCap,
                    theme_weight: themeWeight,
                    immediate_enabled: immediateEnabled,
                    briefing_enabled: briefingEnabled,
                    review_enabled: reviewEnabled,
                    goal_match_enabled: goalMatchEnabled,
                  });
                  showToast(t("settings.pushRuleSaved"), "success");
                } catch (e) {
                  showToast(String(e), "error");
                }
              }}
              className="rounded-lg border border-zinc-300 bg-white px-4 py-2 text-sm font-medium text-zinc-700"
            >
              {t("settings.pushRuleSave")}
            </button>
          </div>
        </div>
      ) : null}
    </section>
  );
}
