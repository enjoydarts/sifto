"use client";

import { FormEvent, useCallback, useEffect, useState } from "react";
import { Target } from "lucide-react";
import { api, ReadingGoal } from "@/lib/api";
import { PageTransition } from "@/components/page-transition";
import { PageHeader } from "@/components/ui/page-header";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";

export default function GoalsPage() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const [loading, setLoading] = useState(true);
  const [savingReadingGoal, setSavingReadingGoal] = useState(false);
  const [activeReadingGoals, setActiveReadingGoals] = useState<ReadingGoal[]>([]);
  const [archivedReadingGoals, setArchivedReadingGoals] = useState<ReadingGoal[]>([]);
  const [editingReadingGoalId, setEditingReadingGoalId] = useState<string | null>(null);
  const [readingGoalTitle, setReadingGoalTitle] = useState("");
  const [readingGoalDescription, setReadingGoalDescription] = useState("");
  const [readingGoalPriority, setReadingGoalPriority] = useState("3");
  const [readingGoalDueDate, setReadingGoalDueDate] = useState("");
  const [modalOpen, setModalOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const readingGoals = await api.getReadingGoals();
      setActiveReadingGoals(readingGoals.active ?? []);
      setArchivedReadingGoals(readingGoals.archived ?? []);
      setError(null);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  function resetReadingGoalForm() {
    setEditingReadingGoalId(null);
    setReadingGoalTitle("");
    setReadingGoalDescription("");
    setReadingGoalPriority("3");
    setReadingGoalDueDate("");
    setModalOpen(false);
  }

  async function submitReadingGoal(e: FormEvent) {
    e.preventDefault();
    setSavingReadingGoal(true);
    try {
      const activeLimitReached = activeReadingGoals.length >= 7 && !editingReadingGoalId;
      if (activeLimitReached) {
        throw new Error(t("settings.readingGoals.limit"));
      }
      const payload = {
        title: readingGoalTitle,
        description: readingGoalDescription,
        priority: Number(readingGoalPriority),
        due_date: readingGoalDueDate.trim() || null,
      };
      if (editingReadingGoalId) {
        await api.updateReadingGoal(editingReadingGoalId, payload);
      } else {
        await api.createReadingGoal(payload);
      }
      await load();
      resetReadingGoalForm();
      showToast(t("settings.toast.readingGoalSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingReadingGoal(false);
    }
  }

  async function archiveReadingGoal(goalId: string) {
    try {
      await api.archiveReadingGoal(goalId);
      await load();
      if (editingReadingGoalId === goalId) resetReadingGoalForm();
      showToast(t("settings.toast.readingGoalArchived"), "success");
    } catch (e) {
      showToast(String(e), "error");
    }
  }

  async function restoreReadingGoal(goalId: string) {
    try {
      await api.restoreReadingGoal(goalId);
      await load();
      showToast(t("settings.toast.readingGoalRestored"), "success");
    } catch (e) {
      showToast(String(e), "error");
    }
  }

  async function deleteReadingGoal(goalId: string) {
    try {
      await api.deleteReadingGoal(goalId);
      await load();
      if (editingReadingGoalId === goalId) resetReadingGoalForm();
      showToast(t("settings.toast.readingGoalDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    }
  }

  function startEditReadingGoal(goal: ReadingGoal) {
    setEditingReadingGoalId(goal.id);
    setReadingGoalTitle(goal.title);
    setReadingGoalDescription(goal.description ?? "");
    setReadingGoalPriority(String(goal.priority));
    setReadingGoalDueDate(goal.due_date ?? "");
    setModalOpen(true);
  }

  function startCreateReadingGoal() {
    setEditingReadingGoalId(null);
    setReadingGoalTitle("");
    setReadingGoalDescription("");
    setReadingGoalPriority("3");
    setReadingGoalDueDate("");
    setModalOpen(true);
  }

  return (
    <PageTransition>
      <div className="space-y-6">
        <PageHeader
          title={t("settings.readingGoals.title")}
          titleIcon={Target}
          description={t("settings.readingGoals.description")}
          actions={
            <div className="flex flex-wrap items-center gap-2">
              <div className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-1.5 text-xs font-medium text-[var(--color-editorial-ink-soft)]">
                {locale === "ja" ? `active ${activeReadingGoals.length}/7` : `${activeReadingGoals.length}/7 active`}
              </div>
              <button
                type="button"
                onClick={startCreateReadingGoal}
                className="inline-flex min-h-11 items-center justify-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-95"
              >
                {t("goals.create")}
              </button>
            </div>
          }
        />

        {error ? (
          <section className="rounded-[24px] border border-rose-200 bg-rose-50 p-5 text-sm text-rose-700 shadow-[var(--shadow-card)]">
            {error}
          </section>
        ) : null}

        <div className="space-y-6">
          <section className="surface-editorial overflow-hidden rounded-[28px] shadow-[var(--shadow-card)]">
            <div className="border-b border-[var(--color-editorial-line)] bg-[rgba(250,246,238,0.95)] px-5 py-4">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                {t("goals.listTitle")}
              </div>
              <div className="mt-3 hidden grid-cols-[minmax(0,1.7fr)_96px_120px_220px] gap-4 text-xs font-semibold uppercase tracking-[0.12em] text-[var(--color-editorial-ink-faint)] md:grid">
                <div>{t("goals.columns.goal")}</div>
                <div>{t("goals.columns.priority")}</div>
                <div>{t("goals.columns.dueDate")}</div>
                <div>{t("goals.columns.actions")}</div>
              </div>
            </div>
            {loading ? (
              <div className="px-5 py-5 text-sm text-[var(--color-editorial-ink-faint)]">{t("common.loading")}</div>
            ) : (
              <div>
                {activeReadingGoals.map((goal) => (
                  <div key={goal.id} className="border-b border-[#ece4d6] bg-[rgba(255,255,255,0.55)] px-5 py-4 last:border-b-0">
                    <div className="grid gap-3 md:grid-cols-[minmax(0,1.7fr)_96px_120px_220px] md:items-center md:gap-4">
                      <div className="min-w-0">
                        <div className="line-clamp-1 text-base font-semibold text-[var(--color-editorial-ink)]">{goal.title}</div>
                        {goal.description ? (
                          <p className="mt-2 line-clamp-2 text-[13px] leading-7 text-[var(--color-editorial-ink-soft)]">{goal.description}</p>
                        ) : null}
                      </div>
                      <div className="font-serif text-[1.45rem] leading-none text-[var(--color-editorial-ink)]">P{goal.priority}</div>
                      <div className="text-sm text-[var(--color-editorial-ink-soft)]">{goal.due_date ?? "—"}</div>
                      <div className="flex flex-wrap gap-3 text-[13px]">
                        <button type="button" onClick={() => startEditReadingGoal(goal)} className="text-[var(--color-editorial-ink-soft)] hover:text-[var(--color-editorial-ink)]">
                          {t("settings.readingGoals.edit")}
                        </button>
                        <button type="button" onClick={() => void archiveReadingGoal(goal.id)} className="text-[var(--color-editorial-ink-soft)] hover:text-[var(--color-editorial-ink)]">
                          {t("settings.readingGoals.archive")}
                        </button>
                        <button type="button" onClick={() => void deleteReadingGoal(goal.id)} className="text-rose-700 hover:text-rose-800">
                          {t("settings.delete")}
                        </button>
                      </div>
                    </div>
                  </div>
                ))}
                {!loading && activeReadingGoals.length === 0 ? (
                  <div className="px-5 py-8 text-sm text-[var(--color-editorial-ink-faint)]">
                    {t("goals.empty")}
                  </div>
                ) : null}
                {archivedReadingGoals.length > 0 ? (
                  <div className="border-t border-[var(--color-editorial-line)] bg-[rgba(250,246,238,0.72)] px-5 py-4">
                    <div className="mb-3 text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("settings.readingGoals.archived")}</div>
                    {archivedReadingGoals.map((goal) => (
                      <div key={goal.id} className="flex items-center justify-between gap-3 border-b border-[var(--color-editorial-line)] py-2.5 last:border-b-0">
                        <div className="min-w-0 text-sm text-[var(--color-editorial-ink-soft)]">{goal.title}</div>
                        <button type="button" onClick={() => void restoreReadingGoal(goal.id)} className="text-xs text-[var(--color-editorial-ink)] hover:underline">
                          {t("settings.readingGoals.restore")}
                        </button>
                      </div>
                    ))}
                  </div>
                ) : null}
              </div>
            )}
          </section>

        </div>

        {modalOpen ? (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-[rgba(31,26,23,0.4)] px-4">
            <form onSubmit={submitReadingGoal} className="w-full max-w-2xl rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-5 shadow-[var(--shadow-card)]">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                    Goal Modal
                  </div>
                  <h2 className="mt-2 font-serif text-[1.9rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                    {editingReadingGoalId ? t("goals.modal.editTitle") : t("goals.modal.createTitle")}
                  </h2>
                  <p className="mt-3 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{t("settings.readingGoals.description")}</p>
                </div>
                <button type="button" onClick={resetReadingGoalForm} className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]">
                  {t("common.close")}
                </button>
              </div>
              <div className="mt-4 space-y-4">
                <div>
                  <label className="block text-sm font-medium text-[var(--color-editorial-ink-soft)]">{t("settings.readingGoals.goalTitle")}</label>
                  <input
                    value={readingGoalTitle}
                    onChange={(e) => setReadingGoalTitle(e.target.value)}
                    className="mt-2 w-full rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3.5 py-3 text-sm text-[var(--color-editorial-ink)] outline-none"
                    placeholder={t("settings.readingGoals.goalTitlePlaceholder")}
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-[var(--color-editorial-ink-soft)]">{t("settings.readingGoals.goalDescription")}</label>
                  <textarea
                    value={readingGoalDescription}
                    onChange={(e) => setReadingGoalDescription(e.target.value)}
                    rows={4}
                    className="mt-2 w-full rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3.5 py-3 text-sm text-[var(--color-editorial-ink)] outline-none"
                    placeholder={t("settings.readingGoals.goalDescriptionPlaceholder")}
                  />
                </div>
                <div className="grid gap-3 sm:grid-cols-2">
                  <div>
                    <label className="block text-sm font-medium text-[var(--color-editorial-ink-soft)]">{t("settings.readingGoals.priority")}</label>
                    <select
                      value={readingGoalPriority}
                      onChange={(e) => setReadingGoalPriority(e.target.value)}
                      className="mt-2 w-full rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3.5 py-3 text-sm text-[var(--color-editorial-ink)] outline-none"
                    >
                      {[5, 4, 3, 2, 1].map((value) => (
                        <option key={value} value={String(value)}>
                          {value}
                        </option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-[var(--color-editorial-ink-soft)]">{t("settings.readingGoals.dueDate")}</label>
                    <input
                      type="date"
                      value={readingGoalDueDate}
                      onChange={(e) => setReadingGoalDueDate(e.target.value)}
                      className="mt-2 w-full rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3.5 py-3 text-sm text-[var(--color-editorial-ink)] outline-none"
                    />
                  </div>
                </div>
              </div>
              <div className="mt-5 flex justify-end gap-2">
                <button type="button" onClick={resetReadingGoalForm} className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm text-[var(--color-editorial-ink-soft)]">
                  {t("common.cancel")}
                </button>
                <button
                  type="submit"
                  disabled={savingReadingGoal}
                  className="rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:opacity-60"
                >
                  {savingReadingGoal ? t("common.saving") : editingReadingGoalId ? t("settings.readingGoals.update") : t("goals.create")}
                </button>
              </div>
            </form>
          </div>
        ) : null}
      </div>
    </PageTransition>
  );
}
