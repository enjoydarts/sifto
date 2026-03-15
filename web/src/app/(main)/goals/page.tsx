"use client";

import { FormEvent, useCallback, useEffect, useState } from "react";
import { Brain } from "lucide-react";
import { api, ReadingGoal } from "@/lib/api";
import { PageTransition } from "@/components/page-transition";
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
        <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div>
              <h1 className="inline-flex items-center gap-2 text-2xl font-bold text-zinc-950">
                <Brain className="size-5 text-zinc-500" aria-hidden="true" />
                {t("settings.readingGoals.title")}
              </h1>
              <p className="mt-2 text-sm text-zinc-500">{t("settings.readingGoals.description")}</p>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <div className="rounded-full bg-zinc-100 px-3 py-1 text-xs font-medium text-zinc-700">
                {locale === "ja" ? `active ${activeReadingGoals.length}/7` : `${activeReadingGoals.length}/7 active`}
              </div>
              <button
                type="button"
                onClick={startCreateReadingGoal}
                className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white"
              >
                {t("goals.create")}
              </button>
            </div>
          </div>
        </section>

        {error ? (
          <section className="rounded-2xl border border-rose-200 bg-rose-50 p-5 text-sm text-rose-700">
            {error}
          </section>
        ) : null}

        <div className="space-y-6">
          <section className="overflow-hidden rounded-2xl border border-zinc-200 bg-white shadow-sm">
            <div className="border-b border-zinc-200 bg-zinc-50 px-5 py-3">
              <div className="hidden grid-cols-[minmax(0,1.7fr)_96px_120px_200px] gap-4 text-xs font-medium uppercase tracking-[0.12em] text-zinc-500 md:grid">
                <div>{t("goals.columns.goal")}</div>
                <div>{t("goals.columns.priority")}</div>
                <div>{t("goals.columns.dueDate")}</div>
                <div>{t("goals.columns.actions")}</div>
              </div>
              <div className="md:hidden text-xs font-medium uppercase tracking-[0.12em] text-zinc-500">{t("goals.listTitle")}</div>
            </div>
            {loading ? (
              <div className="px-5 py-5 text-sm text-zinc-500">{t("common.loading")}</div>
            ) : (
              <div>
                {activeReadingGoals.map((goal) => (
                  <div key={goal.id} className="border-b border-zinc-100 px-5 py-4 last:border-b-0">
                    <div className="grid gap-3 md:grid-cols-[minmax(0,1.7fr)_96px_120px_200px] md:items-center md:gap-4">
                      <div className="min-w-0">
                        <div className="line-clamp-1 text-sm font-semibold text-zinc-900">{goal.title}</div>
                        {goal.description ? (
                          <p className="mt-1 line-clamp-2 text-xs text-zinc-500">{goal.description}</p>
                        ) : null}
                      </div>
                      <div className="text-sm text-zinc-700">P{goal.priority}</div>
                      <div className="text-sm text-zinc-600">{goal.due_date ?? "—"}</div>
                      <div className="flex flex-wrap gap-3 text-xs">
                        <button type="button" onClick={() => startEditReadingGoal(goal)} className="text-zinc-600 hover:text-zinc-900">
                          {t("settings.readingGoals.edit")}
                        </button>
                        <button type="button" onClick={() => void archiveReadingGoal(goal.id)} className="text-zinc-600 hover:text-zinc-900">
                          {t("settings.readingGoals.archive")}
                        </button>
                        <button type="button" onClick={() => void deleteReadingGoal(goal.id)} className="text-rose-600 hover:text-rose-700">
                          {t("settings.delete")}
                        </button>
                      </div>
                    </div>
                  </div>
                ))}
                {!loading && activeReadingGoals.length === 0 ? (
                  <div className="px-5 py-8 text-sm text-zinc-500">
                    {t("goals.empty")}
                  </div>
                ) : null}
                {archivedReadingGoals.length > 0 ? (
                  <div className="border-t border-zinc-200 bg-zinc-50 px-5 py-4">
                    <div className="mb-3 text-xs font-medium uppercase tracking-[0.12em] text-zinc-500">{t("settings.readingGoals.archived")}</div>
                    {archivedReadingGoals.map((goal) => (
                      <div key={goal.id} className="flex items-center justify-between gap-3 border-b border-zinc-200 py-2 last:border-b-0">
                        <div className="min-w-0 text-sm text-zinc-700">{goal.title}</div>
                        <button type="button" onClick={() => void restoreReadingGoal(goal.id)} className="text-xs text-zinc-600 hover:text-zinc-900">
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
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/40 px-4">
            <form onSubmit={submitReadingGoal} className="w-full max-w-xl rounded-3xl border border-zinc-200 bg-white p-5 shadow-xl">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <h2 className="text-lg font-semibold text-zinc-950">
                    {editingReadingGoalId ? t("goals.modal.editTitle") : t("goals.modal.createTitle")}
                  </h2>
                  <p className="mt-1 text-sm text-zinc-500">{t("settings.readingGoals.description")}</p>
                </div>
                <button type="button" onClick={resetReadingGoalForm} className="text-sm text-zinc-500 hover:text-zinc-800">
                  {t("common.close")}
                </button>
              </div>
              <div className="mt-4 space-y-4">
                <div>
                  <label className="block text-sm font-medium text-zinc-700">{t("settings.readingGoals.goalTitle")}</label>
                  <input
                    value={readingGoalTitle}
                    onChange={(e) => setReadingGoalTitle(e.target.value)}
                    className="mt-1 w-full rounded-xl border border-zinc-200 px-3 py-2 text-sm text-zinc-900"
                    placeholder={t("settings.readingGoals.goalTitlePlaceholder")}
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-zinc-700">{t("settings.readingGoals.goalDescription")}</label>
                  <textarea
                    value={readingGoalDescription}
                    onChange={(e) => setReadingGoalDescription(e.target.value)}
                    rows={4}
                    className="mt-1 w-full rounded-xl border border-zinc-200 px-3 py-2 text-sm text-zinc-900"
                    placeholder={t("settings.readingGoals.goalDescriptionPlaceholder")}
                  />
                </div>
                <div className="grid gap-3 sm:grid-cols-2">
                  <div>
                    <label className="block text-sm font-medium text-zinc-700">{t("settings.readingGoals.priority")}</label>
                    <select
                      value={readingGoalPriority}
                      onChange={(e) => setReadingGoalPriority(e.target.value)}
                      className="mt-1 w-full rounded-xl border border-zinc-200 px-3 py-2 text-sm text-zinc-900"
                    >
                      {[5, 4, 3, 2, 1].map((value) => (
                        <option key={value} value={String(value)}>
                          {value}
                        </option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-zinc-700">{t("settings.readingGoals.dueDate")}</label>
                    <input
                      type="date"
                      value={readingGoalDueDate}
                      onChange={(e) => setReadingGoalDueDate(e.target.value)}
                      className="mt-1 w-full rounded-xl border border-zinc-200 px-3 py-2 text-sm text-zinc-900"
                    />
                  </div>
                </div>
              </div>
              <div className="mt-5 flex justify-end gap-2">
                <button type="button" onClick={resetReadingGoalForm} className="rounded-xl border border-zinc-200 px-4 py-2 text-sm text-zinc-700">
                  {t("common.cancel")}
                </button>
                <button
                  type="submit"
                  disabled={savingReadingGoal}
                  className="rounded-xl bg-zinc-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
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
