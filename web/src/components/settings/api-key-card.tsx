"use client";

import { FormEventHandler } from "react";
import { LucideIcon } from "lucide-react";

type ApiKeyCardLabels = {
  configured: string;
  notSet: string;
  newApiKey: string;
  region: string;
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
  secondaryValue,
  onSecondaryChange,
  secondaryLabel,
  secondaryPlaceholder,
  secondaryStatusText,
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
  secondaryValue?: string;
  onSecondaryChange?: (value: string) => void;
  secondaryLabel?: string;
  secondaryPlaceholder?: string;
  secondaryStatusText?: string | null;
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
    <form onSubmit={onSubmit} className="surface-editorial rounded-[var(--radius-panel)] p-5">
      <div className="mb-4">
        <h2 className="inline-flex items-center gap-2 text-base font-semibold text-[var(--color-editorial-ink)]">
          <Icon className="size-4 text-[var(--color-editorial-ink-faint)]" aria-hidden="true" />
          {title}
        </h2>
        <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{description}</p>
      </div>

      <div className="rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink-soft)]">
        {configured ? (
          <>
            {labels.configured}{" "}
            <span className="font-mono text-xs text-[var(--color-editorial-ink-faint)]">••••{last4 ?? "****"}</span>
            {secondaryStatusText ? (
              <>
                {" "}
                <span className="text-xs text-[var(--color-editorial-ink-faint)]">{labels.region}: {secondaryStatusText}</span>
              </>
            ) : null}
          </>
        ) : (
          <span className="text-[var(--color-editorial-ink-faint)]">{labels.notSet}</span>
        )}
      </div>

      <label className="mt-4 block text-sm font-medium text-[var(--color-editorial-ink)]">{labels.newApiKey}</label>
      <input
        type="password"
        autoComplete="off"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="mt-1 w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)] outline-none ring-0 placeholder:text-[var(--color-editorial-ink-faint)]"
      />

      {secondaryLabel && onSecondaryChange ? (
        <>
          <label className="mt-4 block text-sm font-medium text-[var(--color-editorial-ink)]">{secondaryLabel}</label>
          <input
            type="text"
            autoComplete="off"
            value={secondaryValue ?? ""}
            onChange={(e) => onSecondaryChange(e.target.value)}
            placeholder={secondaryPlaceholder}
            className="mt-1 w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)] outline-none ring-0 placeholder:text-[var(--color-editorial-ink-faint)]"
          />
        </>
      ) : null}

      <div className="mt-4 flex flex-wrap gap-2">
        <button
          type="submit"
          disabled={saving}
          className="rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:opacity-60"
        >
          {saving ? labels.saving : labels.saveOrUpdate}
        </button>
        <button
          type="button"
          disabled={deleting || !configured}
          onClick={onDelete}
          className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] disabled:opacity-50"
        >
          {deleting ? labels.deleting : labels.deleteKey}
        </button>
      </div>
    </form>
  );
}
