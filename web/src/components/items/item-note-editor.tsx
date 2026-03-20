"use client";

import { FormEvent, useState } from "react";
import { ItemNote } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";

type ItemNoteEditorProps = {
  note: ItemNote | null;
  onSave: (content: string) => Promise<void>;
  disabled?: boolean;
};

export function ItemNoteEditor({ note, onSave, disabled = false }: ItemNoteEditorProps) {
  const { t } = useI18n();
  const [value, setValue] = useState(note?.content ?? "");
  const [saving, setSaving] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    try {
      await onSave(value);
    } finally {
      setSaving(false);
    }
  }

  return (
    <form onSubmit={submit} className="flex h-full min-h-0 flex-col rounded-xl border border-zinc-200 bg-zinc-50 p-4">
      <div className="flex items-center justify-between gap-3">
        <h4 className="text-sm font-semibold text-zinc-900">{t("itemNote.title")}</h4>
        <button
          type="submit"
          disabled={saving || disabled}
          className="rounded-lg bg-zinc-900 px-3 py-2 text-xs font-medium text-white disabled:opacity-60"
        >
          {saving ? t("common.saving") : t("itemNote.save")}
        </button>
      </div>
      <textarea
        value={value}
        onChange={(e) => setValue(e.target.value)}
        rows={4}
        disabled={disabled}
        className="mt-3 min-h-[176px] w-full flex-1 rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900"
        placeholder={t("itemNote.placeholder")}
      />
    </form>
  );
}
