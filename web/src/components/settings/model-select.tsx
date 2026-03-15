"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Check, ChevronDown, Search } from "lucide-react";

type ModelOption = {
  value: string;
  label: string;
  note?: string;
  provider?: string;
};

export default function ModelSelect({
  label,
  value,
  onChange,
  options,
  labels,
  showMeta = true,
  hideLabel = false,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  options: ModelOption[];
  labels: {
    defaultOption: string;
    searchPlaceholder: string;
    noResults: string;
  };
  showMeta?: boolean;
  hideLabel?: boolean;
}) {
  const rootRef = useRef<HTMLDivElement | null>(null);
  const searchInputRef = useRef<HTMLInputElement | null>(null);
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");

  const selected = useMemo(
    () => options.find((option) => option.value === value) ?? null,
    [options, value]
  );

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return options;
    return options.filter((option) => {
      const haystack = [option.label, option.note ?? "", option.provider ?? ""].join(" ").toLowerCase();
      return haystack.includes(q);
    });
  }, [options, query]);

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

  useEffect(() => {
    if (!open) {
      setQuery("");
      return;
    }
    searchInputRef.current?.focus();
  }, [open]);

  useEffect(() => {
    function handlePointerDown(event: MouseEvent) {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    }
    if (!open) return;
    document.addEventListener("mousedown", handlePointerDown);
    return () => document.removeEventListener("mousedown", handlePointerDown);
  }, [open]);

  return (
    <div ref={rootRef} className="relative min-w-0">
      {!hideLabel ? <label className="block text-sm font-medium text-zinc-700">{label}</label> : null}
      <button
        type="button"
        onClick={() => setOpen((prev) => !prev)}
        className={`${hideLabel ? "" : "mt-1 "}flex w-full items-start justify-between gap-3 rounded-lg border border-zinc-300 bg-white px-3 py-2 text-left text-sm text-zinc-900 shadow-sm hover:border-zinc-400`}
      >
        <span className="min-w-0">
          <span className={`block truncate font-medium ${selected ? "text-zinc-900" : "text-zinc-500"}`}>
            {selected?.label ?? labels.defaultOption}
          </span>
          {showMeta ? (
            <span className="mt-0.5 block truncate text-xs text-zinc-500">
              {selected?.note ?? (selected?.provider ?? "")}
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
                setOpen(false);
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
                          onChange(opt.value);
                          setOpen(false);
                        }}
                        className={`flex w-full items-start justify-between gap-3 rounded-lg px-3 py-2 text-left hover:bg-zinc-50 ${
                          value === opt.value ? "bg-zinc-100" : ""
                        }`}
                      >
                        <span className="min-w-0">
                          <span className="block break-all font-medium text-zinc-900">{opt.label}</span>
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
