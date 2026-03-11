"use client";

import { FormEventHandler } from "react";
import { LucideIcon } from "lucide-react";

type ApiKeyCardLabels = {
  configured: string;
  notSet: string;
  newApiKey: string;
  saveOrUpdate: string;
  saving: string;
  deleteKey: string;
  deleting: string;
};

export default function ApiKeyCard({
  icon: Icon,
  title,
  description,
  configured,
  last4,
  value,
  onChange,
  onSubmit,
  onDelete,
  placeholder,
  saving,
  deleting,
  labels,
}: {
  icon: LucideIcon;
  title: string;
  description: string;
  configured: boolean;
  last4?: string | null;
  value: string;
  onChange: (value: string) => void;
  onSubmit: FormEventHandler<HTMLFormElement>;
  onDelete: () => void;
  placeholder: string;
  saving: boolean;
  deleting: boolean;
  labels: ApiKeyCardLabels;
}) {
  return (
    <form onSubmit={onSubmit} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
      <div className="mb-4">
        <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
          <Icon className="size-4 text-zinc-500" aria-hidden="true" />
          {title}
        </h2>
        <p className="mt-1 text-sm text-zinc-500">{description}</p>
      </div>

      <div className="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-700">
        {configured ? (
          <>
            {labels.configured}{" "}
            <span className="font-mono text-xs text-zinc-500">••••{last4 ?? "****"}</span>
          </>
        ) : (
          <span className="text-zinc-500">{labels.notSet}</span>
        )}
      </div>

      <label className="mt-4 block text-sm font-medium text-zinc-700">{labels.newApiKey}</label>
      <input
        type="password"
        autoComplete="off"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 outline-none ring-0 placeholder:text-zinc-400 focus:border-zinc-400"
      />

      <div className="mt-4 flex flex-wrap gap-2">
        <button
          type="submit"
          disabled={saving}
          className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
        >
          {saving ? labels.saving : labels.saveOrUpdate}
        </button>
        <button
          type="button"
          disabled={deleting || !configured}
          onClick={onDelete}
          className="rounded-lg border border-zinc-300 bg-white px-4 py-2 text-sm font-medium text-zinc-700 disabled:opacity-50"
        >
          {deleting ? labels.deleting : labels.deleteKey}
        </button>
      </div>
    </form>
  );
}
