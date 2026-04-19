"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Check, ChevronDown, Search, X } from "lucide-react";

type ModelOption = {
  value: string;
  label: string;
  selectedLabel?: string;
  note?: string;
  provider?: string;
  disabled?: boolean;
  badge?: string;
};

export default function ModelSelect({
  label,
  value,
  onChange,
  options,
  labels,
  showMeta = true,
  hideLabel = false,
  variant = "dropdown",
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  options: ModelOption[];
  labels: {
    defaultOption: string;
    searchPlaceholder: string;
    noResults: string;
    providerAll: string;
    modalChoose: string;
    close: string;
    confirmTitle: string;
    confirmYes: string;
    confirmNo: string;
    confirmSuffix: string;
    providerColumn: string;
    modelColumn: string;
    pricingColumn: string;
  };
  showMeta?: boolean;
  hideLabel?: boolean;
  variant?: "dropdown" | "modal";
}) {
  const rootRef = useRef<HTMLDivElement | null>(null);
  const searchInputRef = useRef<HTMLInputElement | null>(null);
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [providerFilter, setProviderFilter] = useState("");
  const [pendingValue, setPendingValue] = useState<string | null>(null);

  const selected = useMemo(
    () => options.find((option) => option.value === value) ?? null,
    [options, value]
  );

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return options.filter((option) => {
      if (providerFilter && (option.provider?.trim() || "Other") !== providerFilter) {
        return false;
      }
      if (!q) return true;
      const haystack = [option.label, option.note ?? "", option.provider ?? ""].join(" ").toLowerCase();
      return haystack.includes(q);
    });
  }, [options, providerFilter, query]);

  const grouped = useMemo(() => {
    const groups = new Map<string, ModelOption[]>();
    for (const option of filtered) {
      const provider = option.provider?.trim() || "Other";
      const current = groups.get(provider) ?? [];
      current.push(option);
      groups.set(provider, current);
    }
    return Array.from(groups.entries());
  }, [filtered]);

  const providerOptions = useMemo(() => {
    return Array.from(new Set(options.map((option) => option.provider?.trim() || "Other"))).sort((a, b) => a.localeCompare(b));
  }, [options]);

  const pendingOption = useMemo(
    () => (pendingValue === null ? null : options.find((option) => option.value === pendingValue) ?? null),
    [options, pendingValue],
  );

  const closeSelect = useCallback(() => {
    setQuery("");
    setProviderFilter("");
    setPendingValue(null);
    setOpen(false);
  }, []);

  const toggleSelect = useCallback(() => {
    if (open) {
      closeSelect();
      return;
    }
    setOpen(true);
  }, [closeSelect, open]);

  useEffect(() => {
    if (!open) return;
    searchInputRef.current?.focus();
  }, [open]);

  useEffect(() => {
    function handlePointerDown(event: MouseEvent) {
      if (!rootRef.current?.contains(event.target as Node)) {
        closeSelect();
      }
    }
    if (!open) return;
    document.addEventListener("mousedown", handlePointerDown);
    return () => document.removeEventListener("mousedown", handlePointerDown);
  }, [closeSelect, open]);

  const selectAndClose = (nextValue: string) => {
    onChange(nextValue);
    setPendingValue(null);
    closeSelect();
  };

  if (variant === "modal") {
    return (
      <div ref={rootRef} className="relative min-w-0">
        {!hideLabel ? <label className="block text-sm font-medium text-zinc-700">{label}</label> : null}
        <button
          type="button"
          onClick={toggleSelect}
          className={`${hideLabel ? "" : "mt-1 "}flex w-full items-start justify-between gap-3 rounded-lg border border-zinc-300 bg-white px-3 py-2 text-left text-sm text-zinc-900 shadow-sm hover:border-zinc-400`}
        >
          <span className="min-w-0">
            <span className={`block truncate font-medium ${selected ? "text-zinc-900" : "text-zinc-500"}`}>
              {selected?.selectedLabel ?? selected?.label ?? labels.defaultOption}
            </span>
            {showMeta ? (
              <span className="mt-0.5 block truncate text-xs text-zinc-500">
                {selected?.note ?? ""}
              </span>
            ) : null}
          </span>
          <ChevronDown className="mt-0.5 size-4 shrink-0 text-zinc-400" />
        </button>

        {open ? (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6" onClick={closeSelect}>
            <div
              className="flex max-h-[90vh] w-full max-w-4xl flex-col overflow-hidden rounded-2xl border border-zinc-200 bg-white shadow-2xl"
              onClick={(event) => event.stopPropagation()}
            >
              <div className="flex items-start justify-between gap-4 border-b border-zinc-200 px-5 py-4">
                <div>
                  <h2 className="text-base font-semibold text-zinc-900">{label}</h2>
                  <p className="mt-1 text-sm text-zinc-500">{labels.modalChoose}</p>
                </div>
                <button
                  type="button"
                  onClick={closeSelect}
                  className="inline-flex size-9 items-center justify-center rounded-lg border border-zinc-300 bg-white text-zinc-700 hover:border-zinc-400 hover:text-zinc-900"
                  aria-label={labels.close}
                >
                  <X className="size-4" aria-hidden="true" />
                </button>
              </div>
              <div className="border-b border-zinc-100 p-4">
                <div className="grid gap-3 md:grid-cols-[220px_minmax(0,1fr)]">
                  <select
                    value={providerFilter}
                    onChange={(e) => setProviderFilter(e.target.value)}
                    className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 shadow-sm outline-none"
                  >
                    <option value="">{labels.providerAll}</option>
                    {providerOptions.map((provider) => (
                      <option key={provider} value={provider}>
                        {provider}
                      </option>
                    ))}
                  </select>
                  <div className="flex items-center gap-2 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2">
                    <Search className="size-4 text-zinc-400" />
                    <input
                      ref={searchInputRef}
                      value={query}
                      onChange={(e) => setQuery(e.target.value)}
                      placeholder={labels.searchPlaceholder}
                      className="w-full bg-transparent text-sm text-zinc-900 outline-none placeholder:text-zinc-400"
                    />
                  </div>
                </div>
              </div>
              <div className="overflow-auto p-3">
                <div className="overflow-hidden rounded-xl border border-zinc-200">
                  <div className="overflow-x-auto">
                    <table className="min-w-[920px] divide-y divide-zinc-200 text-sm">
                      <thead className="bg-zinc-50">
                        <tr className="text-left text-xs font-medium uppercase tracking-[0.08em] text-zinc-500">
                          <th className="whitespace-nowrap px-4 py-3">{labels.providerColumn}</th>
                          <th className="whitespace-nowrap px-4 py-3">{labels.modelColumn}</th>
                          <th className="whitespace-nowrap px-4 py-3">{labels.pricingColumn}</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-zinc-200 bg-white">
                        <tr
                          className={`cursor-pointer transition hover:bg-zinc-50 ${value === "" ? "bg-zinc-50" : ""}`}
                          onClick={() => setPendingValue("")}
                        >
                          <td className="whitespace-nowrap px-4 py-3 text-zinc-500">—</td>
                          <td className="whitespace-nowrap px-4 py-3 font-medium text-zinc-900">{labels.defaultOption}</td>
                          <td className="whitespace-nowrap px-4 py-3 text-zinc-500">—</td>
                        </tr>
                        {filtered.length === 0 ? (
                          <tr>
                            <td colSpan={3} className="px-4 py-6 text-sm text-zinc-500">
                              {labels.noResults}
                            </td>
                          </tr>
                        ) : (
                          filtered.map((opt) => (
                            <tr
                              key={opt.value}
                              className={`${opt.disabled ? "cursor-not-allowed opacity-60" : "cursor-pointer hover:bg-zinc-50"} transition ${value === opt.value ? "bg-zinc-50" : ""}`}
                              onClick={() => {
                                if (opt.disabled) return;
                                setPendingValue(opt.value);
                              }}
                            >
                              <td className="whitespace-nowrap px-4 py-3 text-zinc-700">{opt.provider ?? "Other"}</td>
                              <td className="px-4 py-3">
                                <div className="flex items-center gap-2 whitespace-nowrap">
                                  <span className="font-medium text-zinc-900">{opt.label}</span>
                                  {opt.badge ? (
                                    <span className="rounded-full border border-zinc-300 bg-zinc-100 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.08em] text-zinc-600">
                                      {opt.badge}
                                    </span>
                                  ) : null}
                                </div>
                              </td>
                              <td className="whitespace-nowrap px-4 py-3 text-zinc-600">{opt.note ?? "—"}</td>
                            </tr>
                          ))
                        )}
                      </tbody>
                    </table>
                  </div>
                </div>
              </div>

              {pendingValue !== null ? (
                <div className="border-t border-zinc-200 bg-zinc-50 px-5 py-4">
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <div className="text-sm font-semibold text-zinc-900">{labels.confirmTitle}</div>
                      <p className="mt-1 text-sm text-zinc-600">
                        {(pendingOption?.label ?? labels.defaultOption) + labels.confirmSuffix}
                      </p>
                    </div>
                    <div className="flex gap-2">
                      <button
                        type="button"
                        onClick={() => setPendingValue(null)}
                        className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-medium text-zinc-700 hover:border-zinc-400 hover:text-zinc-900"
                      >
                        {labels.confirmNo}
                      </button>
                      <button
                        type="button"
                        onClick={() => selectAndClose(pendingValue)}
                        className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white"
                      >
                        {labels.confirmYes}
                      </button>
                    </div>
                  </div>
                </div>
              ) : null}
            </div>
          </div>
        ) : null}
      </div>
    );
  }

  return (
    <div ref={rootRef} className="relative min-w-0">
      {!hideLabel ? <label className="block text-sm font-medium text-zinc-700">{label}</label> : null}
      <button
        type="button"
        onClick={toggleSelect}
        className={`${hideLabel ? "" : "mt-1 "}flex w-full items-start justify-between gap-3 rounded-lg border border-zinc-300 bg-white px-3 py-2 text-left text-sm text-zinc-900 shadow-sm hover:border-zinc-400`}
      >
        <span className="min-w-0">
          <span className={`block truncate font-medium ${selected ? "text-zinc-900" : "text-zinc-500"}`}>
            {selected?.selectedLabel ?? selected?.label ?? labels.defaultOption}
          </span>
          {showMeta ? (
            <span className="mt-0.5 block truncate text-xs text-zinc-500">
              {selected?.note ?? ""}
            </span>
          ) : null}
        </span>
        <ChevronDown className={`mt-0.5 size-4 shrink-0 text-zinc-400 transition-transform ${open ? "rotate-180" : ""}`} />
      </button>
      {open && (
        <div className="absolute left-0 right-0 z-30 mt-2 w-full max-w-full overflow-hidden rounded-xl border border-zinc-200 bg-white shadow-2xl">
          <div className="border-b border-zinc-100 p-3">
            <div className="flex items-center gap-2 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2">
              <Search className="size-4 text-zinc-400" />
              <input
                ref={searchInputRef}
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={labels.searchPlaceholder}
                className="w-full bg-transparent text-sm text-zinc-900 outline-none placeholder:text-zinc-400"
              />
            </div>
          </div>
          <div className="max-h-80 overflow-y-auto p-2">
            <button
              type="button"
              onClick={() => {
                onChange("");
                closeSelect();
              }}
              className={`flex w-full items-start justify-between rounded-lg px-3 py-2 text-left hover:bg-zinc-50 ${
                value === "" ? "bg-zinc-100" : ""
              }`}
            >
              <span className="min-w-0">
                <span className="block font-medium text-zinc-900">{labels.defaultOption}</span>
              </span>
              {value === "" && <Check className="mt-0.5 size-4 shrink-0 text-zinc-700" />}
            </button>
            {grouped.length === 0 ? (
              <div className="px-3 py-6 text-sm text-zinc-500">{labels.noResults}</div>
            ) : (
              grouped.map(([provider, providerOptions]) => (
                <div key={provider} className="mt-2 first:mt-0">
                  <div className="px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.12em] text-zinc-400">
                    {provider}
                  </div>
                  <div className="space-y-1">
                    {providerOptions.map((opt) => (
                      <button
                        key={opt.value}
                        type="button"
                        onClick={() => {
                          if (opt.disabled) return;
                          onChange(opt.value);
                          closeSelect();
                        }}
                        disabled={opt.disabled}
                        className={`flex w-full items-start justify-between gap-3 rounded-lg px-3 py-2 text-left ${
                          opt.disabled ? "cursor-not-allowed opacity-60" : "hover:bg-zinc-50"
                        } ${
                          value === opt.value ? "bg-zinc-100" : ""
                        }`}
                      >
                        <span className="min-w-0">
                          <span className="flex flex-wrap items-center gap-2">
                            <span className="block break-all font-medium text-zinc-900">{opt.label}</span>
                            {opt.badge ? (
                              <span className="rounded-full border border-zinc-300 bg-zinc-100 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.08em] text-zinc-600">
                                {opt.badge}
                              </span>
                            ) : null}
                          </span>
                          {opt.note && (
                            <span className="mt-0.5 block whitespace-normal break-words text-xs text-zinc-500">
                              {opt.note}
                            </span>
                          )}
                        </span>
                        {value === opt.value && <Check className="mt-0.5 size-4 shrink-0 text-zinc-700" />}
                      </button>
                    ))}
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  );
}

export type { ModelOption };
