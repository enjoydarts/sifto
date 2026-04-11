"use client";

export default function ProviderVoiceSelectionCard({
  label,
  selectedLabel,
  selectedDetail,
  actionLabel,
  onAction,
  actionDisabled = false,
  helpText,
}: {
  label: string;
  selectedLabel: string;
  selectedDetail: string;
  actionLabel?: string;
  onAction?: () => void;
  actionDisabled?: boolean;
  helpText?: string;
}) {
  return (
    <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-white p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
            {label}
          </div>
          <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">{selectedLabel}</div>
          <div className="mt-1 text-[12px] text-[var(--color-editorial-ink-soft)]">{selectedDetail}</div>
          {helpText ? <div className="mt-2 text-[12px] leading-5 text-[var(--color-editorial-ink-soft)]">{helpText}</div> : null}
        </div>
      </div>

      {actionLabel && onAction ? (
        <button
          type="button"
          onClick={onAction}
          disabled={actionDisabled}
          className="mt-4 inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink)] hover:bg-[var(--color-editorial-panel-strong)] disabled:cursor-not-allowed disabled:opacity-60"
        >
          {actionLabel}
        </button>
      ) : null}
    </div>
  );
}
