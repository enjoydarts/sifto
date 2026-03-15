"use client";

import { FormEvent, useMemo, useState } from "react";
import { AskResponse, ReadingGoal } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";

interface InsightSaveDialogProps {
  open: boolean;
  loading?: boolean;
  result: AskResponse | null;
  goals: ReadingGoal[];
  onClose: () => void;
  onSave: (input: { title: string; body: string; goal_id?: string | null; tags: string[]; item_ids: string[] }) => Promise<void>;
}

export function InsightSaveDialog({ open, loading = false, result, goals, onClose, onSave }: InsightSaveDialogProps) {
  const { t } = useI18n();
  const [title, setTitle] = useState("");
  const [tagsText, setTagsText] = useState("");
  const [goalId, setGoalId] = useState("");

  const itemIds = useMemo(() => {
    const ids = new Set<string>();
    for (const citation of result?.citations ?? []) ids.add(citation.item_id);
    for (const item of result?.related_items ?? []) ids.add(item.id);
    return Array.from(ids);
  }, [result]);

  if (!open || !result) return null;
  const current = result;

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const nextTitle = title.trim() || current.query;
    const nextBody = current.answer.trim();
    if (!nextTitle || !nextBody) return;
    const tags = tagsText
      .split(",")
      .map((tag) => tag.trim())
      .filter(Boolean);
    await onSave({
      title: nextTitle,
      body: nextBody,
      goal_id: goalId || null,
      tags,
      item_ids: itemIds,
    });
    setTitle("");
    setTagsText("");
    setGoalId("");
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/40 px-4">
      <form onSubmit={handleSubmit} className="w-full max-w-xl rounded-3xl border border-zinc-200 bg-white p-5 shadow-xl">
        <div className="flex items-start justify-between gap-3">
          <div>
            <h2 className="text-lg font-semibold text-zinc-950">{t("ask.insight.title")}</h2>
            <p className="mt-1 text-sm text-zinc-500">{t("ask.insight.subtitle")}</p>
          </div>
          <button type="button" onClick={onClose} className="text-sm text-zinc-500 hover:text-zinc-800">
            {t("common.close")}
          </button>
        </div>
        <div className="mt-4 space-y-4">
          <div>
            <label className="block text-sm font-medium text-zinc-700">{t("ask.insight.fieldTitle")}</label>
            <input
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder={current.query}
              className="mt-1 w-full rounded-xl border border-zinc-200 px-3 py-2 text-sm text-zinc-900"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-zinc-700">{t("ask.insight.fieldGoal")}</label>
            <select
              value={goalId}
              onChange={(e) => setGoalId(e.target.value)}
              className="mt-1 w-full rounded-xl border border-zinc-200 px-3 py-2 text-sm text-zinc-900"
            >
              <option value="">{t("ask.insight.goalNone")}</option>
              {goals.map((goal) => (
                <option key={goal.id} value={goal.id}>
                  {goal.title}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium text-zinc-700">{t("ask.insight.fieldTags")}</label>
            <input
              value={tagsText}
              onChange={(e) => setTagsText(e.target.value)}
              placeholder={t("ask.insight.tagsPlaceholder")}
              className="mt-1 w-full rounded-xl border border-zinc-200 px-3 py-2 text-sm text-zinc-900"
            />
          </div>
          <div className="rounded-2xl bg-zinc-50 p-4 text-sm text-zinc-700">
            <p className="font-medium text-zinc-900">{current.query}</p>
            <p className="mt-2 whitespace-pre-wrap">{current.answer}</p>
          </div>
        </div>
        <div className="mt-5 flex justify-end gap-2">
          <button type="button" onClick={onClose} className="rounded-xl border border-zinc-200 px-4 py-2 text-sm text-zinc-700">
            {t("common.cancel")}
          </button>
          <button
            type="submit"
            disabled={loading}
            className="rounded-xl bg-zinc-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
          >
            {loading ? t("common.saving") : t("ask.insight.save")}
          </button>
        </div>
      </form>
    </div>
  );
}
