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
    <form
      onSubmit={submit}
      className="flex h-full min-h-0 flex-col rounded-[20px] border border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,rgba(245,240,233,0.78),rgba(255,255,255,0.92))] p-4"
    >
      <div className="flex items-center justify-between gap-3">
        <h4 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("itemNote.title")}</h4>
        <button
          type="submit"
          disabled={saving || disabled}
          className="rounded-full bg-[var(--color-editorial-ink)] px-3 py-2 text-xs font-medium text-[var(--color-editorial-panel-strong)] disabled:opacity-60"
        >
          {saving ? t("common.saving") : t("itemNote.save")}
        </button>
      </div>
      <textarea
        value={value}
        onChange={(e) => setValue(e.target.value)}
        rows={4}
        disabled={disabled}
        className="mt-3 min-h-[176px] w-full flex-1 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2 text-sm text-[var(--color-editorial-ink)]"
        placeholder={t("itemNote.placeholder")}
      />
    </form>
  );
}
